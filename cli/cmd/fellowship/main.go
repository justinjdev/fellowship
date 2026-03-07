package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/cv"
	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/install"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/status"
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
	case "status":
		os.Exit(runStatus(os.Args[2:]))
	case "cv":
		os.Exit(runCV(os.Args[2:]))
	case "dashboard":
		os.Exit(runDashboard(os.Args[2:]))
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
  hook file-track        Record file touches in quest CV

Agent/lead commands:
  gate status            Show current phase, prereqs, pending state
  gate approve           Approve a pending gate (advances to next phase)
  gate reject            Reject a pending gate (clears pending, keeps phase)
  cv show [--json]       Show quest CV (phases, gates, files touched)
  status [--json]        Scan worktrees and show fellowship recovery status

Setup commands:
  install                Merge gate hooks into .claude/settings.json
  uninstall              Remove gate hooks from .claude/settings.json
  init                   Create tmp/quest-state.json with defaults

Dashboard:
  dashboard              Start live web dashboard
    --port N             HTTP port (default: 3000)
    --poll N             Poll interval in seconds (default: 5)

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
		if sr.StateChanged && !sr.Block {
			cvPath := filepath.Join(filepath.Dir(statePath), "quest-cv.json")
			hooks.RecordGateSubmitted(cvPath, s.Phase)
		}
	case "gate-prereq":
		stateChanged = hooks.GatePrereq(s, input)
	case "metadata-track":
		stateChanged = hooks.MetadataTrack(s, input)
	case "completion-guard":
		result = hooks.CompletionGuard(s, input)
		if !result.Block && input.ToolInput.Status == "completed" {
			cvPath := filepath.Join(filepath.Dir(statePath), "quest-cv.json")
			hooks.MarkCVCompleted(cvPath)
		}
	case "file-track":
		cvPath := filepath.Join(filepath.Dir(statePath), "quest-cv.json")
		hooks.FileTrack(s, input, cvPath)
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
		prevPhase := s.Phase
		s.GatePending = false
		s.Phase = nextPhase
		s.GateID = nil
		s.LembasCompleted = false
		s.MetadataUpdated = false
		if err := state.Save(statePath, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		cvPath := filepath.Join(filepath.Dir(statePath), "quest-cv.json")
		c := cv.LoadOrCreate(cvPath)
		cv.RecordGate(c, prevPhase, "approved")
		cv.Save(cvPath, c)
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
		cvPath := filepath.Join(filepath.Dir(statePath), "quest-cv.json")
		c := cv.LoadOrCreate(cvPath)
		cv.RecordGate(c, s.Phase, "rejected")
		cv.Save(cvPath, c)
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

func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	result, err := status.Scan(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Println("Fellowship Status")
	fmt.Println(strings.Repeat("\u2501", 40))

	if result.Fellowship != nil {
		fmt.Printf("Name: %s\n", result.Fellowship.Name)
		fmt.Println()
	}

	if len(result.Quests) == 0 && len(result.MergedBranches) == 0 {
		fmt.Println("No active quests found.")
		return 0
	}

	for _, q := range result.Quests {
		checkpoint := "no checkpoint"
		if q.HasCheckpoint {
			checkpoint = "checkpoint \u2713"
		}
		changes := "clean"
		if q.HasUncommitted {
			changes = "uncommitted changes"
		}
		fmt.Printf("%-20s \u2502 %-10s \u2502 %-14s \u2502 %-20s \u2502 %s\n",
			q.Name, q.Phase, checkpoint, changes, q.Classification)
	}

	if len(result.MergedBranches) > 0 {
		fmt.Println()
		fmt.Println("Merged:")
		for _, b := range result.MergedBranches {
			fmt.Printf("  %s\n", b)
		}
	}

	return 0
}

func runDashboard(args []string) int {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	port := fs.Int("port", 3000, "HTTP port")
	poll := fs.Int("poll", 5, "Poll interval in seconds")
	fs.Parse(args)

	root := gitRootOrCwd()
	srv := dashboard.NewServer(root, *poll)

	addr := fmt.Sprintf("localhost:%d", *port)
	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Fellowship dashboard: %s\n", url)

	// Open browser
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	}

	if err := http.ListenAndServe(addr, srv); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	return 0
}

func runCV(args []string) int {
	if len(args) < 1 || args[0] != "show" {
		fmt.Fprintln(os.Stderr, "usage: fellowship cv show [--dir <path>] [--json]")
		return 1
	}

	fs := flag.NewFlagSet("cv show", flag.ExitOnError)
	dir := fs.String("dir", "", "Directory to search for CV (default: auto-detect)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args[1:])

	searchDir := *dir
	if searchDir == "" {
		searchDir = gitRootOrCwd()
	}

	cvPath, err := cv.FindCV(searchDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if cvPath == "" {
		fmt.Fprintln(os.Stderr, "No quest CV found.")
		return 1
	}

	c, err := cv.Load(cvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Printf("Quest CV: %s\n", c.QuestName)
	fmt.Printf("Status:   %s\n", c.Status)
	fmt.Printf("Task:     %s\n", c.Task)
	fmt.Printf("Respawns: %d\n", c.Respawns)
	fmt.Println()

	if len(c.PhasesCompleted) > 0 {
		fmt.Println("Phases Completed:")
		for _, p := range c.PhasesCompleted {
			dur := ""
			if p.Duration != "" {
				dur = fmt.Sprintf(" (%s)", p.Duration)
			}
			fmt.Printf("  - %s at %s%s\n", p.Phase, p.CompletedAt, dur)
		}
		fmt.Println()
	}

	if len(c.GateHistory) > 0 {
		fmt.Println("Gate History:")
		for _, g := range c.GateHistory {
			reason := ""
			if g.Reason != "" {
				reason = fmt.Sprintf(" — %s", g.Reason)
			}
			fmt.Printf("  - [%s] %s %s%s\n", g.Timestamp, g.Phase, g.Action, reason)
		}
		fmt.Println()
	}

	if len(c.FilesTouched) > 0 {
		fmt.Printf("Files Touched (%d):\n", len(c.FilesTouched))
		for _, f := range c.FilesTouched {
			fmt.Printf("  - %s\n", f)
		}
	}

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
