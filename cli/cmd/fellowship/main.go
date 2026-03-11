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
	"text/tabwriter"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/autopsy"
	"github.com/justinjdev/fellowship/cli/internal/bulletin"
	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/company"
	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/eagles"
	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/hooks"

	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/status"
	"github.com/justinjdev/fellowship/cli/internal/tome"
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
	case "init":
		os.Exit(runInit())
	case "status":
		os.Exit(runStatus(os.Args[2:]))
	case "company":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship company <list|show|approve>")
			os.Exit(1)
		}
		os.Exit(runCompany(os.Args[2:]))
	case "tome":
		os.Exit(runTome(os.Args[2:]))
	case "eagles":
		os.Exit(runEagles(os.Args[2:]))
	case "bulletin":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship bulletin <post|scan|list|clear>")
			os.Exit(1)
		}
		os.Exit(runBulletin(os.Args[2:]))
	case "errand":
		os.Exit(runErrand(os.Args[2:]))
	case "state":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship state <init|add-quest|add-scout|add-company|update-quest|show>")
			os.Exit(1)
		}
		os.Exit(runState(os.Args[2:]))
	case "hold":
		os.Exit(runHold(os.Args[2:]))
	case "unhold":
		os.Exit(runUnhold(os.Args[2:]))
	case "autopsy":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship autopsy <create|scan|infer>")
			os.Exit(1)
		}
		os.Exit(runAutopsy(os.Args[2:]))
	case "herald":
		os.Exit(runHerald(os.Args[2:]))
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
  hook file-track        Record file touches in quest tome

Agent/lead commands:
  gate status            Show current phase, prereqs, pending/held state
  gate approve           Approve a pending gate (advances to next phase)
  gate reject            Reject a pending gate (clears pending, keeps phase)
  hold                   Hold (pause) a quest — blocks Edit/Write/Bash/Agent/Skill/NotebookEdit
    --dir DIR            Worktree directory (required)
    --reason MSG         Reason for holding
  unhold                 Unhold (resume) a held quest
    --dir DIR            Worktree directory (required)
  tome show [--json]     Show quest tome (phases, gates, files touched)
  status [--json]        Scan worktrees and show fellowship recovery status
  eagles                 Scan quest health and write eagles report
    --dir DIR            Git repo root (default: auto-detect)
    --threshold N        Gate pending timeout in minutes (default: 10)
    --json               Output as JSON

Setup commands:
  init                   Create quest-state.json in data directory
    --dir PATH           Worktree or repo root (default: auto-detect via git)
    --phase PHASE        Initial phase (default: Onboard)
    --plan-skip          Record Onboard/Research/Plan as skipped in tome
    --quest NAME         Quest name for tome recording

Company commands:
  company list            List all companies and their quest/scout counts
    --dir PATH            Git repo root (default: auto-detect)
  company show <name>     Show detailed company status (phases, progress)
    --dir PATH            Git repo root (default: auto-detect)
  company approve <name>  Batch-approve all pending gates in a company
    --dir PATH            Git repo root (default: auto-detect)

Fellowship state:
  state init              Create fellowship-state.json in data directory
    --dir PATH            Git repo root (default: auto-detect)
    --name NAME           Fellowship name (required)
    --base-branch BRANCH  Base branch for quest worktrees (default: auto-detected)
  state add-quest         Add a quest entry to fellowship state
    --dir PATH            Git repo root (default: auto-detect)
    --name NAME           Quest name (required)
    --task "DESC"         Task description (required)
    --branch BRANCH       Branch name
    --worktree PATH       Worktree path
    --task-id ID          Task ID
  state add-scout         Add a scout entry to fellowship state
    --dir PATH            Git repo root (default: auto-detect)
    --name NAME           Scout name (required)
    --question "Q"        Research question (required)
    --task-id ID          Task ID
  state add-company       Add a company entry to fellowship state
    --dir PATH            Git repo root (default: auto-detect)
    --name NAME           Company name (required)
    --quests q1,q2        Comma-separated quest names
    --scouts s1,s2        Comma-separated scout names
  state update-quest      Update an existing quest entry
    --dir PATH            Git repo root (default: auto-detect)
    --name NAME           Quest name (required)
    --worktree PATH       Worktree path
    --branch BRANCH       Branch name
    --task-id ID          Task ID
    --status STATUS       Quest status (active, completed, cancelled)
  state show              Show fellowship state as JSON
    --dir PATH            Git repo root (default: auto-detect)
  state clean-worktrees   Reset stale gate_pending/held flags in all worktrees
    --dir PATH            Git repo root (default: auto-detect)

Errands (persistent work items):
  errand init            Create initial quest-errands.json
    --dir PATH           Worktree directory
    --quest NAME         Quest name
    --task "DESC"        Task description
  errand list            Show all errands with status
    --dir PATH           Worktree directory
  errand add             Add a new errand
    --dir PATH           Worktree directory
    --phase PHASE        Quest phase (optional)
    "description"        Errand description (positional arg)
  errand update          Update an errand's status
    --dir PATH           Worktree directory
    <id> <status>        Item ID and new status (positional args)
  errand show            Show full errand file as JSON
    --dir PATH           Worktree directory

Bulletin (cross-quest knowledge sharing):
  bulletin post          Post a discovery to the shared bulletin board
    --quest NAME         Quest name (required)
    --topic TOPIC        Topic tag (required)
    --files FILE,FILE    Comma-separated relevant file paths
    --discovery "TEXT"   Discovery description (required)
  bulletin scan          Scan bulletin for relevant entries
    --files FILE,FILE    Comma-separated file paths to match
    --topics T1,T2       Comma-separated topics to match
    --json               Output as JSON
  bulletin list          Show all bulletin entries
    --json               Output as JSON
  bulletin clear         Clear the bulletin board

Herald (activity tidings):
  herald                 Show recent quest tidings
    --dir PATH           Git repo root (default: auto-detect)
    --problems           Show only detected problems
    --json               Output as JSON

Dashboard:
  dashboard              Start live web dashboard
    --port N             HTTP port (default: 3000)
    --poll N             Poll interval in seconds (default: 5)

Autopsy (failure memory):
  autopsy create         Write a structured failure record (reads JSON from stdin)
    --dir DIR            Git repo root (default: auto-detect)
  autopsy scan           Find autopsies matching files, modules, or tags
    --dir DIR            Git repo root (default: auto-detect)
    --files f1,f2        Comma-separated file paths to match
    --modules m1,m2      Comma-separated module names to match
    --tags t1,t2         Comma-separated tags to match
  autopsy infer          Reconstruct autopsy from worktree signals (tome, herald)
    --dir DIR            Quest worktree directory (required)
    --repo DIR           Git repo root for storing autopsy (default: auto-detect)

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

	dir := filepath.Dir(filepath.Dir(statePath)) // strip <datadir>/quest-state.json

	// Read-only hooks: no locking needed, just load and check.
	switch name {
	case "gate-guard":
		s, err := state.Load(statePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 2
		}
		result := hooks.GateGuard(s, input)
		if result.Block {
			fmt.Fprintln(os.Stderr, result.Message)
			return 2
		}
		return 0
	case "completion-guard":
		s, err := state.Load(statePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 2
		}
		result := hooks.CompletionGuard(s, input)
		if !result.Block && input.ToolInput.Status == "completed" {
			tomePath := filepath.Join(filepath.Dir(statePath), "quest-tome.json")
			hooks.MarkTomeCompleted(tomePath)
		}
		if result.Block {
			fmt.Fprintln(os.Stderr, result.Message)
			return 2
		}
		return 0
	case "file-track":
		s, err := state.Load(statePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 2
		}
		tomePath := filepath.Join(filepath.Dir(statePath), "quest-tome.json")
		hooks.FileTrack(s, input, tomePath)
		return 0
	}

	// Mutating hooks: use WithLock for atomic load→mutate→save.
	var result hooks.HookResult
	if err := state.WithLock(statePath, func(s *state.State) error {
		questName := s.QuestName
		if questName == "" {
			questName = filepath.Base(dir)
		}

		switch name {
		case "gate-submit":
			prevPhase := s.Phase
			sr := hooks.GateSubmit(s, input)
			result = hooks.HookResult{Block: sr.Block, Message: sr.Message}
			if sr.StateChanged && !sr.Block {
				tomePath := filepath.Join(filepath.Dir(statePath), "quest-tome.json")
				hooks.RecordGateSubmitted(tomePath, prevPhase, s.Phase != prevPhase)
				herald.Announce(dir, herald.Tiding{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Quest:     questName,
					Type:      herald.GateSubmitted,
					Phase:     s.Phase,
					Detail:    "Gate submitted for review",
				})
			}
			if !sr.StateChanged {
				return state.ErrNoSave
			}
		case "gate-prereq":
			changed := hooks.GatePrereq(s, input)
			if changed {
				herald.Announce(dir, herald.Tiding{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Quest:     questName,
					Type:      herald.LembasCompleted,
					Phase:     s.Phase,
					Detail:    "Lembas skill completed",
				})
			} else {
				return state.ErrNoSave
			}
		case "metadata-track":
			changed := hooks.MetadataTrack(s, input)
			if changed {
				herald.Announce(dir, herald.Tiding{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Quest:     questName,
					Type:      herald.MetadataUpdated,
					Phase:     s.Phase,
					Detail:    "Task metadata updated",
				})
			} else {
				return state.ErrNoSave
			}
		default:
			fmt.Fprintf(os.Stderr, "fellowship: unknown hook %q\n", name)
			result = hooks.HookResult{Block: true, Message: fmt.Sprintf("unknown hook %q", name)}
			return state.ErrNoSave
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 2
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
		fmt.Printf("Held:     %v\n", s.Held)
		if s.HeldReason != nil {
			fmt.Printf("Reason:   %s\n", *s.HeldReason)
		}
		fmt.Printf("Lembas:   %v\n", s.LembasCompleted)
		fmt.Printf("Metadata: %v\n", s.MetadataUpdated)
		if s.GateID != nil {
			fmt.Printf("Gate ID:  %s\n", *s.GateID)
		}
		return 0

	case "approve":
		var prevPhase, nextPhase, questName string
		if err := state.WithLock(statePath, func(s *state.State) error {
			if !s.GatePending {
				return fmt.Errorf("no gate pending")
			}
			np, err := state.NextPhase(s.Phase)
			if err != nil {
				return err
			}
			prevPhase = s.Phase
			nextPhase = np
			questName = s.QuestName
			s.GatePending = false
			s.Phase = nextPhase
			s.GateID = nil
			s.LembasCompleted = false
			s.MetadataUpdated = false
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		tomePath := filepath.Join(filepath.Dir(statePath), "quest-tome.json")
		c := tome.LoadOrCreate(tomePath)
		tome.RecordGate(c, prevPhase, "approved")
		tome.RecordPhase(c, prevPhase)
		tome.Save(tomePath, c)
		gateDir := filepath.Dir(filepath.Dir(statePath))
		if questName == "" {
			questName = filepath.Base(gateDir)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		herald.Announce(gateDir, herald.Tiding{
			Timestamp: now, Quest: questName, Type: herald.GateApproved,
			Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
		})
		herald.Announce(gateDir, herald.Tiding{
			Timestamp: now, Quest: questName, Type: herald.PhaseTransition,
			Phase: nextPhase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, nextPhase),
		})
		fmt.Printf("Gate approved. Phase advanced to %s.\n", nextPhase)
		return 0

	case "reject":
		var phase, questName string
		if err := state.WithLock(statePath, func(s *state.State) error {
			if !s.GatePending {
				return fmt.Errorf("no gate pending")
			}
			s.GatePending = false
			s.GateID = nil
			phase = s.Phase
			questName = s.QuestName
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		tomePath := filepath.Join(filepath.Dir(statePath), "quest-tome.json")
		c := tome.LoadOrCreate(tomePath)
		tome.RecordGate(c, phase, "rejected")
		tome.Save(tomePath, c)
		rejDir := filepath.Dir(filepath.Dir(statePath))
		if questName == "" {
			questName = filepath.Base(rejDir)
		}
		herald.Announce(rejDir, herald.Tiding{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest: questName, Type: herald.GateRejected,
			Phase: phase, Detail: fmt.Sprintf("Gate rejected for %s", phase),
		})
		fmt.Println("Gate rejected. Teammate unblocked to address feedback.")
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown gate command: %s\n", args[0])
		return 1
	}
}


func runHold(args []string) int {
	fs := flag.NewFlagSet("hold", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (required)")
	reason := fs.String("reason", "", "Reason for holding the quest")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship hold --dir <worktree> [--reason \"message\"]")
		return 1
	}

	statePath := filepath.Join(*dir, datadir.Name(), "quest-state.json")
	var questName, phase string
	if err := state.WithLock(statePath, func(s *state.State) error {
		if s.Held {
			return fmt.Errorf("quest is already held")
		}
		s.Held = true
		if *reason != "" {
			s.HeldReason = reason
		}
		questName = s.QuestName
		phase = s.Phase
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if questName == "" {
		questName = filepath.Base(*dir)
	}
	detail := "Quest held"
	if *reason != "" {
		detail += ": " + *reason
	}
	herald.Announce(*dir, herald.Tiding{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Quest:     questName,
		Type:      herald.QuestHeld,
		Phase:     phase,
		Detail:    detail,
	})

	fmt.Printf("Quest held.%s\n", func() string {
		if *reason != "" {
			return " Reason: " + *reason
		}
		return ""
	}())
	return 0
}

func runUnhold(args []string) int {
	fs := flag.NewFlagSet("unhold", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (required)")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship unhold --dir <worktree>")
		return 1
	}

	statePath := filepath.Join(*dir, datadir.Name(), "quest-state.json")
	var questName, phase string
	if err := state.WithLock(statePath, func(s *state.State) error {
		if !s.Held {
			return fmt.Errorf("quest is not held")
		}
		s.Held = false
		s.HeldReason = nil
		questName = s.QuestName
		phase = s.Phase
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if questName == "" {
		questName = filepath.Base(*dir)
	}
	herald.Announce(*dir, herald.Tiding{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Quest:     questName,
		Type:      herald.QuestUnheld,
		Phase:     phase,
		Detail:    "Quest unheld — resumed",
	})

	fmt.Println("Quest unheld.")
	return 0
}

func runInit() int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	phase := fs.String("phase", "", "Initial phase (default: Onboard)")
	planSkip := fs.Bool("plan-skip", false, "Record Onboard/Research/Plan as skipped in tome")
	questName := fs.String("quest", "", "Quest name for tome recording")
	initDir := fs.String("dir", "", "Worktree or repo root (default: auto-detect via git)")
	fs.Parse(os.Args[2:])

	validPhases := map[string]bool{
		"Onboard": true, "Research": true, "Plan": true,
		"Implement": true, "Review": true, "Complete": true,
	}
	if *phase != "" && !validPhases[*phase] {
		fmt.Fprintf(os.Stderr, "fellowship: invalid phase %q\n", *phase)
		return 1
	}

	if *planSkip && *phase == "" {
		*phase = "Implement"
	}
	if *planSkip && *phase != "Implement" {
		fmt.Fprintln(os.Stderr, "fellowship: --plan-skip requires --phase Implement")
		return 1
	}

	root := *initDir
	if root == "" {
		root = gitRootOrCwd()
	}
	dataDir := filepath.Join(root, datadir.Name())
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: creating data directory: %v\n", err)
		return 1
	}
	path := filepath.Join(dataDir, "quest-state.json")

	if _, err := os.Stat(path); err == nil {
		s, err := state.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		s.GatePending = false
		s.GateID = nil
		if *phase != "" {
			s.Phase = *phase
			s.LembasCompleted = false
			s.MetadataUpdated = false
		}
		if err := state.Save(path, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("State file reset (gate_pending cleared, phase: %s).\n", s.Phase)
	} else {
		initPhase := "Onboard"
		if *phase != "" {
			initPhase = *phase
		}
		s := &state.State{
			Version:          1,
			Phase:            initPhase,
			AutoApproveGates: []string{},
		}
		if err := state.Save(path, s); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("State file created at %s/quest-state.json (phase: %s)\n", datadir.Name(), initPhase)
	}

	if *planSkip {
		tomePath := filepath.Join(dataDir, "quest-tome.json")
		c := tome.LoadOrCreate(tomePath)
		if *questName != "" {
			c.QuestName = *questName
		}
		tome.RecordSkippedPhases(c, []string{"Onboard", "Research", "Plan"}, "pre-existing plan")
		if err := tome.Save(tomePath, c); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Println("Recorded Onboard/Research/Plan as skipped (pre-existing plan).")
	}

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

func runEagles(args []string) int {
	fs := flag.NewFlagSet("eagles", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	threshold := fs.Int("threshold", 10, "Gate pending timeout in minutes")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	opts := eagles.DefaultOptions()
	opts.GateThreshold = time.Duration(*threshold) * time.Minute

	report, err := eagles.Sweep(root, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	// Write report to data directory
	if err := eagles.WriteReport(root, report); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: warning: %v\n", err)
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Print(eagles.FormatTable(report))
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

func runAutopsy(args []string) int {
	switch args[0] {
	case "create":
		return runAutopsyCreate(args[1:])
	case "scan":
		return runAutopsyScan(args[1:])
	case "infer":
		return runAutopsyInfer(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown autopsy command: %s\n", args[0])
		return 1
	}
}

func runAutopsyCreate(args []string) int {
	fs := flag.NewFlagSet("autopsy create", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	var input autopsy.CreateInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: reading JSON from stdin: %v\n", err)
		return 1
	}

	path, err := autopsy.Create(root, &input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Autopsy written to %s\n", path)
	return 0
}

func runAutopsyScan(args []string) int {
	fs := flag.NewFlagSet("autopsy scan", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	files := fs.String("files", "", "Comma-separated file paths to match")
	modules := fs.String("modules", "", "Comma-separated module names to match")
	tags := fs.String("tags", "", "Comma-separated tags to match")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	opts := autopsy.ScanOptions{}
	if *files != "" {
		opts.Files = strings.Split(*files, ",")
	}
	if *modules != "" {
		opts.Modules = strings.Split(*modules, ",")
	}
	if *tags != "" {
		opts.Tags = strings.Split(*tags, ",")
	}

	expiryDays := datadir.AutopsyExpiryDays(autopsy.DefaultExpiryDays)
	matches, err := autopsy.Scan(root, opts, expiryDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(matches, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runAutopsyInfer(args []string) int {
	fs := flag.NewFlagSet("autopsy infer", flag.ExitOnError)
	dir := fs.String("dir", "", "Quest worktree directory (required)")
	repo := fs.String("repo", "", "Git repo root for storing autopsy (default: auto-detect)")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship autopsy infer --dir <worktree> [--repo DIR]")
		return 1
	}

	root := *repo
	if root == "" {
		root = gitRootOrCwd()
	}

	path, err := autopsy.Infer(*dir, root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Inferred autopsy written to %s\n", path)
	return 0
}

func runHerald(args []string) int {
	fs := flag.NewFlagSet("herald", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	problems := fs.Bool("problems", false, "Show only detected problems")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	ds, err := dashboard.DiscoverQuests(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	var worktrees []string
	for _, q := range ds.Quests {
		worktrees = append(worktrees, q.Worktree)
	}

	if *problems {
		detected := herald.DetectProblems(worktrees)
		if *jsonOut {
			data, _ := json.MarshalIndent(detected, "", "  ")
			fmt.Println(string(data))
			return 0
		}
		if len(detected) == 0 {
			fmt.Println("No problems detected.")
			return 0
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(tw, "SEVERITY\tQUEST\tTYPE\tMESSAGE\n")
		for _, p := range detected {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Severity, p.Quest, p.Type, p.Message)
		}
		tw.Flush()
		return 0
	}

	evts, err := herald.ReadAll(worktrees, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(evts, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(evts) == 0 {
		fmt.Println("No tidings.")
		return 0
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "TIME\tQUEST\tTYPE\tPHASE\tDETAIL\n")
	for _, e := range evts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.Timestamp, e.Quest, e.Type, e.Phase, e.Detail)
	}
	tw.Flush()
	return 0
}

func runCompany(args []string) int {
	sub := args[0]
	rest := args[1:]

	switch sub {
	case "list":
		fs := flag.NewFlagSet("company list", flag.ExitOnError)
		dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
		fs.Parse(rest)

		root := *dir
		if root == "" {
			root = gitRootOrCwd()
		}
		statePath := filepath.Join(root, datadir.Name(), "fellowship-state.json")
		if err := company.List(statePath); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	case "show":
		fs := flag.NewFlagSet("company show", flag.ExitOnError)
		dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
		fs.Parse(rest)

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fellowship company show <name> [--dir PATH]")
			return 1
		}
		name := fs.Arg(0)

		root := *dir
		if root == "" {
			root = gitRootOrCwd()
		}
		statePath := filepath.Join(root, datadir.Name(), "fellowship-state.json")
		if err := company.Show(statePath, name); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	case "approve":
		fs := flag.NewFlagSet("company approve", flag.ExitOnError)
		dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
		fs.Parse(rest)

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fellowship company approve <name> [--dir PATH]")
			return 1
		}
		name := fs.Arg(0)

		root := *dir
		if root == "" {
			root = gitRootOrCwd()
		}
		statePath := filepath.Join(root, datadir.Name(), "fellowship-state.json")
		if err := company.Approve(statePath, name); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown company command: %s\n", sub)
		return 1
	}
}

func runTome(args []string) int {
	if len(args) < 1 || args[0] != "show" {
		fmt.Fprintln(os.Stderr, "usage: fellowship tome show [--dir <path>] [--json]")
		return 1
	}

	fs := flag.NewFlagSet("tome show", flag.ExitOnError)
	dir := fs.String("dir", "", "Directory to search for tome (default: auto-detect)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args[1:])

	searchDir := *dir
	if searchDir == "" {
		searchDir = gitRootOrCwd()
	}

	tomePath, err := tome.FindTome(searchDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if tomePath == "" {
		fmt.Fprintln(os.Stderr, "No quest tome found.")
		return 1
	}

	c, err := tome.Load(tomePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Printf("Quest Tome: %s\n", c.QuestName)
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

func runErrand(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand <init|list|add|update|show>")
		return 1
	}

	switch args[0] {
	case "init":
		return runErrandInit(args[1:])
	case "list":
		return runErrandList(args[1:])
	case "add":
		return runErrandAdd(args[1:])
	case "update":
		return runErrandUpdate(args[1:])
	case "show":
		return runErrandShow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown errand command: %s\n", args[0])
		return 1
	}
}

func runErrandInit(args []string) int {
	fs := flag.NewFlagSet("errand init", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory")
	quest := fs.String("quest", "", "Quest name")
	task := fs.String("task", "", "Task description")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	errandDir := filepath.Join(root, datadir.Name())
	if err := os.MkdirAll(errandDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: creating data directory: %v\n", err)
		return 1
	}
	errandPath := filepath.Join(errandDir, "quest-errands.json")

	if _, err := os.Stat(errandPath); err == nil {
		fmt.Fprintln(os.Stderr, "fellowship: quest-errands.json already exists")
		return 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	h := &errand.QuestErrandList{
		Version:   1,
		QuestName: *quest,
		Task:      *task,
		Items:     []errand.Errand{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := errand.Save(errandPath, h); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Errand file created at %s\n", errandPath)
	return 0
}

func runErrandList(args []string) int {
	fs := flag.NewFlagSet("errand list", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory")
	fs.Parse(args)

	h, _, err := loadErrandFile(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if len(h.Items) == 0 {
		fmt.Println("No errands.")
		return 0
	}

	for _, item := range h.Items {
		phase := ""
		if item.Phase != "" {
			phase = fmt.Sprintf(" [%s]", item.Phase)
		}
		deps := ""
		if len(item.DependsOn) > 0 {
			deps = fmt.Sprintf(" (depends: %s)", strings.Join(item.DependsOn, ", "))
		}
		fmt.Printf("%-6s %-8s %s%s%s\n", item.ID, item.Status, item.Description, phase, deps)
	}

	done, total := errand.Progress(h)
	fmt.Printf("\nProgress: %d/%d done\n", done, total)
	return 0
}

func runErrandAdd(args []string) int {
	fs := flag.NewFlagSet("errand add", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory")
	phase := fs.String("phase", "", "Quest phase")
	fs.Parse(args)

	desc := strings.Join(fs.Args(), " ")
	if desc == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand add --dir <path> \"description\"")
		return 1
	}

	h, errandPath, err := loadErrandFile(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	id := errand.AddErrand(h, desc, *phase)
	if err := errand.Save(errandPath, h); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added %s: %s\n", id, desc)
	return 0
}

func runErrandUpdate(args []string) int {
	fs := flag.NewFlagSet("errand update", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand update --dir <path> <id> <status>")
		return 1
	}

	id := remaining[0]
	statusStr := remaining[1]

	ws, ok := errand.ValidStatus(statusStr)
	if !ok {
		fmt.Fprintf(os.Stderr, "fellowship: invalid status %q (use: pending, active, done, blocked)\n", statusStr)
		return 1
	}

	h, errandPath, err := loadErrandFile(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if err := errand.UpdateStatus(h, id, ws); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if err := errand.Save(errandPath, h); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Updated %s → %s\n", id, statusStr)
	return 0
}

func runErrandShow(args []string) int {
	fs := flag.NewFlagSet("errand show", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory")
	fs.Parse(args)

	h, _, err := loadErrandFile(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(h, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runState(args []string) int {
	switch args[0] {
	case "init":
		return runStateInit(args[1:])
	case "add-quest":
		return runStateAddQuest(args[1:])
	case "add-scout":
		return runStateAddScout(args[1:])
	case "update-quest":
		return runStateUpdateQuest(args[1:])
	case "add-company":
		return runStateAddCompany(args[1:])
	case "show":
		return runStateShow(args[1:])
	case "clean-worktrees":
		return runStateCleanWorktrees(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown state command: %s\n", args[0])
		return 1
	}
}

func runStateInit(args []string) int {
	fs := flag.NewFlagSet("state init", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	name := fs.String("name", "", "Fellowship name (required)")
	baseBranch := fs.String("base-branch", "", "Base branch for quest worktrees (Gandalf detects automatically; use this to override)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state init --name <name> [--dir PATH] [--base-branch BRANCH]")
		return 1
	}

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	dataDirPath := filepath.Join(root, datadir.Name())
	if err := os.MkdirAll(dataDirPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: creating data directory: %v\n", err)
		return 1
	}
	statePath := filepath.Join(dataDirPath, "fellowship-state.json")

	if _, err := os.Stat(statePath); err == nil {
		if existing, loadErr := dashboard.LoadFellowshipState(statePath); loadErr == nil {
			fmt.Fprintf(os.Stderr, "fellowship: warning: overwriting existing fellowship-state.json (name=%q, quests=%d)\n",
				existing.Name, len(existing.Quests))
		} else {
			fmt.Fprintln(os.Stderr, "fellowship: warning: overwriting existing fellowship-state.json")
		}
	}

	s := &dashboard.FellowshipState{
		Version:    1,
		Name:       *name,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		MainRepo:   root,
		BaseBranch: *baseBranch,
		Quests:     []dashboard.QuestEntry{},
		Scouts:     []dashboard.ScoutEntry{},
		Companies:  []dashboard.CompanyEntry{},
	}
	if err := dashboard.SaveFellowshipState(statePath, s); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Fellowship state created at %s\n", statePath)
	return 0
}

func runStateAddQuest(args []string) int {
	fs := flag.NewFlagSet("state add-quest", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	name := fs.String("name", "", "Quest name (required)")
	task := fs.String("task", "", "Task description (required)")
	branch := fs.String("branch", "", "Branch name")
	worktree := fs.String("worktree", "", "Worktree path")
	taskID := fs.String("task-id", "", "Task ID")
	fs.Parse(args)

	if *name == "" || *task == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-quest --name <name> --task \"<desc>\" [--dir PATH] [--branch BRANCH] [--worktree PATH] [--task-id ID]")
		return 1
	}

	statePath := fellowshipStatePath(*dir)
	questName := *name
	if err := dashboard.WithStateLock(statePath, func(s *dashboard.FellowshipState) error {
		for _, q := range s.Quests {
			if q.Name == questName {
				return fmt.Errorf("quest %q already exists", questName)
			}
		}
		s.Quests = append(s.Quests, dashboard.QuestEntry{
			Name:            *name,
			TaskDescription: *task,
			Worktree:        *worktree,
			Branch:          *branch,
			TaskID:          *taskID,
		})
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added quest %q\n", *name)
	return 0
}

func runStateAddScout(args []string) int {
	fs := flag.NewFlagSet("state add-scout", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	name := fs.String("name", "", "Scout name (required)")
	question := fs.String("question", "", "Research question (required)")
	taskID := fs.String("task-id", "", "Task ID")
	fs.Parse(args)

	if *name == "" || *question == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-scout --name <name> --question \"<question>\" [--dir PATH] [--task-id ID]")
		return 1
	}

	statePath := fellowshipStatePath(*dir)
	scoutName := *name
	if err := dashboard.WithStateLock(statePath, func(s *dashboard.FellowshipState) error {
		for _, sc := range s.Scouts {
			if sc.Name == scoutName {
				return fmt.Errorf("scout %q already exists", scoutName)
			}
		}
		s.Scouts = append(s.Scouts, dashboard.ScoutEntry{
			Name:     *name,
			Question: *question,
			TaskID:   *taskID,
		})
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added scout %q\n", *name)
	return 0
}

func runStateAddCompany(args []string) int {
	fs := flag.NewFlagSet("state add-company", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	name := fs.String("name", "", "Company name (required)")
	quests := fs.String("quests", "", "Comma-separated quest names")
	scouts := fs.String("scouts", "", "Comma-separated scout names")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-company --name <name> [--quests q1,q2] [--scouts s1,s2] [--dir PATH]")
		return 1
	}

	statePath := fellowshipStatePath(*dir)
	companyName := *name
	if err := dashboard.WithStateLock(statePath, func(s *dashboard.FellowshipState) error {
		for _, c := range s.Companies {
			if c.Name == companyName {
				return fmt.Errorf("company %q already exists", companyName)
			}
		}
		entry := dashboard.CompanyEntry{Name: *name}
		if *quests != "" {
			entry.Quests = strings.Split(*quests, ",")
		} else {
			entry.Quests = []string{}
		}
		if *scouts != "" {
			entry.Scouts = strings.Split(*scouts, ",")
		} else {
			entry.Scouts = []string{}
		}
		s.Companies = append(s.Companies, entry)
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added company %q\n", *name)
	return 0
}

func runStateUpdateQuest(args []string) int {
	fs := flag.NewFlagSet("state update-quest", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	name := fs.String("name", "", "Quest name (required)")
	worktree := fs.String("worktree", "", "Worktree path")
	branch := fs.String("branch", "", "Branch name")
	taskID := fs.String("task-id", "", "Task ID")
	statusFlag := fs.String("status", "", "Quest status (active, completed, cancelled)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state update-quest --name <name> [--worktree PATH] [--branch BRANCH] [--task-id ID] [--status STATUS] [--dir PATH]")
		return 1
	}

	if *statusFlag != "" && *statusFlag != "active" && *statusFlag != "completed" && *statusFlag != "cancelled" {
		fmt.Fprintf(os.Stderr, "fellowship: invalid status %q (must be active, completed, or cancelled)\n", *statusFlag)
		return 1
	}

	statePath := fellowshipStatePath(*dir)
	questName := *name
	if err := dashboard.WithStateLock(statePath, func(s *dashboard.FellowshipState) error {
		for i := range s.Quests {
			if s.Quests[i].Name == questName {
				if *worktree != "" {
					s.Quests[i].Worktree = *worktree
				}
				if *branch != "" {
					s.Quests[i].Branch = *branch
				}
				if *taskID != "" {
					s.Quests[i].TaskID = *taskID
				}
				if *statusFlag != "" {
					s.Quests[i].Status = *statusFlag
				}
				return nil
			}
		}
		return fmt.Errorf("quest %q not found", questName)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Updated quest %q\n", *name)
	return 0
}

func runStateShow(args []string) int {
	fs := flag.NewFlagSet("state show", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	fs.Parse(args)

	statePath := fellowshipStatePath(*dir)
	s, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runStateCleanWorktrees(args []string) int {
	fs := flag.NewFlagSet("state clean-worktrees", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	worktreesDir := filepath.Join(root, ".claude", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No worktrees directory found.")
			return 0
		}
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	cleaned := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		statePath := filepath.Join(worktreesDir, entry.Name(), datadir.Name(), "quest-state.json")
		if _, err := os.Stat(statePath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "fellowship: warning: could not access %s: %v\n", statePath, err)
			}
			continue
		}
		var prevPending, prevHeld bool
		err := state.WithLock(statePath, func(s *state.State) error {
			if !s.GatePending && !s.Held {
				return state.ErrNoSave
			}
			prevPending, prevHeld = s.GatePending, s.Held
			s.GatePending = false
			s.GateID = nil
			s.Held = false
			s.HeldReason = nil
			return nil
		})
		if err == state.ErrNoSave {
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: warning: could not save %s: %v\n", statePath, err)
			continue
		}
		fmt.Printf("Cleared stale state in %s (gate_pending=%v, held=%v)\n", entry.Name(), prevPending, prevHeld)
		cleaned++
	}

	if cleaned == 0 {
		fmt.Println("No stale state found.")
	} else {
		fmt.Printf("Cleaned %d worktree(s).\n", cleaned)
	}
	return 0
}

func fellowshipStatePath(dir string) string {
	root := dir
	if root == "" {
		root = gitRootOrCwd()
	}
	return filepath.Join(root, datadir.Name(), "fellowship-state.json")
}

func loadErrandFile(dir string) (*errand.QuestErrandList, string, error) {
	root := dir
	if root == "" {
		root = gitRootOrCwd()
	}
	errandPath := filepath.Join(root, datadir.Name(), "quest-errands.json")
	h, err := errand.Load(errandPath)
	if err != nil {
		return nil, "", err
	}
	return h, errandPath, nil
}

func runBulletin(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fellowship bulletin <post|scan|list|clear>")
		return 1
	}
	switch args[0] {
	case "post":
		return runBulletinPost(args[1:])
	case "scan":
		return runBulletinScan(args[1:])
	case "list":
		return runBulletinList(args[1:])
	case "clear":
		return runBulletinClear()
	default:
		fmt.Fprintf(os.Stderr, "unknown bulletin command: %s\n", args[0])
		return 1
	}
}

func runBulletinPost(args []string) int {
	fs := flag.NewFlagSet("bulletin post", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	topic := fs.String("topic", "", "Topic tag")
	files := fs.String("files", "", "Comma-separated file paths")
	discovery := fs.String("discovery", "", "Discovery description")
	fs.Parse(args)

	if *quest == "" || *topic == "" || *discovery == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship bulletin post --quest NAME --topic TOPIC --discovery \"TEXT\" [--files FILE,FILE]")
		return 1
	}

	var fileList []string
	if *files != "" {
		fileList = strings.Split(*files, ",")
	}

	path, err := bulletin.BulletinPath("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	entry := bulletin.Entry{
		Quest:     *quest,
		Topic:     *topic,
		Files:     fileList,
		Discovery: *discovery,
	}
	if err := bulletin.Post(path, entry); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Posted to bulletin: [%s] %s\n", *topic, *discovery)
	return 0
}

func runBulletinScan(args []string) int {
	fs := flag.NewFlagSet("bulletin scan", flag.ExitOnError)
	files := fs.String("files", "", "Comma-separated file paths to match")
	topics := fs.String("topics", "", "Comma-separated topics to match")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	var fileList, topicList []string
	if *files != "" {
		fileList = strings.Split(*files, ",")
	}
	if *topics != "" {
		topicList = strings.Split(*topics, ",")
	}

	path, err := bulletin.BulletinPath("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	entries, err := bulletin.Scan(path, fileList, topicList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(entries) == 0 {
		fmt.Println("No matching bulletin entries.")
		return 0
	}

	for _, e := range entries {
		fmt.Printf("[%s] %s (%s): %s\n", e.Topic, e.Quest, strings.Join(e.Files, ", "), e.Discovery)
	}
	fmt.Printf("\n%d entries found.\n", len(entries))
	return 0
}

func runBulletinList(args []string) int {
	fs := flag.NewFlagSet("bulletin list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	path, err := bulletin.BulletinPath("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	entries, err := bulletin.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		if entries == nil {
			entries = []bulletin.Entry{}
		}
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(entries) == 0 {
		fmt.Println("No bulletin entries.")
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tQUEST\tTOPIC\tDISCOVERY")
	for _, e := range entries {
		ts := e.Timestamp
		if len(ts) > 19 {
			ts = ts[:19]
		}
		disc := e.Discovery
		if len(disc) > 60 {
			disc = disc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ts, e.Quest, e.Topic, disc)
	}
	w.Flush()
	fmt.Printf("\n%d entries total.\n", len(entries))
	return 0
}

func runBulletinClear() int {
	path, err := bulletin.BulletinPath("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if err := bulletin.Clear(path); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Println("Bulletin cleared.")
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
