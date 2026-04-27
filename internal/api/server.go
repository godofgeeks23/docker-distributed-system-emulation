package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/catalog"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/control"
)

type Server struct {
	control *control.Service
	root    string
}

func New(root string, control *control.Service) *Server {
	return &Server{
		control: control,
		root:    root,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/topologies", s.handleTopologies)
	mux.HandleFunc("/api/topologies/", s.handleTopologyByID)
	mux.HandleFunc("/api/labs", s.handleLabs)
	mux.HandleFunc("/api/labs/", s.handleLabByID)
	mux.HandleFunc("/api/profiles", s.handleProfiles)
	mux.HandleFunc("/api/jobs/", s.handleJobByID)
	mux.HandleFunc("/api/topology/up", s.handleAction("topology.up", nil))
	mux.HandleFunc("/api/topology/down", s.handleAction("topology.down", nil))
	mux.HandleFunc("/api/topology/reset", s.handleAction("topology.reset", nil))
	mux.HandleFunc("/api/profiles/apply", s.handleProfileApply)
	mux.HandleFunc("/api/labs/run", s.handleLabRun)
	mux.HandleFunc("/api/events/stream", s.handleEvents)
	mux.Handle("/", s.staticHandler())

	return loggingMiddleware(mux)
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	return srv.ListenAndServe()
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	status, err := s.control.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleTopologies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	cat, err := s.control.Catalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	type item struct {
		Topologies []TopologyView `json:"topologies"`
	}

	topologies := make([]TopologyView, 0, len(cat.Topologies))
	for _, topology := range cat.Topologies {
		topologies = append(topologies, buildTopologyView(topology, nil))
	}

	writeJSON(w, http.StatusOK, item{Topologies: topologies})
}

func (s *Server) handleTopologyByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/topologies/")
	cat, err := s.control.Catalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	for _, topology := range cat.Topologies {
		if topology.ID == id {
			writeJSON(w, http.StatusOK, buildTopologyView(topology, nil))
			return
		}
	}

	writeErrorMessage(w, http.StatusNotFound, "topology not found")
}

func (s *Server) handleLabs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	cat, err := s.control.Catalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	type item struct {
		Labs []catalog.Lab `json:"labs"`
	}

	writeJSON(w, http.StatusOK, item{Labs: cat.Labs})
}

func (s *Server) handleLabByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/labs/")
	cat, err := s.control.Catalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var selected *catalog.Lab
	for i := range cat.Labs {
		if cat.Labs[i].ID == id {
			selected = &cat.Labs[i]
			break
		}
	}
	if selected == nil {
		writeErrorMessage(w, http.StatusNotFound, "lab not found")
		return
	}

	var topology *catalog.Topology
	for i := range cat.Topologies {
		if cat.Topologies[i].ID == selected.Topology {
			topology = &cat.Topologies[i]
			break
		}
	}

	type response struct {
		Lab      catalog.Lab   `json:"lab"`
		Topology *TopologyView `json:"topology,omitempty"`
	}

	var view *TopologyView
	if topology != nil {
		topologyView := buildTopologyView(*topology, selected)
		view = &topologyView
	}

	writeJSON(w, http.StatusOK, response{Lab: *selected, Topology: view})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	cat, err := s.control.Catalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	type item struct {
		Profiles []catalog.Profile `json:"profiles"`
	}

	writeJSON(w, http.StatusOK, item{Profiles: cat.Profiles})
}

func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	job, ok := s.control.Job(id)
	if !ok {
		writeErrorMessage(w, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleAction(actionType string, body func(*http.Request) (map[string]any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}

		var input map[string]any
		var err error
		if body != nil {
			input, err = body(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}

		job := s.control.Enqueue(actionType, input)
		writeJSON(w, http.StatusAccepted, job)
	}
}

func (s *Server) handleProfileApply(w http.ResponseWriter, r *http.Request) {
	s.handleAction("profile.apply", func(r *http.Request) (map[string]any, error) {
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		return map[string]any{"path": body.Path}, nil
	})(w, r)
}

func (s *Server) handleLabRun(w http.ResponseWriter, r *http.Request) {
	s.handleAction("lab.run", func(r *http.Request) (map[string]any, error) {
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		return map[string]any{"path": body.Path}, nil
	})(w, r)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorMessage(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventsCh, unsubscribe := s.control.Subscribe()
	defer unsubscribe()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-eventsCh:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func (s *Server) staticHandler() http.Handler {
	distDir := filepath.Join(s.root, "web", "dist")
	indexPath := filepath.Join(distDir, "index.html")

	if _, err := os.Stat(indexPath); err == nil {
		fs := http.FileServer(http.Dir(distDir))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			target := filepath.Join(distDir, strings.TrimPrefix(filepath.Clean(r.URL.Path), "/"))
			if info, err := os.Stat(target); err == nil && !info.IsDir() {
				fs.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, indexPath)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body style="font-family: sans-serif; padding: 2rem;"><h1>UI build missing</h1><p>Run <code>npm install</code> and <code>npm run build</code> in <code>web/</code>, then start <code>dslab serve</code> again.</p></body></html>`)
	})
}

type TopologyView struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Label       string         `json:"label"`
	ParentID    string         `json:"parent_id,omitempty"`
	Service     string         `json:"service,omitempty"`
	RegionID    string         `json:"region_id,omitempty"`
	Subnet      string         `json:"subnet,omitempty"`
	IP          string         `json:"ip,omitempty"`
	Highlighted bool           `json:"highlighted,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

type GraphEdge struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Source      string         `json:"source"`
	Target      string         `json:"target"`
	Label       string         `json:"label,omitempty"`
	Highlighted bool           `json:"highlighted,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

func buildTopologyView(topology catalog.Topology, lab *catalog.Lab) TopologyView {
	highlightNodes := map[string]bool{}
	highlightEdges := map[string]bool{}
	if lab != nil {
		for _, id := range lab.UI.Focus.Nodes {
			highlightNodes[id] = true
		}
		for _, id := range lab.UI.Focus.Edges {
			highlightEdges[id] = true
		}
	}

	nodes := make([]GraphNode, 0, len(topology.Regions)*3+1)
	nodes = append(nodes, GraphNode{
		ID:    topology.Backbone.ID,
		Kind:  "backbone",
		Label: topology.Backbone.Label,
	})

	for _, region := range topology.Regions {
		regionNodeID := "region:" + region.ID
		nodes = append(nodes, GraphNode{
			ID:          regionNodeID,
			Kind:        "region",
			Label:       region.Label,
			RegionID:    region.ID,
			Subnet:      region.Subnet,
			Highlighted: highlightNodes[regionNodeID],
		})
		nodes = append(nodes, GraphNode{
			ID:          region.Router.ID,
			Kind:        "router",
			Label:       region.Router.Label,
			Service:     region.Router.Service,
			RegionID:    region.ID,
			ParentID:    regionNodeID,
			IP:          region.Router.IP,
			Highlighted: highlightNodes[region.Router.ID],
		})
		for _, node := range region.Nodes {
			nodes = append(nodes, GraphNode{
				ID:          node.ID,
				Kind:        node.Kind,
				Label:       node.Label,
				Service:     node.Service,
				RegionID:    region.ID,
				ParentID:    regionNodeID,
				IP:          node.IP,
				Highlighted: highlightNodes[node.ID],
			})
		}
	}

	edges := make([]GraphEdge, 0, len(topology.Links)+(len(topology.Regions)*2))
	for _, region := range topology.Regions {
		for _, node := range region.Nodes {
			edges = append(edges, GraphEdge{
				ID:     "lan:" + node.ID + ":" + region.Router.ID,
				Kind:   "lan-link",
				Source: node.ID,
				Target: region.Router.ID,
				Label:  "LAN",
			})
		}
		edges = append(edges, GraphEdge{
			ID:     "uplink:" + region.Router.ID + ":" + topology.Backbone.ID,
			Kind:   "uplink",
			Source: region.Router.ID,
			Target: topology.Backbone.ID,
			Label:  "Backbone",
		})
	}
	for _, link := range topology.Links {
		edges = append(edges, GraphEdge{
			ID:          link.ID,
			Kind:        link.Kind,
			Source:      link.Source,
			Target:      link.Target,
			Label:       "WAN",
			Highlighted: highlightEdges[link.ID],
		})
	}

	return TopologyView{
		ID:          topology.ID,
		Name:        topology.Name,
		Summary:     topology.Summary,
		Description: topology.Description,
		Nodes:       nodes,
		Edges:       edges,
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeErrorMessage(w, status, err.Error())
}

func writeErrorMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeErrorMessage(w, http.StatusMethodNotAllowed, "method not allowed")
}
