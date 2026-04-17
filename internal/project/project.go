package project

import (
	"errors"
	"os"
	"path/filepath"
)

var rootMarkers = []string{
	filepath.Join("compose", "base.yml"),
	filepath.Join("compose", "observability.yml"),
}

func Root() (string, error) {
	if fromEnv := os.Getenv("DSLAB_ROOT"); fromEnv != "" {
		return filepath.Abs(fromEnv)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if hasRootMarkers(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("failed to locate repository root; run from the repo or set DSLAB_ROOT")
}

func ResolveRepoPath(root string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func RelativeToRoot(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func hasRootMarkers(root string) bool {
	for _, marker := range rootMarkers {
		if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
			return false
		}
	}
	return true
}
