package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/justinjdev/fellowship/cli/internal/db"
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
		if err := runMigrate(); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: migrate: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Reject unknown commands before going anywhere near the store. Opening the
	// store used to be the first thing every invocation did, so a typo like
	// `fellowship bogus` created .fellowship/fellowship.db in whatever repo the
	// user happened to be standing in.
	if !isKnownCommand(os.Args[1]) {
		usage()
		os.Exit(1)
	}

	// Open the store for all other commands. Only store-creating commands may
	// bring one into existence; everything else opens what is already there.
	cwd, _ := os.Getwd()
	d, err := openStore(cwd, os.Args[1:])
	if err != nil {
		os.Exit(storeOpenExit(cwd, os.Args[1:], err))
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

// knownCommands is every top-level subcommand the CLI dispatches, including the
// two (version, migrate) handled before the store is opened.
var knownCommands = map[string]bool{
	"version": true, "migrate": true, "hook": true, "gate": true, "init": true,
	"status": true, "company": true, "tome": true, "eagles": true,
	"bulletin": true, "errand": true, "state": true, "hold": true,
	"unhold": true, "autopsy": true, "herald": true, "dashboard": true,
}

func isKnownCommand(name string) bool { return knownCommands[name] }

// storeCreatingCommand reports whether args name a command allowed to create
// the fellowship store. Only explicit initialization qualifies; a hook that
// created the store it is about to read would turn every unrelated repo into a
// fellowship and would report "no quest here" for a store it just made.
func storeCreatingCommand(args []string) bool {
	switch args[0] {
	case "init":
		return true
	case "state":
		return len(args) > 1 && args[1] == "init"
	}
	return false
}

func openStore(cwd string, args []string) (*db.DB, error) {
	if storeCreatingCommand(args) {
		return db.Open(cwd)
	}
	return db.OpenExisting(cwd)
}

// storeOpenExit turns a store-open failure into an exit code according to the
// enforcement posture:
//
//   - No store at all — this is an ordinary repo with no fellowship, so hooks
//     allow (exit 0) and there is nothing to enforce. Other commands report the
//     missing store.
//   - Store present but unopenable (corrupt, unreadable, locked out) — gate
//     hooks block (exit 2) because enforcement state is unknown; worktree-guard
//     still fails open.
func storeOpenExit(cwd string, args []string, err error) int {
	isHook := args[0] == "hook"
	hookName := ""
	if isHook && len(args) > 1 {
		hookName = args[1]
	}

	if errors.Is(err, db.ErrNoStore) {
		if isHook {
			return 0 // no fellowship here — nothing to enforce
		}
		if jsonFilesExist(cwd) {
			fmt.Fprintln(os.Stderr, `fellowship: Run "fellowship migrate" to upgrade to the new storage format.`)
			return 1
		}
		fmt.Fprintln(os.Stderr, `fellowship: no fellowship state in this repo — run "fellowship init" first.`)
		return 1
	}

	if isGateHook(hookName) {
		fmt.Fprintf(os.Stderr, "fellowship: cannot read fellowship state (%v) — blocking for safety.\n", err)
		return 2
	}
	if isHook {
		return 0 // worktree-guard and unknown hooks fail open
	}
	if jsonFilesExist(cwd) {
		fmt.Fprintln(os.Stderr, `fellowship: Run "fellowship migrate" to upgrade to the new storage format.`)
		return 1
	}
	fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
	return 1
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: fellowship <command>

Hook commands (called by Claude Code hooks, read stdin):
  hook gate-guard        Block tools when gate pending or early-phase file writes
  hook gate-submit       Detect [GATE] messages, check prereqs, advance state
  hook gate-prereq       Track lembas skill invocation
  hook metadata-track    Track phase metadata updates
  hook completion-guard  Block task completion unless phase is Review with no pending gate
  hook file-track        Record file touches in quest tome
  hook worktree-guard    Block source writes to the main tree during a fellowship

Agent/lead commands:
  gate status            Show current phase, prereqs, pending/held state
    --dir DIR            Worktree directory (default: current directory)
  gate approve           Approve a pending gate (advances to next phase)
    --dir DIR            Worktree directory (default: current directory)
  gate reject            Reject a pending gate (clears pending, keeps phase)
    --dir DIR            Worktree directory (default: current directory)
  hold                   Hold (pause) a quest — blocks Edit/Write/Bash/Agent/Skill/NotebookEdit
    --dir DIR            Worktree directory (required)
    --reason MSG         Reason for holding
  unhold                 Unhold (resume) a held quest
    --dir DIR            Worktree directory (required)
  tome show              Show quest tome (phases, gates, files touched)
    --quest NAME         Quest name (default: resolved from --dir/cwd)
    --dir DIR            Worktree directory (default: current directory)
    --json               Output as JSON
  status [--json]        Scan worktrees and show fellowship recovery status
  eagles                 Scan quest health (stalled/zombie/struggling classification)
    --threshold N        Gate pending timeout in minutes (default: 10)
    --json               Output as JSON

Setup commands:
  init                   Initialize quest state in DB
    --dir PATH           Worktree or repo root (default: auto-detect via git)
    --phase PHASE        Initial phase (default: Research)
    --plan-skip          Record Research/Plan as skipped in tome
    --quest NAME         Quest name (default: the name registered for the
                         worktree, else its directory name)
                         Auto-approved gates are read from gates.autoApprove
                         in the merged fellowship config.

Company commands:
  company list            List all companies and their quest/scout counts
  company show <name>     Show detailed company status (phases, progress)
  company approve <name>  Batch-approve all pending gates in a company

Fellowship state:
  state init              Initialize fellowship in DB
    --name NAME           Fellowship name (required)
    --base-branch BRANCH  Base branch for quest worktrees (default: auto-detected)
    --skip-hook-install   Skip registering the worktree-guard hook in settings.local.json
  state add-quest         Add a quest entry to fellowship state
    --name NAME           Quest name (required)
    --task "DESC"         Task description (required)
    --branch BRANCH       Branch name
    --worktree PATH       Worktree path
    --task-id ID          Task ID
    --dir DIR             Repo or worktree directory (default: current dir)
  state add-scout         Add a scout entry to fellowship state
    --name NAME           Scout name (required)
    --question "Q"        Research question (required)
    --task-id ID          Task ID
    --dir DIR             Repo or worktree directory (default: current dir)
  state add-company       Add a company entry to fellowship state
    --name NAME           Company name (required)
    --quests q1,q2        Comma-separated quest names
    --scouts s1,s2        Comma-separated scout names
    --dir DIR             Repo or worktree directory (default: current dir)
  state update-quest      Update an existing quest entry
    --name NAME           Quest name (required)
    --worktree PATH       Worktree path
    --branch BRANCH       Branch name
    --task-id ID          Task ID
    --status STATUS       Quest status (active, completed, cancelled)
    --dir DIR             Repo or worktree directory (default: current dir)
  state show              Show fellowship state as JSON
    --dir DIR             Repo or worktree directory (default: current dir)
  state clean-worktrees   Reset stale gate_pending/held flags in all quests

Errands (persistent work items). Every errand command resolves the quest from
--quest, else from --dir, else from the current directory. Valid statuses:
pending, in_progress, done, blocked, skipped.
  errand init            Initialize errands for a quest
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
    --task "DESC"        Task description
  errand list            Show all errands with status
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
  errand add             Add a new errand
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
    --phase PHASE        Quest phase (optional)
    "description"        Errand description (positional arg)
  errand update          Update an errand's status
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
    <id> <status>        Item ID and new status (positional args)
  errand show            Show all errands as JSON
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)

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
    --quest NAME         Show tidings for one quest only
    --limit N            Maximum tidings to show (default: 20, 0 for all)
    --json               Output as JSON
  herald post            Record a tiding (used by the palantir monitor)
    --quest NAME         Quest the tiding is about (required)
    --type TYPE          Tiding type (required) — e.g. palantir_stuck,
                         palantir_drift, palantir_conflict, palantir_health,
                         palantir_bulletin
    --phase PHASE        Quest phase (optional)
    --detail "TEXT"      Detail text (required)

Dashboard:
  dashboard              Start live web dashboard
    --port N             HTTP port (default: 3000)
    --poll N             Poll interval in seconds (default: 5)

Autopsy (failure memory):
  autopsy create         Write a structured failure record (reads JSON from stdin)
    --dir DIR            Repo or worktree directory (default: current dir)
  autopsy scan           Find autopsies matching files, modules, or tags
    --files f1,f2        Comma-separated file paths to match
    --modules m1,m2      Comma-separated module names to match
    --tags t1,t2         Comma-separated tags to match
    --all                Return every unexpired autopsy (ignores filters)
    --dir DIR            Repo or worktree directory (default: current dir)
  autopsy infer          Reconstruct autopsy from quest signals
    --quest NAME         Quest name (default: resolved from --dir/cwd)
    --dir DIR            Worktree directory (default: current directory)

Other:
  migrate                Migrate JSON files to SQLite
  version                Print version`)
}
