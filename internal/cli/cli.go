package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/labs"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/netem"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/project"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/runtime"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("missing command")
	}

	root, err := project.Root()
	if err != nil {
		return err
	}
	rt := runtime.New(root)

	switch args[0] {
	case "up":
		return runUp(rt)
	case "down":
		return runDown(rt)
	case "reset":
		return runReset(rt)
	case "apply-profile":
		return runApplyProfile(rt, root, args[1:])
	case "run-lab":
		return runLab(rt, root, args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runUp(rt runtime.Runtime) error {
	if err := rt.RunDockerCompose("up", "-d", "--build"); err != nil {
		return err
	}
	fmt.Println("Topology is up.")
	return nil
}

func runDown(rt runtime.Runtime) error {
	if err := rt.RunDockerCompose("down", "--remove-orphans", "-v"); err != nil {
		return err
	}
	fmt.Println("Topology is down.")
	return nil
}

func runReset(rt runtime.Runtime) error {
	if err := netem.ResetAll(rt); err != nil {
		return err
	}
	fmt.Println("Router qdiscs cleared.")
	return nil
}

func runApplyProfile(rt runtime.Runtime, root string, args []string) error {
	fs := flag.NewFlagSet("apply-profile", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: dslab apply-profile <profile>")
	}

	path := project.ResolveRepoPath(root, fs.Arg(0))
	if err := netem.ApplyProfile(rt, path); err != nil {
		return err
	}
	fmt.Printf("Applied profile: %s\n", project.RelativeToRoot(root, path))
	return nil
}

func runLab(rt runtime.Runtime, root string, args []string) error {
	fs := flag.NewFlagSet("run-lab", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: dslab run-lab <lab>")
	}

	path := project.ResolveRepoPath(root, fs.Arg(0))
	artifact, err := labs.Run(rt, root, path)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote lab results: %s\n", project.RelativeToRoot(root, artifact))
	return nil
}

func printUsage() {
	fmt.Println("Usage: dslab <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  up             Build and start the topology")
	fmt.Println("  down           Stop and remove the topology")
	fmt.Println("  reset          Remove all configured qdiscs from the routers")
	fmt.Println("  apply-profile  Apply a latency/fault profile")
	fmt.Println("  run-lab        Run a lab manifest")
}
