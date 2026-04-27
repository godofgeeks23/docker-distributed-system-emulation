package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/netem"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/profile"
)

type Catalog struct {
	Labs       []Lab      `json:"labs"`
	Profiles   []Profile  `json:"profiles"`
	Topologies []Topology `json:"topologies"`
}

type Lab struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Category    string   `json:"category,omitempty" yaml:"category"`
	Type        string   `json:"type" yaml:"type"`
	Summary     string   `json:"summary,omitempty" yaml:"summary"`
	Description string   `json:"description,omitempty" yaml:"description"`
	Tags        []string `json:"tags,omitempty" yaml:"tags"`
	Topology    string   `json:"topology,omitempty" yaml:"topology"`
	Regions     []string `json:"regions,omitempty" yaml:"regions"`
	Count       int      `json:"count,omitempty" yaml:"count"`
	Profiles    LabLinks `json:"profiles,omitempty" yaml:"profiles"`
	UI          LabUI    `json:"ui,omitempty" yaml:"ui"`
	Path        string   `json:"path"`
}

type LabLinks struct {
	Suggested []string `json:"suggested,omitempty" yaml:"suggested"`
}

type LabUI struct {
	Focus       LabFocus        `json:"focus,omitempty" yaml:"focus"`
	Annotations []LabAnnotation `json:"annotations,omitempty" yaml:"annotations"`
}

type LabFocus struct {
	Nodes []string `json:"nodes,omitempty" yaml:"nodes"`
	Edges []string `json:"edges,omitempty" yaml:"edges"`
}

type LabAnnotation struct {
	Title  string `json:"title" yaml:"title"`
	Body   string `json:"body" yaml:"body"`
	Target string `json:"target" yaml:"target"`
}

type Profile struct {
	ID    string                           `json:"id"`
	Name  string                           `json:"name" yaml:"name"`
	Path  string                           `json:"path"`
	Links map[string]map[string]Impairment `json:"links" yaml:"links"`
}

type Impairment struct {
	DelayMS      *float64 `json:"delay_ms,omitempty" yaml:"delay_ms"`
	JitterMS     *float64 `json:"jitter_ms,omitempty" yaml:"jitter_ms"`
	LossPct      *float64 `json:"loss_pct,omitempty" yaml:"loss_pct"`
	RateMbit     *float64 `json:"rate_mbit,omitempty" yaml:"rate_mbit"`
	ReorderPct   *float64 `json:"reorder_pct,omitempty" yaml:"reorder_pct"`
	DuplicatePct *float64 `json:"duplicate_pct,omitempty" yaml:"duplicate_pct"`
	CorruptPct   *float64 `json:"corrupt_pct,omitempty" yaml:"corrupt_pct"`
}

type Topology struct {
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Summary     string         `json:"summary,omitempty" yaml:"summary"`
	Description string         `json:"description,omitempty" yaml:"description"`
	Regions     []Region       `json:"regions" yaml:"regions"`
	Backbone    Backbone       `json:"backbone" yaml:"backbone"`
	Links       []TopologyLink `json:"links" yaml:"links"`
	Path        string         `json:"path"`
}

type Region struct {
	ID     string         `json:"id" yaml:"id"`
	Label  string         `json:"label" yaml:"label"`
	Subnet string         `json:"subnet" yaml:"subnet"`
	Router RegionEndpoint `json:"router" yaml:"router"`
	Nodes  []RegionNode   `json:"nodes" yaml:"nodes"`
}

type RegionEndpoint struct {
	ID      string `json:"id" yaml:"id"`
	Label   string `json:"label" yaml:"label"`
	Service string `json:"service" yaml:"service"`
	IP      string `json:"ip" yaml:"ip"`
}

type RegionNode struct {
	ID      string `json:"id" yaml:"id"`
	Kind    string `json:"kind" yaml:"kind"`
	Label   string `json:"label" yaml:"label"`
	Service string `json:"service" yaml:"service"`
	IP      string `json:"ip" yaml:"ip"`
}

type Backbone struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
}

type TopologyLink struct {
	ID     string `json:"id" yaml:"id"`
	Kind   string `json:"kind" yaml:"kind"`
	Source string `json:"source" yaml:"source"`
	Target string `json:"target" yaml:"target"`
}

func Load(root string) (Catalog, error) {
	labs, err := loadLabs(root)
	if err != nil {
		return Catalog{}, err
	}
	profiles, err := loadProfiles(root)
	if err != nil {
		return Catalog{}, err
	}
	topologies, err := loadTopologies(root)
	if err != nil {
		return Catalog{}, err
	}

	return Catalog{
		Labs:       labs,
		Profiles:   profiles,
		Topologies: topologies,
	}, nil
}

func loadLabs(root string) ([]Lab, error) {
	var labs []Lab
	err := walkYAML(filepath.Join(root, "labs"), func(path string) error {
		var lab Lab
		if err := profile.LoadYAML(path, &lab); err != nil {
			return err
		}
		lab.Path = relativePath(root, path)
		if lab.ID == "" {
			lab.ID = strings.TrimSuffix(lab.Path, "/lab.yml")
		}
		labs = append(labs, lab)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(labs, func(i, j int) bool { return labs[i].ID < labs[j].ID })
	return labs, nil
}

func loadProfiles(root string) ([]Profile, error) {
	var profiles []Profile
	err := walkYAML(filepath.Join(root, "profiles"), func(path string) error {
		var raw netem.LinkProfile
		if err := profile.LoadYAML(path, &raw); err != nil {
			return err
		}
		item := Profile{
			ID:    strings.TrimSuffix(relativePath(root, path), ".yml"),
			Name:  raw.Name,
			Path:  relativePath(root, path),
			Links: map[string]map[string]Impairment{},
		}
		for source, dests := range raw.Links {
			item.Links[source] = map[string]Impairment{}
			for dest, impairments := range dests {
				item.Links[source][dest] = Impairment{
					DelayMS:      impairments.DelayMS,
					JitterMS:     impairments.JitterMS,
					LossPct:      impairments.LossPct,
					RateMbit:     impairments.RateMbit,
					ReorderPct:   impairments.ReorderPct,
					DuplicatePct: impairments.DuplicatePct,
					CorruptPct:   impairments.CorruptPct,
				}
			}
		}
		profiles = append(profiles, item)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func loadTopologies(root string) ([]Topology, error) {
	var topologies []Topology
	err := walkYAML(filepath.Join(root, "topologies"), func(path string) error {
		var topology Topology
		if err := profile.LoadYAML(path, &topology); err != nil {
			return err
		}
		topology.Path = relativePath(root, path)
		if topology.ID == "" {
			return fmt.Errorf("topology missing id: %s", path)
		}
		topologies = append(topologies, topology)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(topologies, func(i, j int) bool { return topologies[i].ID < topologies[j].ID })
	return topologies, nil
}

func walkYAML(root string, fn func(path string) error) error {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		return fn(path)
	})
}

func relativePath(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
