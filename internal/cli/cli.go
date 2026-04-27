package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/godofgeeks/docker-distributed-system-emulation/internal/api"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/control"
	"github.com/godofgeeks/docker-distributed-system-emulation/internal/events"
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
	broker := events.NewBroker()
	svc := control.New(root, rt, broker)

	switch args[0] {
	case "up":
		return runUp(svc)
	case "down":
		return runDown(svc)
	case "reset":
		return runReset(svc)
	case "apply-profile":
		return runApplyProfile(svc, root, args[1:])
	case "run-lab":
		return runLab(svc, root, args[1:])
	case "serve":
		return runServe(root, svc, args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runUp(svc *control.Service) error {
	if _, err := svc.Perform("topology.up", nil); err != nil {
		return err
	}
	fmt.Println("Topology is up.")
	return nil
}

func runDown(svc *control.Service) error {
	if _, err := svc.Perform("topology.down", nil); err != nil {
		return err
	}
	fmt.Println("Topology is down.")
	return nil
}

func runReset(svc *control.Service) error {
	if _, err := svc.Perform("topology.reset", nil); err != nil {
		return err
	}
	fmt.Println("Router qdiscs cleared.")
	return nil
}

func runApplyProfile(svc *control.Service, root string, args []string) error {
	fs := flag.NewFlagSet("apply-profile", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: dslab apply-profile <profile>")
	}

	path := project.ResolveRepoPath(root, fs.Arg(0))
	if _, err := svc.Perform("profile.apply", map[string]any{"path": path}); err != nil {
		return err
	}
	fmt.Printf("Applied profile: %s\n", project.RelativeToRoot(root, path))
	return nil
}

func runLab(svc *control.Service, root string, args []string) error {
	fs := flag.NewFlagSet("run-lab", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: dslab run-lab <lab>")
	}

	path := project.ResolveRepoPath(root, fs.Arg(0))
	output, err := svc.Perform("lab.run", map[string]any{"path": path})
	if err != nil {
		return err
	}
	artifact, _ := output["path"].(string)
	fmt.Printf("Wrote lab results: %s\n", project.RelativeToRoot(root, artifact))
	return nil
}

func runServe(root string, svc *control.Service, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", ":8088", "listen address")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}

	server := api.New(root, svc)
	fmt.Printf("Serving UI on %s\n", normalizeAddr(*addr))
	err := server.ListenAndServe(context.Background(), *addr)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func normalizeAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "http://localhost" + addr
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
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
	fmt.Println("  serve          Start the web UI and API server")
}
