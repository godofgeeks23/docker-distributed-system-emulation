package labs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/profile"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/runtime"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/topology"
)

var pingRe = regexp.MustCompile(`rtt min/avg/max/(?:mdev|stddev) = ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+) ms`)

type Lab struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Regions []string `yaml:"regions"`
	Count   int      `yaml:"count"`
}

type PingStats struct {
	MinMS    float64 `json:"min_ms"`
	AvgMS    float64 `json:"avg_ms"`
	MaxMS    float64 `json:"max_ms"`
	StdDevMS float64 `json:"stddev_ms"`
	Count    int     `json:"count"`
}

type PingMatrixResults struct {
	Name       string                          `json:"name"`
	Type       string                          `json:"type"`
	CapturedAt string                          `json:"captured_at"`
	Results    map[string]map[string]PingStats `json:"results"`
}

func Run(rt runtime.Runtime, root string, path string) (string, error) {
	var lab Lab
	if err := profile.LoadYAML(path, &lab); err != nil {
		return "", err
	}
	if lab.Type != "ping-matrix" {
		return "", fmt.Errorf("unsupported lab type: %s", lab.Type)
	}
	if lab.Count == 0 {
		lab.Count = 4
	}
	if len(lab.Regions) == 0 {
		lab.Regions = append([]string{}, topology.RegionOrder...)
	}

	results, err := runPingMatrix(rt, lab)
	if err != nil {
		return "", err
	}

	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return "", err
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	outputPath := filepath.Join(artifactsDir, lab.Name+"-"+timestamp+".json")

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return "", err
	}

	return outputPath, nil
}

func runPingMatrix(rt runtime.Runtime, lab Lab) (PingMatrixResults, error) {
	results := make(map[string]map[string]PingStats)

	for _, sourceName := range lab.Regions {
		source, ok := topology.Regions[sourceName]
		if !ok {
			return PingMatrixResults{}, fmt.Errorf("unknown source region in lab: %s", sourceName)
		}
		results[sourceName] = make(map[string]PingStats)

		for _, destName := range lab.Regions {
			if destName == sourceName {
				continue
			}
			dest, ok := topology.Regions[destName]
			if !ok {
				return PingMatrixResults{}, fmt.Errorf("unknown destination region in lab: %s", destName)
			}

			stdout, err := rt.ExecService(source.ProbeService, fmt.Sprintf("ping -q -c %d -W 2 %s", lab.Count, dest.ProbeIP))
			if err != nil {
				return PingMatrixResults{}, err
			}
			stats, err := parsePingOutput(stdout, lab.Count)
			if err != nil {
				return PingMatrixResults{}, fmt.Errorf("failed to parse ping output for %s -> %s: %w", sourceName, destName, err)
			}
			results[sourceName][destName] = stats
		}
	}

	return PingMatrixResults{
		Name:       lab.Name,
		Type:       lab.Type,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Results:    results,
	}, nil
}

func parsePingOutput(stdout string, count int) (PingStats, error) {
	match := pingRe.FindStringSubmatch(stdout)
	if match == nil {
		return PingStats{}, fmt.Errorf("missing summary line in output: %s", strings.TrimSpace(stdout))
	}

	values := make([]float64, 0, 4)
	for _, raw := range match[1:] {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return PingStats{}, err
		}
		values = append(values, value)
	}

	return PingStats{
		MinMS:    values[0],
		AvgMS:    values[1],
		MaxMS:    values[2],
		StdDevMS: values[3],
		Count:    count,
	}, nil
}
