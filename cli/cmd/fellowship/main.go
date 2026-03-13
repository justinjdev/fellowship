package main

import (
	"context"
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
	"github.com/justinjdev/fellowship/cli/internal/company"
	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/db"
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

	// Commands that don't need DB.
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
		return
	case "migrate":
		// TODO: implement migration command
		fmt.Fprintln(os.Stderr, "fellowship: migrate command not yet implemented")
		os.Exit(1)
	}

	// Open DB for all other commands.
	cwd, _ := os.Getwd()
	d, err := db.Open(cwd)
	if err != nil {
		if jsonFilesExist(cwd) {
			fmt.Fprintln(os.Stderr, `fellowship: Run "fellowship migrate" to upgrade to the new storage format.`)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	switch os.Args[1] {
	case "hook":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship hook <name>")
			os.Exit(1)
		}
		os.Exit(runHook(d, os.Args[2]))
	case "gate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship gate <status|approve|reject>")
			os.Exit(1)
		}
		os.Exit(runGate(d, os.Args[2:]))
	case "init":
		os.Exit(runInit(d))
	case "status":
		os.Exit(runStatus(d, os.Args[2:]))
	case "company":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship company <list|show|approve>")
			os.Exit(1)
		}
		os.Exit(runCompany(d, os.Args[2:]))
	case "tome":
		os.Exit(runTome(d, os.Args[2:]))
	case "eagles":
		os.Exit(runEagles(d, os.Args[2:]))
	case "bulletin":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship bulletin <post|scan|list|clear>")
			os.Exit(1)
		}
		os.Exit(runBulletin(d, os.Args[2:]))
	case "errand":
		os.Exit(runErrand(d, os.Args[2:]))
	case "state":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship state <init|add-quest|add-scout|add-company|update-quest|show>")
			os.Exit(1)
		}
		os.Exit(runState(d, os.Args[2:]))
	case "hold":
		os.Exit(runHold(d, os.Args[2:]))
	case "unhold":
		os.Exit(runUnhold(d, os.Args[2:]))
	case "autopsy":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship autopsy <create|scan|infer>")
			os.Exit(1)
		}
		os.Exit(runAutopsy(d, os.Args[2:]))
	case "herald":
		os.Exit(runHerald(d, os.Args[2:]))
	case "dashboard":
		os.Exit(runDashboard(d, os.Args[2:]))
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
  init                   Initialize quest state in DB
    --dir PATH           Worktree or repo root (default: auto-detect via git)
    --phase PHASE        Initial phase (default: Onboard)
    --plan-skip          Record Onboard/Research/Plan as skipped in tome
    --quest NAME         Quest name for tome recording

Company commands:
  company list            List all companies and their quest/scout counts
  company show <name>     Show detailed company status (phases, progress)
  company approve <name>  Batch-approve all pending gates in a company

Fellowship state:
  state init              Initialize fellowship in DB
    --name NAME           Fellowship name (required)
    --base-branch BRANCH  Base branch for quest worktrees (default: auto-detected)
  state add-quest         Add a quest entry to fellowship state
    --name NAME           Quest name (required)
    --task "DESC"         Task description (required)
    --branch BRANCH       Branch name
    --worktree PATH       Worktree path
    --task-id ID          Task ID
  state add-scout         Add a scout entry to fellowship state
    --name NAME           Scout name (required)
    --question "Q"        Research question (required)
    --task-id ID          Task ID
  state add-company       Add a company entry to fellowship state
    --name NAME           Company name (required)
    --quests q1,q2        Comma-separated quest names
    --scouts s1,s2        Comma-separated scout names
  state update-quest      Update an existing quest entry
    --name NAME           Quest name (required)
    --worktree PATH       Worktree path
    --branch BRANCH       Branch name
    --task-id ID          Task ID
    --status STATUS       Quest status (active, completed, cancelled)
  state show              Show fellowship state as JSON
  state clean-worktrees   Reset stale gate_pending/held flags in all quests

Errands (persistent work items):
  errand init            Initialize errands for a quest
    --quest NAME         Quest name
    --task "DESC"        Task description
  errand list            Show all errands with status
    --quest NAME         Quest name
  errand add             Add a new errand
    --quest NAME         Quest name
    --phase PHASE        Quest phase (optional)
    "description"        Errand description (positional arg)
  errand update          Update an errand's status
    --quest NAME         Quest name
    <id> <status>        Item ID and new status (positional args)
  errand show            Show all errands as JSON
    --quest NAME         Quest name

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
    --problems           Show only detected problems
    --json               Output as JSON

Dashboard:
  dashboard              Start live web dashboard
    --port N             HTTP port (default: 3000)
    --poll N             Poll interval in seconds (default: 5)

Autopsy (failure memory):
  autopsy create         Write a structured failure record (reads JSON from stdin)
  autopsy scan           Find autopsies matching files, modules, or tags
    --files f1,f2        Comma-separated file paths to match
    --modules m1,m2      Comma-separated module names to match
    --tags t1,t2         Comma-separated tags to match
  autopsy infer          Reconstruct autopsy from quest signals
    --quest NAME         Quest name (required)

Other:
  migrate                Migrate JSON files to SQLite
  version                Print version`)
}

func runHook(d *db.DB, name string) int {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	gitRoot := gitRootFrom(cwd)

	// Find quest name for this worktree.
	var questName string
	d.WithConn(ctx, func(conn *db.Conn) error {
		questName, _ = state.FindQuest(conn, gitRoot)
		return nil
	})
	// Lead session (no quest found): only the CWD guard applies.
	if questName == "" {
		if name == "gate-guard" {
			input, err := hooks.ParseInput(os.Stdin)
			if err != nil {
				input = &hooks.HookInput{}
			}
			if result := hooks.WorktreeGuard(input); result.Block {
				fmt.Fprintln(os.Stderr, result.Message)
				return 2
			}
		}
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

	// Read-only hooks: use WithConn.
	switch name {
	case "gate-guard":
		var result hooks.HookResult
		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			result = hooks.GateGuard(s, input)
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

	case "gate-prereq":
		var result hooks.HookResult
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			changed := hooks.GatePrereq(s, input)
			if changed {
				if err := state.Upsert(conn, s); err != nil {
					return err
				}
				herald.Announce(conn, herald.Tiding{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Quest:     questName,
					Type:      herald.LembasCompleted,
					Phase:     s.Phase,
					Detail:    "Lembas skill completed",
				})
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

	case "completion-guard":
		var result hooks.HookResult
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			result = hooks.CompletionGuard(s, input)
			if !result.Block && input.ToolInput.Status == "completed" {
				hooks.MarkTomeCompleted(conn, questName)
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

	case "file-track":
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			hooks.FileTrack(conn, s, input, questName)
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 2
		}
		return 0

	case "gate-submit":
		var result hooks.HookResult
		var gateSubmitEnrich bool
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			prevPhase := s.Phase
			sr := hooks.GateSubmit(s, input)
			result = hooks.HookResult{Block: sr.Block, Message: sr.Message}
			if sr.StateChanged {
				if err := state.Upsert(conn, s); err != nil {
					return err
				}
				if !sr.Block {
					gateSubmitEnrich = true
					hooks.RecordGateSubmitted(conn, questName, prevPhase, s.Phase != prevPhase)
					herald.Announce(conn, herald.Tiding{
						Timestamp: time.Now().UTC().Format(time.RFC3339),
						Quest:     questName,
						Type:      herald.GateSubmitted,
						Phase:     s.Phase,
						Detail:    "Gate submitted for review",
					})
				}
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 2
		}
		if result.Block {
			out := hooks.NewDenyOutput(result.Message)
			json.NewEncoder(os.Stdout).Encode(out)
			return 0 // exit 0 with JSON deny — Claude Code reads the JSON
		}
		if gateSubmitEnrich {
			var enrichment string
			d.WithConn(ctx, func(conn *db.Conn) error {
				enrichment = hooks.GatherEnrichment(conn, questName, gitRoot)
				return nil
			})
			if enrichment != "" {
				enrichedContent := input.ToolInput.Content + enrichment
				out := hooks.NewAllowOutput(map[string]string{"content": enrichedContent})
				json.NewEncoder(os.Stdout).Encode(out)
			}
		}
		return 0

	case "metadata-track":
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			changed := hooks.MetadataTrack(s, input)
			if changed {
				if err := state.Upsert(conn, s); err != nil {
					return err
				}
				herald.Announce(conn, herald.Tiding{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Quest:     questName,
					Type:      herald.MetadataUpdated,
					Phase:     s.Phase,
					Detail:    "Task metadata updated",
				})
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 2
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "fellowship: unknown hook %q\n", name)
		return 2
	}
}

func runGate(d *db.DB, args []string) int {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	gitRoot := gitRootFrom(cwd)

	var questName string
	d.WithConn(ctx, func(conn *db.Conn) error {
		questName, _ = state.FindQuest(conn, gitRoot)
		return nil
	})
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest state found")
		return 1
	}

	switch args[0] {
	case "status":
		var s *state.State
		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			var err error
			s, err = state.Load(conn, questName)
			return err
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
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
		var prevPhase, nextPhase string
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			if !s.GatePending {
				return fmt.Errorf("no gate pending")
			}
			np, err := state.NextPhase(s.Phase)
			if err != nil {
				return err
			}
			prevPhase = s.Phase
			nextPhase = np
			s.GatePending = false
			s.Phase = nextPhase
			s.GateID = nil
			s.LembasCompleted = false
			s.MetadataUpdated = false
			if err := state.Upsert(conn, s); err != nil {
				return err
			}

			tome.RecordGate(conn, questName, prevPhase, "approved", "")
			tome.RecordPhase(conn, questName, prevPhase, 0)

			now := time.Now().UTC().Format(time.RFC3339)
			herald.Announce(conn, herald.Tiding{
				Timestamp: now, Quest: questName, Type: herald.GateApproved,
				Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
			})
			herald.Announce(conn, herald.Tiding{
				Timestamp: now, Quest: questName, Type: herald.PhaseTransition,
				Phase: nextPhase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, nextPhase),
			})
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("Gate approved. Phase advanced to %s.\n", nextPhase)
		return 0

	case "reject":
		var phase string
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			if !s.GatePending {
				return fmt.Errorf("no gate pending")
			}
			s.GatePending = false
			s.GateID = nil
			phase = s.Phase
			if err := state.Upsert(conn, s); err != nil {
				return err
			}

			tome.RecordGate(conn, questName, phase, "rejected", "")

			herald.Announce(conn, herald.Tiding{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Quest:     questName, Type: herald.GateRejected,
				Phase: phase, Detail: fmt.Sprintf("Gate rejected for %s", phase),
			})
			return nil
		}); err != nil {
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

func runHold(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("hold", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (required)")
	reason := fs.String("reason", "", "Reason for holding the quest")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship hold --dir <worktree> [--reason \"message\"]")
		return 1
	}

	// Find quest for the given worktree dir.
	var questName string
	d.WithConn(ctx, func(conn *db.Conn) error {
		questName, _ = state.FindQuest(conn, *dir)
		return nil
	})
	if questName == "" {
		questName = filepath.Base(*dir)
	}

	var phase string
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		s, err := state.Load(conn, questName)
		if err != nil {
			return err
		}
		if s.Held {
			return fmt.Errorf("quest is already held")
		}
		s.Held = true
		if *reason != "" {
			s.HeldReason = reason
		}
		phase = s.Phase
		if err := state.Upsert(conn, s); err != nil {
			return err
		}

		detail := "Quest held"
		if *reason != "" {
			detail += ": " + *reason
		}
		herald.Announce(conn, herald.Tiding{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     questName,
			Type:      herald.QuestHeld,
			Phase:     phase,
			Detail:    detail,
		})
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	fmt.Printf("Quest held.%s\n", func() string {
		if *reason != "" {
			return " Reason: " + *reason
		}
		return ""
	}())
	return 0
}

func runUnhold(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("unhold", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (required)")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship unhold --dir <worktree>")
		return 1
	}

	var questName string
	d.WithConn(ctx, func(conn *db.Conn) error {
		questName, _ = state.FindQuest(conn, *dir)
		return nil
	})
	if questName == "" {
		questName = filepath.Base(*dir)
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		s, err := state.Load(conn, questName)
		if err != nil {
			return err
		}
		if !s.Held {
			return fmt.Errorf("quest is not held")
		}
		s.Held = false
		s.HeldReason = nil
		if err := state.Upsert(conn, s); err != nil {
			return err
		}

		herald.Announce(conn, herald.Tiding{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     questName,
			Type:      herald.QuestUnheld,
			Phase:     s.Phase,
			Detail:    "Quest unheld — resumed",
		})
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	fmt.Println("Quest unheld.")
	return 0
}

func runInit(d *db.DB) int {
	ctx := context.Background()
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

	// Still create .fellowship/ directory marker.
	dataDir := filepath.Join(root, datadir.Name())
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: creating data directory: %v\n", err)
		return 1
	}

	// Determine quest name: explicit flag, or derive from directory.
	qn := *questName
	if qn == "" {
		qn = filepath.Base(root)
	}

	initPhase := "Onboard"
	if *phase != "" {
		initPhase = *phase
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		// Try to load existing state to reset it.
		existing, loadErr := state.Load(conn, qn)
		if loadErr == nil {
			// Reset existing state.
			existing.GatePending = false
			existing.GateID = nil
			if *phase != "" {
				existing.Phase = *phase
				existing.LembasCompleted = false
				existing.MetadataUpdated = false
			}
			if err := state.Upsert(conn, existing); err != nil {
				return err
			}
			fmt.Printf("State reset (gate_pending cleared, phase: %s).\n", existing.Phase)
		} else {
			// Create new state.
			s := &state.State{
				QuestName:        qn,
				Phase:            initPhase,
				AutoApproveGates: []string{},
			}
			if err := state.Upsert(conn, s); err != nil {
				return err
			}
			fmt.Printf("Quest state created (quest: %s, phase: %s)\n", qn, initPhase)
		}

		if *planSkip {
			if err := tome.RecordSkippedPhases(conn, qn, []string{"Onboard", "Research", "Plan"}, "pre-existing plan"); err != nil {
				return err
			}
			fmt.Println("Recorded Onboard/Research/Plan as skipped (pre-existing plan).")
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	return 0
}

func runStatus(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dir := fs.String("dir", "", "Git repo root (default: auto-detect)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	root := *dir
	if root == "" {
		root = gitRootOrCwd()
	}

	var result *status.StatusResult
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		result, err = status.Scan(conn, root)
		return err
	}); err != nil {
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

func runEagles(d *db.DB, args []string) int {
	ctx := context.Background()
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

	var report *eagles.EaglesReport
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		report, err = eagles.Sweep(conn, opts)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	// Write report to data directory.
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

func runDashboard(d *db.DB, args []string) int {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	port := fs.Int("port", 3000, "HTTP port")
	poll := fs.Int("poll", 5, "Poll interval in seconds")
	fs.Parse(args)

	srv := dashboard.NewServer(d, *poll)

	addr := fmt.Sprintf("localhost:%d", *port)
	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Fellowship dashboard: %s\n", url)

	// Open browser.
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

func runAutopsy(d *db.DB, args []string) int {
	switch args[0] {
	case "create":
		return runAutopsyCreate(d, args[1:])
	case "scan":
		return runAutopsyScan(d, args[1:])
	case "infer":
		return runAutopsyInfer(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown autopsy command: %s\n", args[0])
		return 1
	}
}

func runAutopsyCreate(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("autopsy create", flag.ExitOnError)
	fs.Parse(args)

	var input autopsy.CreateInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: reading JSON from stdin: %v\n", err)
		return 1
	}

	var id int64
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		var err error
		id, err = autopsy.Create(conn, &input)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Autopsy created (id=%d)\n", id)
	return 0
}

func runAutopsyScan(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("autopsy scan", flag.ExitOnError)
	files := fs.String("files", "", "Comma-separated file paths to match")
	modules := fs.String("modules", "", "Comma-separated module names to match")
	tags := fs.String("tags", "", "Comma-separated tags to match")
	fs.Parse(args)

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

	var matches []autopsy.Autopsy
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		matches, err = autopsy.Scan(conn, opts, expiryDays)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(matches, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runAutopsyInfer(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("autopsy infer", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name (required)")
	fs.Parse(args)

	if *quest == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship autopsy infer --quest <name>")
		return 1
	}

	var id int64
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		var err error
		id, err = autopsy.Infer(conn, *quest)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Inferred autopsy created (id=%d)\n", id)
	return 0
}

func runHerald(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("herald", flag.ExitOnError)
	problems := fs.Bool("problems", false, "Show only detected problems")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	if *problems {
		var detected []herald.Problem
		d.WithConn(ctx, func(conn *db.Conn) error {
			detected = herald.DetectProblems(conn)
			return nil
		})
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

	var evts []herald.Tiding
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		evts, err = herald.ReadAll(conn, 20)
		return err
	}); err != nil {
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

func runCompany(d *db.DB, args []string) int {
	ctx := context.Background()
	sub := args[0]
	rest := args[1:]

	switch sub {
	case "list":
		fs := flag.NewFlagSet("company list", flag.ExitOnError)
		fs.Parse(rest)

		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			return company.List(conn)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	case "show":
		fs := flag.NewFlagSet("company show", flag.ExitOnError)
		fs.Parse(rest)

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fellowship company show <name>")
			return 1
		}
		name := fs.Arg(0)

		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			return company.Show(conn, name)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	case "approve":
		fs := flag.NewFlagSet("company approve", flag.ExitOnError)
		fs.Parse(rest)

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fellowship company approve <name>")
			return 1
		}
		name := fs.Arg(0)

		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			return company.Approve(conn, name)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown company command: %s\n", sub)
		return 1
	}
}

func runTome(d *db.DB, args []string) int {
	ctx := context.Background()
	if len(args) < 1 || args[0] != "show" {
		fmt.Fprintln(os.Stderr, "usage: fellowship tome show [--quest <name>] [--json]")
		return 1
	}

	fs := flag.NewFlagSet("tome show", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name (default: auto-detect from worktree)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args[1:])

	questName := *quest
	if questName == "" {
		cwd, _ := os.Getwd()
		gitRoot := gitRootFrom(cwd)
		d.WithConn(ctx, func(conn *db.Conn) error {
			questName, _ = state.FindQuest(conn, gitRoot)
			return nil
		})
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var c *tome.QuestTome
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		c, err = tome.Load(conn, questName)
		return err
	}); err != nil {
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
			if p.DurationS > 0 {
				dur = fmt.Sprintf(" (%ds)", p.DurationS)
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

func runErrand(d *db.DB, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand <init|list|add|update|show>")
		return 1
	}

	switch args[0] {
	case "init":
		return runErrandInit(d, args[1:])
	case "list":
		return runErrandList(d, args[1:])
	case "add":
		return runErrandAdd(d, args[1:])
	case "update":
		return runErrandUpdate(d, args[1:])
	case "show":
		return runErrandShow(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown errand command: %s\n", args[0])
		return 1
	}
}

func runErrandInit(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand init", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	task := fs.String("task", "", "Task description")
	fs.Parse(args)

	if *quest == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand init --quest <name> [--task \"desc\"]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return errand.Init(conn, *quest, *task)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Errand tracking initialized for quest %q\n", *quest)
	return 0
}

func runErrandList(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand list", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	fs.Parse(args)

	questName := *quest
	if questName == "" {
		questName = autoDetectQuest(d)
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var items []errand.Errand
	var done, total int
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		items, err = errand.List(conn, questName)
		if err != nil {
			return err
		}
		done, total, err = errand.Progress(conn, questName)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if len(items) == 0 {
		fmt.Println("No errands.")
		return 0
	}

	for _, item := range items {
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

	fmt.Printf("\nProgress: %d/%d done\n", done, total)
	return 0
}

func runErrandAdd(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand add", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	phase := fs.String("phase", "", "Quest phase")
	fs.Parse(args)

	desc := strings.Join(fs.Args(), " ")
	if desc == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand add --quest <name> \"description\"")
		return 1
	}

	questName := *quest
	if questName == "" {
		questName = autoDetectQuest(d)
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var id string
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		var err error
		id, err = errand.Add(conn, questName, desc, *phase)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added %s: %s\n", id, desc)
	return 0
}

func runErrandUpdate(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand update", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand update --quest <name> <id> <status>")
		return 1
	}

	id := remaining[0]
	statusStr := remaining[1]

	ws, ok := errand.ValidStatus(statusStr)
	if !ok {
		fmt.Fprintf(os.Stderr, "fellowship: invalid status %q (use: pending, active, done, blocked)\n", statusStr)
		return 1
	}

	questName := *quest
	if questName == "" {
		questName = autoDetectQuest(d)
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return errand.UpdateStatus(conn, questName, id, ws)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Updated %s → %s\n", id, statusStr)
	return 0
}

func runErrandShow(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand show", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	fs.Parse(args)

	questName := *quest
	if questName == "" {
		questName = autoDetectQuest(d)
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var list *errand.QuestErrandList
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		items, err := errand.List(conn, questName)
		if err != nil {
			return err
		}
		list = &errand.QuestErrandList{
			QuestName: questName,
			Items:     items,
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(list, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runState(d *db.DB, args []string) int {
	switch args[0] {
	case "init":
		return runStateInit(d, args[1:])
	case "add-quest":
		return runStateAddQuest(d, args[1:])
	case "add-scout":
		return runStateAddScout(d, args[1:])
	case "update-quest":
		return runStateUpdateQuest(d, args[1:])
	case "add-company":
		return runStateAddCompany(d, args[1:])
	case "show":
		return runStateShow(d, args[1:])
	case "clean-worktrees":
		return runStateCleanWorktrees(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown state command: %s\n", args[0])
		return 1
	}
}

func runStateInit(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state init", flag.ExitOnError)
	name := fs.String("name", "", "Fellowship name (required)")
	baseBranch := fs.String("base-branch", "", "Base branch for quest worktrees (Gandalf detects automatically; use this to override)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state init --name <name> [--base-branch BRANCH]")
		return 1
	}

	root := gitRootOrCwd()

	// Check for existing fellowship to warn about overwrite.
	d.WithConn(ctx, func(conn *db.Conn) error {
		if existing, err := dashboard.LoadFellowship(conn); err == nil {
			fmt.Fprintf(os.Stderr, "fellowship: warning: overwriting existing fellowship (name=%q, quests=%d)\n",
				existing.Name, len(existing.Quests))
		}
		return nil
	})

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return dashboard.InitFellowship(conn, *name, root, *baseBranch)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Fellowship %q initialized\n", *name)
	return 0
}

func runStateAddQuest(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state add-quest", flag.ExitOnError)
	name := fs.String("name", "", "Quest name (required)")
	task := fs.String("task", "", "Task description (required)")
	branch := fs.String("branch", "", "Branch name")
	worktree := fs.String("worktree", "", "Worktree path")
	taskID := fs.String("task-id", "", "Task ID")
	fs.Parse(args)

	if *name == "" || *task == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-quest --name <name> --task \"<desc>\" [--branch BRANCH] [--worktree PATH] [--task-id ID]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return dashboard.AddQuest(conn, dashboard.QuestEntry{
			Name:            *name,
			TaskDescription: *task,
			Worktree:        *worktree,
			Branch:          *branch,
			TaskID:          *taskID,
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added quest %q\n", *name)
	return 0
}

func runStateAddScout(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state add-scout", flag.ExitOnError)
	name := fs.String("name", "", "Scout name (required)")
	question := fs.String("question", "", "Research question (required)")
	taskID := fs.String("task-id", "", "Task ID")
	fs.Parse(args)

	if *name == "" || *question == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-scout --name <name> --question \"<question>\" [--task-id ID]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return dashboard.AddScout(conn, dashboard.ScoutEntry{
			Name:     *name,
			Question: *question,
			TaskID:   *taskID,
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added scout %q\n", *name)
	return 0
}

func runStateAddCompany(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state add-company", flag.ExitOnError)
	name := fs.String("name", "", "Company name (required)")
	quests := fs.String("quests", "", "Comma-separated quest names")
	scouts := fs.String("scouts", "", "Comma-separated scout names")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-company --name <name> [--quests q1,q2] [--scouts s1,s2]")
		return 1
	}

	questList := []string{}
	if *quests != "" {
		questList = strings.Split(*quests, ",")
	}
	scoutList := []string{}
	if *scouts != "" {
		scoutList = strings.Split(*scouts, ",")
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return dashboard.AddCompany(conn, *name, questList, scoutList)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added company %q\n", *name)
	return 0
}

func runStateUpdateQuest(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state update-quest", flag.ExitOnError)
	name := fs.String("name", "", "Quest name (required)")
	worktree := fs.String("worktree", "", "Worktree path")
	branch := fs.String("branch", "", "Branch name")
	taskID := fs.String("task-id", "", "Task ID")
	statusFlag := fs.String("status", "", "Quest status (active, completed, cancelled)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state update-quest --name <name> [--worktree PATH] [--branch BRANCH] [--task-id ID] [--status STATUS]")
		return 1
	}

	if *statusFlag != "" && *statusFlag != "active" && *statusFlag != "completed" && *statusFlag != "cancelled" {
		fmt.Fprintf(os.Stderr, "fellowship: invalid status %q (must be active, completed, or cancelled)\n", *statusFlag)
		return 1
	}

	updates := make(map[string]any)
	if *worktree != "" {
		updates["worktree"] = *worktree
	}
	if *branch != "" {
		updates["branch"] = *branch
	}
	if *taskID != "" {
		updates["task_id"] = *taskID
	}
	if *statusFlag != "" {
		updates["status"] = *statusFlag
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return dashboard.UpdateQuest(conn, *name, updates)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Updated quest %q\n", *name)
	return 0
}

func runStateShow(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state show", flag.ExitOnError)
	fs.Parse(args)

	var s *dashboard.FellowshipState
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		s, err = dashboard.LoadFellowship(conn)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runStateCleanWorktrees(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state clean-worktrees", flag.ExitOnError)
	fs.Parse(args)

	type cleanResult struct {
		name        string
		wasPending  bool
		wasHeld     bool
	}

	var cleaned []cleanResult
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		// Query all quest_state rows that have stale flags.
		type staleQuest struct {
			name        string
			gatePending bool
			held        bool
		}
		var stale []staleQuest
		if err := sqliteExecRows(conn, `SELECT quest_name, gate_pending, held FROM quest_state WHERE gate_pending = 1 OR held = 1`,
			func(name string, gp, h bool) {
				stale = append(stale, staleQuest{name, gp, h})
			}); err != nil {
			return err
		}

		for _, sq := range stale {
			s, err := state.Load(conn, sq.name)
			if err != nil {
				continue
			}
			s.GatePending = false
			s.GateID = nil
			s.Held = false
			s.HeldReason = nil
			if err := state.Upsert(conn, s); err != nil {
				fmt.Fprintf(os.Stderr, "fellowship: warning: could not clean %s: %v\n", sq.name, err)
				continue
			}
			cleaned = append(cleaned, cleanResult{sq.name, sq.gatePending, sq.held})
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if len(cleaned) == 0 {
		fmt.Println("No stale state found.")
	} else {
		for _, c := range cleaned {
			fmt.Printf("Cleared stale state in %s (gate_pending=%v, held=%v)\n", c.name, c.wasPending, c.wasHeld)
		}
		fmt.Printf("Cleaned %d quest(s).\n", len(cleaned))
	}
	return 0
}

// sqliteExecRows is a tiny helper for the clean-worktrees query.
func sqliteExecRows(conn *db.Conn, query string, fn func(name string, gatePending, held bool)) error {
	return execSqlite(conn, query, func(name string, gp, h int) {
		fn(name, gp != 0, h != 0)
	})
}

func execSqlite(conn *db.Conn, query string, fn func(name string, gp, h int)) error {
	stmt, _, err := conn.PrepareTransient(query)
	if err != nil {
		return err
	}
	defer stmt.Finalize()
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return err
		}
		if !hasRow {
			break
		}
		fn(stmt.ColumnText(0), stmt.ColumnInt(1), stmt.ColumnInt(2))
	}
	return nil
}

func runBulletin(d *db.DB, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fellowship bulletin <post|scan|list|clear>")
		return 1
	}
	switch args[0] {
	case "post":
		return runBulletinPost(d, args[1:])
	case "scan":
		return runBulletinScan(d, args[1:])
	case "list":
		return runBulletinList(d, args[1:])
	case "clear":
		return runBulletinClear(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown bulletin command: %s\n", args[0])
		return 1
	}
}

func runBulletinPost(d *db.DB, args []string) int {
	ctx := context.Background()
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

	fileList := splitCSV(*files)

	entry := bulletin.Entry{
		Quest:     *quest,
		Topic:     *topic,
		Files:     fileList,
		Discovery: *discovery,
	}
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return bulletin.Post(conn, entry)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Posted to bulletin: [%s] %s\n", *topic, *discovery)
	return 0
}

func runBulletinScan(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("bulletin scan", flag.ExitOnError)
	files := fs.String("files", "", "Comma-separated file paths to match")
	topics := fs.String("topics", "", "Comma-separated topics to match")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	fileList := splitCSV(*files)
	topicList := splitCSV(*topics)

	var entries []bulletin.Entry
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		entries, err = bulletin.Scan(conn, fileList, topicList)
		return err
	}); err != nil {
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

func runBulletinList(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("bulletin list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	var entries []bulletin.Entry
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		entries, err = bulletin.Load(conn)
		return err
	}); err != nil {
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

func runBulletinClear(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("bulletin clear", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: fellowship bulletin clear")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return bulletin.Clear(conn)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Println("Bulletin cleared.")
	return 0
}

// splitCSV splits a comma-separated string, trimming whitespace and removing empty segments.
func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
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

// gitRootFrom returns the git root for a given directory.
func gitRootFrom(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return dir
	}
	return strings.TrimSpace(string(out))
}

// autoDetectQuest tries to find the quest name for the current worktree.
func autoDetectQuest(d *db.DB) string {
	cwd, _ := os.Getwd()
	gitRoot := gitRootFrom(cwd)
	var questName string
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		questName, _ = state.FindQuest(conn, gitRoot)
		return nil
	})
	return questName
}

// jsonFilesExist checks whether legacy JSON state files exist in the .fellowship
// directory, indicating a migration is needed.
func jsonFilesExist(fromDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = fromDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	gitCommon := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitCommon) {
		gitCommon = filepath.Join(fromDir, gitCommon)
	}
	gitCommon = filepath.Clean(gitCommon)

	var mainRepo string
	if filepath.Base(gitCommon) == ".git" {
		mainRepo = filepath.Dir(gitCommon)
	} else {
		mainRepo = filepath.Dir(gitCommon)
	}
	dataDir := filepath.Join(mainRepo, ".fellowship")
	for _, name := range []string{"fellowship-state.json", "quest-state.json"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err == nil {
			return true
		}
	}
	return false
}
