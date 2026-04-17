package profile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return err
	}
	if isNilMapping(out) {
		return fmt.Errorf("%s did not contain a mapping", path)
	}
	return nil
}

func isNilMapping(v any) bool {
	switch typed := v.(type) {
	case *map[string]any:
		return *typed == nil
	}
	return false
}
