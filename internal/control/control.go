package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/catalog"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/events"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/labs"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/netem"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/project"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/runtime"
)

type Service struct {
	root   string
	rt     runtime.Runtime
	events *events.Broker

	jobMu   sync.Mutex
	jobs    map[string]*Job
	jobSeq  uint64
	jobChan chan *Job
}

type Job struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Status      string         `json:"status"`
	RequestedAt string         `json:"requested_at"`
	StartedAt   string         `json:"started_at,omitempty"`
	FinishedAt  string         `json:"finished_at,omitempty"`
	Summary     string         `json:"summary"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
}

type Status struct {
	Root          string             `json:"root"`
	TopologyUp    bool               `json:"topology_up"`
	ActiveProfile string             `json:"active_profile,omitempty"`
	Services      []ServiceStatus    `json:"services"`
	RecentRuns    []ArtifactMetadata `json:"recent_runs"`
}

type ServiceStatus struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Health string `json:"health"`
}

type ArtifactMetadata struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	CapturedAt string `json:"captured_at"`
}

func New(root string, rt runtime.Runtime, broker *events.Broker) *Service {
	s := &Service{
		root:    root,
		rt:      rt,
		events:  broker,
		jobs:    make(map[string]*Job),
		jobChan: make(chan *Job, 16),
	}

	go s.worker()

	return s
}

func (s *Service) Catalog() (catalog.Catalog, error) {
	return catalog.Load(s.root)
}

func (s *Service) Status() (Status, error) {
	services, err := s.composeServices()
	if err != nil {
		return Status{}, err
	}
	if services == nil {
		services = []ServiceStatus{}
	}
	artifacts := s.listArtifacts()
	if artifacts == nil {
		artifacts = []ArtifactMetadata{}
	}

	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	return Status{
		Root:          s.root,
		TopologyUp:    hasRunningServices(services),
		ActiveProfile: s.readActiveProfile(),
		Services:      services,
		RecentRuns:    artifacts,
	}, nil
}

func (s *Service) Subscribe() (<-chan events.Event, func()) {
	return s.events.Subscribe()
}

func (s *Service) Enqueue(actionType string, input map[string]any) *Job {
	job := &Job{
		ID:          s.nextJobID(),
		Type:        actionType,
		Status:      "queued",
		RequestedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     actionSummary(actionType),
		Input:       input,
	}

	s.jobMu.Lock()
	s.jobs[job.ID] = job
	s.jobMu.Unlock()

	s.events.Publish("job.queued", job.Summary, map[string]any{
		"job_id": job.ID,
		"type":   job.Type,
	})

	s.jobChan <- job

	return cloneJob(job)
}

func (s *Service) Job(id string) (*Job, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	return cloneJob(job), true
}

func (s *Service) Perform(actionType string, input map[string]any) (map[string]any, error) {
	return s.execute(&Job{Type: actionType, Input: input})
}

func (s *Service) worker() {
	for job := range s.jobChan {
		s.runJob(job)
	}
}

func (s *Service) runJob(job *Job) {
	s.updateJob(job.ID, func(j *Job) {
		j.Status = "running"
		j.StartedAt = time.Now().UTC().Format(time.RFC3339)
	})
	s.events.Publish("job.started", job.Summary, map[string]any{
		"job_id": job.ID,
		"type":   job.Type,
	})

	output, err := s.execute(job)
	if err != nil {
		s.updateJob(job.ID, func(j *Job) {
			j.Status = "failed"
			j.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			j.Error = err.Error()
		})
		s.events.Publish(job.Type+".failed", err.Error(), map[string]any{
			"job_id": job.ID,
			"type":   job.Type,
			"error":  err.Error(),
		})
		return
	}

	s.updateJob(job.ID, func(j *Job) {
		j.Status = "succeeded"
		j.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		j.Output = output
	})
	s.events.Publish(job.Type+".completed", job.Summary, map[string]any{
		"job_id": job.ID,
		"type":   job.Type,
		"output": output,
	})
}

func (s *Service) execute(job *Job) (map[string]any, error) {
	switch job.Type {
	case "topology.up":
		if err := s.rt.RunDockerCompose("up", "-d", "--build"); err != nil {
			return nil, err
		}
		return map[string]any{"message": "topology is up"}, nil
	case "topology.down":
		if err := s.rt.RunDockerCompose("down", "--remove-orphans", "-v"); err != nil {
			return nil, err
		}
		s.writeActiveProfile("")
		return map[string]any{"message": "topology is down"}, nil
	case "topology.reset":
		if err := netem.ResetAll(s.rt); err != nil {
			return nil, err
		}
		s.writeActiveProfile("")
		return map[string]any{"message": "router qdiscs cleared"}, nil
	case "profile.apply":
		path, ok := stringInput(job.Input, "path")
		if !ok || path == "" {
			return nil, errors.New("missing profile path")
		}
		resolved := project.ResolveRepoPath(s.root, path)
		if err := netem.ApplyProfile(s.rt, resolved); err != nil {
			return nil, err
		}
		s.writeActiveProfile(project.RelativeToRoot(s.root, resolved))
		return map[string]any{"path": project.RelativeToRoot(s.root, resolved)}, nil
	case "lab.run":
		path, ok := stringInput(job.Input, "path")
		if !ok || path == "" {
			return nil, errors.New("missing lab path")
		}
		resolved := project.ResolveRepoPath(s.root, path)
		artifact, err := labs.Run(s.rt, s.root, resolved)
		if err != nil {
			return nil, err
		}
		parsed := s.readArtifact(project.RelativeToRoot(s.root, artifact))
		output := map[string]any{
			"path": project.RelativeToRoot(s.root, artifact),
		}
		if parsed != nil {
			output["artifact"] = parsed
		}
		return output, nil
	default:
		return nil, fmt.Errorf("unsupported job type: %s", job.Type)
	}
}

func (s *Service) composeServices() ([]ServiceStatus, error) {
	output, err := s.rt.DockerComposeOutput("ps", "--format", "json", "--all")
	if err != nil {
		var commandErr *runtime.CommandError
		if errors.As(err, &commandErr) && strings.Contains(commandErr.Stderr, "no containers") {
			return nil, nil
		}
		return nil, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	statuses := make([]ServiceStatus, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row struct {
			Service string `json:"Service"`
			State   string `json:"State"`
			Health  string `json:"Health"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		statuses = append(statuses, ServiceStatus{
			Name:   row.Service,
			State:  strings.ToLower(row.State),
			Health: strings.ToLower(row.Health),
		})
	}

	return statuses, nil
}

func hasRunningServices(services []ServiceStatus) bool {
	for _, service := range services {
		if service.State == "running" {
			return true
		}
	}
	return false
}

func (s *Service) listArtifacts() []ArtifactMetadata {
	pattern := filepath.Join(s.root, "artifacts", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	sort.Sort(sort.Reverse(sort.StringSlice(paths)))

	items := make([]ArtifactMetadata, 0, len(paths))
	for _, path := range paths {
		rel := project.RelativeToRoot(s.root, path)
		item := ArtifactMetadata{
			Path: rel,
			Name: filepath.Base(path),
			Type: "artifact",
		}

		if parsed := s.readArtifact(rel); parsed != nil {
			item.Name = valueString(parsed, "name", item.Name)
			item.Type = valueString(parsed, "type", item.Type)
			item.CapturedAt = valueString(parsed, "captured_at", "")
		}

		items = append(items, item)
		if len(items) >= 10 {
			break
		}
	}

	return items
}

func (s *Service) readArtifact(rel string) map[string]any {
	data, err := os.ReadFile(filepath.Join(s.root, rel))
	if err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload
}

func (s *Service) stateDir() string {
	return filepath.Join(s.root, ".dslab")
}

func (s *Service) activeProfilePath() string {
	return filepath.Join(s.stateDir(), "active-profile")
}

func (s *Service) readActiveProfile() string {
	data, err := os.ReadFile(s.activeProfilePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *Service) writeActiveProfile(value string) {
	if err := os.MkdirAll(s.stateDir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(s.activeProfilePath(), []byte(value+"\n"), 0o644)
}

func (s *Service) nextJobID() string {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	s.jobSeq++
	return fmt.Sprintf("job-%06d", s.jobSeq)
}

func (s *Service) updateJob(id string, fn func(job *Job)) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if job, ok := s.jobs[id]; ok {
		fn(job)
	}
}

func cloneJob(job *Job) *Job {
	if job == nil {
		return nil
	}
	copy := *job
	return &copy
}

func stringInput(input map[string]any, key string) (string, bool) {
	if input == nil {
		return "", false
	}
	raw, ok := input[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	return value, ok
}

func valueString(payload map[string]any, key string, fallback string) string {
	value, ok := payload[key]
	if !ok {
		return fallback
	}
	typed, ok := value.(string)
	if !ok {
		return fallback
	}
	return typed
}

func actionSummary(actionType string) string {
	switch actionType {
	case "topology.up":
		return "Starting topology"
	case "topology.down":
		return "Stopping topology"
	case "topology.reset":
		return "Resetting active impairments"
	case "profile.apply":
		return "Applying profile"
	case "lab.run":
		return "Running lab"
	default:
		return actionType
	}
}
