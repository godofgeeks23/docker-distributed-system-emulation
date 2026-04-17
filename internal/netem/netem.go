package netem

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/profile"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/runtime"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/topology"
)

type Impairments struct {
	DelayMS      *float64 `yaml:"delay_ms"`
	JitterMS     *float64 `yaml:"jitter_ms"`
	LossPct      *float64 `yaml:"loss_pct"`
	RateMbit     *float64 `yaml:"rate_mbit"`
	ReorderPct   *float64 `yaml:"reorder_pct"`
	DuplicatePct *float64 `yaml:"duplicate_pct"`
	CorruptPct   *float64 `yaml:"corrupt_pct"`
}

type LinkProfile struct {
	Name  string                            `yaml:"name"`
	Links map[string]map[string]Impairments `yaml:"links"`
}

func ResetAll(rt runtime.Runtime) error {
	for _, regionName := range topology.RegionOrder {
		region := topology.Regions[regionName]
		iface, err := backboneIface(rt, region.RouterService)
		if err != nil {
			return err
		}
		if _, err := rt.ExecService(region.RouterService, fmt.Sprintf("tc qdisc del dev %s root >/dev/null 2>&1 || true", iface)); err != nil {
			return err
		}
	}
	return nil
}

func ApplyProfile(rt runtime.Runtime, path string) error {
	var cfg LinkProfile
	if err := profile.LoadYAML(path, &cfg); err != nil {
		return err
	}
	if cfg.Links == nil {
		cfg.Links = map[string]map[string]Impairments{}
	}

	for _, sourceName := range topology.RegionOrder {
		source := topology.Regions[sourceName]
		iface, err := backboneIface(rt, source.RouterService)
		if err != nil {
			return err
		}

		remoteLinks := cfg.Links[sourceName]
		bands := len(remoteLinks) + 1
		if bands < 2 {
			bands = 2
		}

		commands := []string{
			fmt.Sprintf("tc qdisc del dev %s root >/dev/null 2>&1 || true", iface),
			fmt.Sprintf("tc qdisc replace dev %s root handle 1: prio bands %d", iface, bands),
		}

		destinations := sortedLinkDestinations(remoteLinks)
		classIndex := 1
		for _, destName := range destinations {
			impairments := remoteLinks[destName]
			dest, ok := topology.Regions[destName]
			if !ok {
				return fmt.Errorf("unknown destination region in profile: %s", destName)
			}
			netemArgs := buildNetemArgs(impairments)
			if netemArgs == "" {
				continue
			}

			flowID := fmt.Sprintf("1:%d", classIndex)
			handle := fmt.Sprintf("%d:", classIndex+10)
			commands = append(commands,
				fmt.Sprintf("tc qdisc replace dev %s parent %s handle %s netem %s", iface, flowID, handle, netemArgs),
				fmt.Sprintf(
					"tc filter replace dev %s protocol ip parent 1:0 prio %d u32 match ip dst %s flowid %s",
					iface,
					classIndex,
					dest.Subnet,
					flowID,
				),
			)
			classIndex++
		}

		if _, err := rt.ExecService(source.RouterService, strings.Join(commands, " && ")); err != nil {
			return err
		}
	}

	return nil
}

func backboneIface(rt runtime.Runtime, service string) (string, error) {
	stdout, err := rt.ExecService(service, "cat /run/dslab/backbone_iface")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func sortedLinkDestinations(remoteLinks map[string]Impairments) []string {
	if len(remoteLinks) == 0 {
		return nil
	}
	keys := make([]string, 0, len(remoteLinks))
	for key := range remoteLinks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func buildNetemArgs(impairments Impairments) string {
	var parts []string

	if impairments.DelayMS != nil {
		if impairments.JitterMS != nil {
			parts = append(parts, "delay "+formatFloat(*impairments.DelayMS)+"ms "+formatFloat(*impairments.JitterMS)+"ms distribution normal")
		} else {
			parts = append(parts, "delay "+formatFloat(*impairments.DelayMS)+"ms")
		}
	}
	if impairments.LossPct != nil {
		parts = append(parts, "loss "+formatFloat(*impairments.LossPct)+"%")
	}
	if impairments.ReorderPct != nil {
		parts = append(parts, "reorder "+formatFloat(*impairments.ReorderPct)+"%")
	}
	if impairments.DuplicatePct != nil {
		parts = append(parts, "duplicate "+formatFloat(*impairments.DuplicatePct)+"%")
	}
	if impairments.CorruptPct != nil {
		parts = append(parts, "corrupt "+formatFloat(*impairments.CorruptPct)+"%")
	}
	if impairments.RateMbit != nil {
		parts = append(parts, "rate "+formatFloat(*impairments.RateMbit)+"mbit")
	}

	return strings.Join(parts, " ")
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
