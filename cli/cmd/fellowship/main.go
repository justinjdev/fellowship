package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/install"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "hook":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship hook <name>")
			os.Exit(1)
		}
		os.Exit(runHook(os.Args[2]))
	case "gate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship gate <status|approve|reject>")
			os.Exit(1)
		}
		os.Exit(runGate(os.Args[2:]))
	case "install":
		os.Exit(runInstall())
	case "uninstall":
		os.Exit(runUninstall())
	case "init":
		os.Exit(runInit())
	case "version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: fellowship <command>

Hook commands (called by Claude Code hooks, read stdin):
  hook gate-guard        Block tools when gate pending or early-phase file writes
  hook gate-submit       Detect [GATE] messages, check prereqs, advance state
  hook gate-prereq       Track lembas skill invocation
  hook metadata-track    Track phase metadata updates
  hook completion-guard  Block task completion unless phase is Complete

Agent/lead commands:
  gate status            Show current phase, prereqs, pending state
  gate approve           Approve a pending gate (advances to next phase)
  gate reject            Reject a pending gate (clears pending, keeps phase)

Setup commands:
  install                Merge gate hooks into .claude/settings.json
  uninstall              Remove gate hooks from .claude/settings.json
  init                   Create tmp/quest-state.json with defaults

Other:
  version                Print version`)
}

func runHook(name string) int {
	cwd, _ := os.Getwd()
	statePath, err := state.FindStateFile(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 2
	}
	if statePath == "" {
		return 0
	}

	s, err := state.Load(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 2
	}

	input, err := hooks.ParseInput(os.Stdin)
	if err != nil {
		switch name {
		case "gate-guard":
			input = &hooks.HookInput{}
		default:
			fmt.Fprintln(os.Stderr, "fellowship: malformed hook input — blocking for safety")
			return 2
		}
	}

	var result hooks.HookResult
	stateChanged := false

	switch name {
	case "gate-guard":
		result = hooks.GateGuard(s, input)
	case "gate-submit":
		sr := hooks.GateSubmit(s, input)
		result = hooks.HookResult{Block: sr.Block, Message: sr.Message}
		stateChanged = sr.StateChanged
	case "gate-prereq":
		stateChanged = hooks.GatePrereq(s, input)
	case "metadata-track":
		stateChanged = hooks.MetadataTrack(s, input)
	case "completion-guard":
		result = hooks.CompletionGuard(s, input)
	default:
		fmt.Fprintf(os.Stderr, "fellowship: unknown hook %q\n", name)
		return 2
	}

	if stateChanged {
		if err := state.Save(statePath, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: failed to save state: %v\n", err)
			return 2
		}
	}

	if result.Block {
		fmt.Fprintln(os.Stderr, result.Message)
		return 2
	}
	return 0
}

func runGate(args []string) int {
	cwd, _ := os.Getwd()
	statePath, err := state.FindStateFile(cwd)
	if err != nil || statePath == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest state file found")
		return 1
	}
	s, err := state.Load(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	switch args[0] {
	case "status":
		fmt.Printf("Phase:    %s\n", s.Phase)
		fmt.Printf("Pending:  %v\n", s.GatePending)
		fmt.Printf("Lembas:   %v\n", s.LembasCompleted)
		fmt.Printf("Metadata: %v\n", s.MetadataUpdated)
		if s.GateID != nil {
			fmt.Printf("Gate ID:  %s\n", *s.GateID)
		}
		return 0

	case "approve":
		if !s.GatePending {
			fmt.Fprintln(os.Stderr, "No gate pending")
			return 1
		}
		nextPhase, err := state.NextPhase(s.Phase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		s.GatePending = false
		s.Phase = nextPhase
		s.GateID = nil
		s.LembasCompleted = false
		s.MetadataUpdated = false
		if err := state.Save(statePath, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("Gate approved. Phase advanced to %s.\n", nextPhase)
		return 0

	case "reject":
		if !s.GatePending {
			fmt.Fprintln(os.Stderr, "No gate pending")
			return 1
		}
		s.GatePending = false
		s.GateID = nil
		if err := state.Save(statePath, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Println("Gate rejected. Teammate unblocked to address feedback.")
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown gate command: %s\n", args[0])
		return 1
	}
}

func runInstall() int {
	cwd, _ := os.Getwd()
	binPath, _ := os.Executable()
	if err := install.Install(cwd, binPath); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Println("Gate hooks installed.")
	return 0
}

func runUninstall() int {
	cwd, _ := os.Getwd()
	if err := install.Uninstall(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Println("Gate hooks removed.")
	return 0
}

func runInit() int {
	root := gitRootOrCwd()
	dir := filepath.Join(root, "tmp")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "quest-state.json")

	if _, err := os.Stat(path); err == nil {
		s, err := state.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		s.GatePending = false
		s.GateID = nil
		if err := state.Save(path, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("State file reset (gate_pending cleared, phase preserved: %s).\n", s.Phase)
		return 0
	}

	s := &state.State{
		Version:          1,
		Phase:            "Onboard",
		AutoApproveGates: []string{},
	}
	if err := state.Save(path, s); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Println("State file created at tmp/quest-state.json")
	return 0
}

func gitRootOrCwd() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return strings.TrimSpace(string(out))
}
