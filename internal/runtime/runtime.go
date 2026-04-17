package runtime

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

type Runtime struct {
	Root string
}

func New(root string) Runtime {
	return Runtime{Root: root}
}

func (r Runtime) ComposeArgs(args ...string) []string {
	base := []string{
		"compose",
		"-f", filepath.Join(r.Root, "compose", "base.yml"),
		"-f", filepath.Join(r.Root, "compose", "observability.yml"),
	}
	return append(base, args...)
}

func (r Runtime) RunDockerCompose(args ...string) error {
	cmd := exec.Command("docker", r.ComposeArgs(args...)...)
	cmd.Dir = r.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r Runtime) ExecService(service string, shellCommand string) (string, error) {
	args := append(r.ComposeArgs("exec", "-T", service, "sh", "-lc", shellCommand))
	cmd := exec.Command("docker", args...)
	cmd.Dir = r.Root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", &CommandError{Err: err, Stderr: stderr.String()}
		}
		return "", err
	}

	return stdout.String(), nil
}

type CommandError struct {
	Err    error
	Stderr string
}

func (e *CommandError) Error() string {
	return e.Err.Error() + ": " + e.Stderr
}
