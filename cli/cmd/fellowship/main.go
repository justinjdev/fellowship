package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
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

	// Old subcommand nouns are kept as aliases for one release: print one
	// deprecation line to stderr, then run the renamed command.
	if renamed, ok := commandAliases[os.Args[1]]; ok {
		fmt.Fprintf(os.Stderr, "fellowship: %q is deprecated, use %q instead\n", os.Args[1], renamed)
		os.Args[1] = renamed
	}

	// Open the store for all other commands. Only store-creating commands may
	// bring one into existence; everything else opens what is already there.
	cwd, _ := os.Getwd()
	d, err := openStore(cwd, os.Args[1:])
	if err != nil {
		os.Exit(storeOpenExit(cwd, os.Args[1:], err))
	}
	defer d.Close()

	// One line, once, when this session is not the recorded lead but is
	// standing where only the lead may write.
	noticeLeadMismatch(d, cwd, os.Args[1:])

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
		os.Exit(runInit(d, os.Args[2:]))
	case "status":
		os.Exit(runStatus(d, os.Args[2:]))
	case "group":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship group <list|show|approve>")
			os.Exit(1)
		}
		os.Exit(runGroup(d, os.Args[2:]))
	case "history":
		os.Exit(runHistory(d, os.Args[2:]))
	case "health":
		os.Exit(runHealth(d, os.Args[2:]))
	case "notes":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship notes <post|scan|list|clear>")
			os.Exit(1)
		}
		os.Exit(runNotes(d, os.Args[2:]))
	case "todo":
		os.Exit(runTodo(d, os.Args[2:]))
	case "state":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship state <init|add-quest|add-scout|add-group|update-quest|show>")
			os.Exit(1)
		}
		os.Exit(runState(d, os.Args[2:]))
	case "phase":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship phase confirm --dir <worktree> --phase <phase>")
			os.Exit(1)
		}
		os.Exit(runPhase(d, os.Args[2:]))
	case "complete":
		os.Exit(runComplete(d, os.Args[2:]))
	case "hold":
		os.Exit(runHold(d, os.Args[2:]))
	case "unhold":
		os.Exit(runUnhold(d, os.Args[2:]))
	case "failures":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fellowship failures <create|scan|infer>")
			os.Exit(1)
		}
		os.Exit(runFailures(d, os.Args[2:]))
	case "events":
		os.Exit(runEvents(d, os.Args[2:]))
	case "dashboard":
		os.Exit(runDashboard(d, os.Args[2:]))
	default:
		usage()
		os.Exit(1)
	}
}

// knownCommands is every top-level subcommand the CLI dispatches, including
// the two (version, migrate) handled before the store is opened, plus the
// deprecated old nouns in commandAliases (kept working for one release).
var knownCommands = map[string]bool{
	"version": true, "migrate": true, "hook": true, "gate": true, "init": true,
	"status": true, "group": true, "history": true, "health": true,
	"notes": true, "todo": true, "state": true, "hold": true,
	"phase": true, "complete": true,
	"unhold": true, "failures": true, "events": true, "dashboard": true,
	// Deprecated aliases — see commandAliases.
	"company": true, "tome": true, "eagles": true,
	"bulletin": true, "errand": true, "autopsy": true, "herald": true,
}

// commandAliases maps a deprecated subcommand noun to its replacement. Kept
// so a stale skill from an older plugin cache still works for one release —
// see the CLI subcommand rename table in the changelog.
var commandAliases = map[string]string{
	"herald":   "events",
	"tome":     "history",
	"errand":   "todo",
	"eagles":   "health",
	"bulletin": "notes",
	"autopsy":  "failures",
	"company":  "group",
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

// readOnlyHooks are the hooks that only decide — they read the store and write
// nothing back. They get a read-only connection, so a bug or a future edit
// cannot quietly turn a decision into a write. The recording hooks
// (gate-submit, gate-prereq, file-track) need the write side.
var readOnlyHooks = map[string]bool{
	"gate-guard":     true,
	"worktree-guard": true,
}

// pendingToolUpgradesStore reports whether the tool call this hook is deciding
// on is a fellowship command that would bring an out-of-date store up to date.
//
// It consumes stdin, which is safe only because every caller exits immediately
// afterwards: the payload is never needed again. A payload that will not parse,
// or a tool that is not Bash, reads as "not the upgrade" and the block stands.
func pendingToolUpgradesStore() bool {
	input, err := hooks.ParseInput(os.Stdin)
	if err != nil || input == nil || input.ToolName != "Bash" {
		return false
	}
	return hooks.IsStoreUpgradeCommand(input.ToolInput.Command)
}

func openStore(cwd string, args []string) (*db.DB, error) {
	if storeCreatingCommand(args) {
		return db.Open(cwd)
	}
	// Hooks never migrate: the schema ladder belongs to `init`, not to a
	// decision that fires on every tool call.
	if args[0] == "hook" {
		hookName := ""
		if len(args) > 1 {
			hookName = args[1]
		}
		return db.OpenForHook(cwd, readOnlyHooks[hookName])
	}
	return db.OpenExisting(cwd)
}

// storeOpenExit turns a store-open failure into an exit code according to the
// enforcement posture:
//
//   - No store, and no fellowship data directory either — this is an ordinary
//     repo with no fellowship, so hooks allow (exit 0) and there is nothing to
//     enforce. Other commands report the missing store.
//   - No store (or a zero-byte one) in a repo that HAS a data directory — a
//     fellowship is expected here and its state is gone. Deleting the store was
//     the cheapest way to switch enforcement off, so gate hooks block (exit 2);
//     worktree-guard still fails open.
//   - Store present but unopenable (corrupt, unreadable, locked out) — gate
//     hooks block (exit 2) because enforcement state is unknown; worktree-guard
//     still fails open.
func storeOpenExit(cwd string, args []string, err error) int {
	isHook := args[0] == "hook"
	hookName := ""
	if isHook && len(args) > 1 {
		hookName = args[1]
	}

	if errors.Is(err, db.ErrNoStore) || errors.Is(err, db.ErrEmptyStore) {
		if fellowshipExpected(cwd) {
			if isGateHook(hookName) {
				fmt.Fprintln(os.Stderr, "fellowship: store missing or empty — restore it, or run \"fellowship state init\" from a terminal outside this session (this hook blocks the session's own tool calls). Blocking for safety.")
				return 2
			}
			if isHook {
				return 0 // worktree-guard and unknown hooks fail open
			}
			fmt.Fprintln(os.Stderr, "fellowship: store missing or empty — restore it or run \"fellowship state init\".")
			return 1
		}
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

	if errors.Is(err, db.ErrSchemaOutOfDate) {
		// A hook found a store written by an older binary. Migrating is not a
		// decision hook's job, so say what to run — and let that command
		// through. gate-guard gates Bash, so a blanket block would deny the
		// only way out of itself: after any binary upgrade every tool call in
		// the repo would be refused, including the one the refusal asks for.
		// What is let through is a read-only command: every non-hook
		// invocation migrates on open, so the remedy does not have to be one
		// that can also reset a quest's gate (see hooks.IsStoreUpgradeCommand).
		if isGateHook(hookName) {
			if hookName == "gate-guard" && pendingToolUpgradesStore() {
				fmt.Fprintln(os.Stderr, `fellowship: the store is out of date — allowing this command to upgrade it.`)
				return 0
			}
			fmt.Fprintln(os.Stderr, `fellowship: the store is out of date — run "fellowship status" to upgrade it (every non-hook command migrates the store on open). Run it as the whole command, on its own: not after a cd, and with no && or pipe chained on. Blocking for safety.`)
			return 2
		}
		if isHook {
			return 0 // worktree-guard fails open
		}
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
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
  hook file-track        Record file touches in quest history
  hook worktree-guard    Block source writes to the main tree during a fellowship

Agent/lead commands:
  gate status            Show current phase, prereqs, pending/held state
    --dir DIR            Worktree directory (default: current directory)
  gate approve           Approve a pending gate (advances to next phase)
    --dir DIR            Worktree directory (default: current directory)
  gate reject            Reject a pending gate (clears pending, keeps phase)
    --dir DIR            Worktree directory (default: current directory)
  phase confirm          Confirm the quest's current phase (a gate prerequisite; never moves it)
    --dir DIR            Worktree directory (default: current directory)
    --phase PHASE        The phase the quest is in (must match)
  complete               End the quest: allowed only in Review with no gate pending
    --dir DIR            Worktree directory (default: current directory)
  hold                   Hold (pause) a quest — blocks Edit/Write/Bash/Agent/Skill/NotebookEdit
    --dir DIR            Worktree directory (required)
    --reason MSG         Reason for holding
  unhold                 Unhold (resume) a held quest
    --dir DIR            Worktree directory (required)
  history show            Show quest history (phases, gates, files touched)
    --quest NAME         Quest name (default: resolved from --dir/cwd)
    --dir DIR            Worktree directory (default: current directory)
    --json               Output as JSON
  status [--json]        Scan worktrees and show fellowship recovery status
  health                 Scan quest health (stalled/zombie/struggling classification)
    --threshold N        Gate pending timeout in minutes (default: 10)
    --json               Output as JSON

Setup commands:
  init                   Initialize quest state in DB
    --dir PATH           Worktree or repo root (default: auto-detect via git)
    --phase PHASE        Initial phase (default: Research)
    --plan-skip          Record Research/Plan as skipped in history
    --quest NAME         Quest name (default: the name registered for the
                         worktree, else its directory name)
                         Auto-approved gates are read from gates.autoApprove
                         in the merged fellowship config.

Group commands:
  group list            List all groups and their quest/scout counts
  group show <name>     Show detailed group status (phases, progress)
    --json                Output as JSON
  group approve <name>  Batch-approve all pending gates in a group

Fellowship state:
  state init              Initialize fellowship in DB
    --name NAME           Fellowship name (required)
    --base-branch BRANCH  Base branch for quest worktrees (default: auto-detected)
    --skip-hook-install   Skip registering the worktree-guard hook in settings.local.json
    --claim-lead          Record this session as the lead (alone: changes nothing else)
  state add-quest         Add a quest entry to fellowship state
    --name NAME           Quest name (required)
    --task "DESC"         Task description (required)
    --branch BRANCH       Branch name
    --worktree PATH       Worktree path
    --dir DIR             Repo or worktree directory (default: current dir)
  state add-scout         Add a scout entry to fellowship state
    --name NAME           Scout name (required)
    --question "Q"        Research question (required)
    --dir DIR             Repo or worktree directory (default: current dir)
  state add-group         Add a group entry to fellowship state
    --name NAME           Group name (required)
    --quests q1,q2        Comma-separated quest names
    --scouts s1,s2        Comma-separated scout names
    --dir DIR             Repo or worktree directory (default: current dir)
  state update-quest      Update an existing quest entry
    --name NAME           Quest name (required)
    --worktree PATH       Worktree path
    --branch BRANCH       Branch name
    --status STATUS       Quest status (active, completed, cancelled)
    --dir DIR             Repo or worktree directory (default: current dir)
  state show              Show fellowship state as JSON
    --dir DIR             Repo or worktree directory (default: current dir)
    --json                Accepted for consistency (output is always JSON)
  state clean-worktrees   Reset stale gate_pending/held flags in all quests

Todos (persistent work items). Every todo command resolves the quest from
--quest, else from --dir, else from the current directory. Valid statuses:
pending, in_progress, done, blocked, skipped.
  todo init              Initialize todos for a quest
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
  todo list              Show all todos with status
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
  todo add               Add a new todo
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
    --phase PHASE        Quest phase (optional)
    "description"        Todo description (positional arg)
  todo update            Update a todo's status
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)
    <id> <status>        Item ID and new status (positional args)
  todo show              Show all todos as JSON
    --quest NAME         Quest name
    --dir DIR            Worktree directory (default: current directory)

Notes (cross-quest knowledge sharing):
  notes post             Post a discovery to the shared notes board
    --quest NAME         Quest name (required)
    --topic TOPIC        Topic tag (required)
    --files FILE,FILE    Comma-separated relevant file paths
    --discovery "TEXT"   Discovery description (required)
  notes scan             Scan notes for relevant entries
    --files FILE,FILE    Comma-separated file paths to match
    --topics T1,T2       Comma-separated topics to match
    --json               Output as JSON
  notes list             Show all notes entries
    --json               Output as JSON
  notes clear            Clear the notes board

Events (activity log):
  events                 Show recent quest events
    --problems           Show only detected problems
    --quest NAME         Show events for one quest only
    --limit N            Maximum events to show (default: 20, 0 for all)
    --json               Output as JSON
  events post            Record an event (used by the palantir monitor)
    --quest NAME         Quest the event is about (required)
    --type TYPE          Event type (required) — e.g. palantir_stuck,
                         palantir_drift, palantir_conflict, palantir_health,
                         palantir_notes
    --phase PHASE        Quest phase (optional)
    --detail "TEXT"      Detail text (required)

Dashboard:
  dashboard              Start live web dashboard
    --port N             HTTP port (default: 3000)
    --poll N             Poll interval in seconds (default: 5)

Failures (failure memory):
  failures create        Write a structured failure record (reads JSON from stdin)
    --dir DIR            Repo or worktree directory (default: current dir)
  failures scan          Find failure records matching files, modules, or tags
    --files f1,f2        Comma-separated file paths to match
    --modules m1,m2      Comma-separated module names to match
    --tags t1,t2         Comma-separated tags to match
    --all                Return every unexpired failure record (ignores filters)
    --dir DIR            Repo or worktree directory (default: current dir)
  failures infer         Reconstruct a failure record from quest signals
    --quest NAME         Quest name (default: resolved from --dir/cwd)
    --dir DIR            Worktree directory (default: current directory)

Deprecated aliases (kept working for one release, print a warning to stderr):
  herald -> events, tome -> history, errand -> todo, eagles -> health,
  bulletin -> notes, autopsy -> failures, company -> group,
  state add-company -> state add-group

Other:
  migrate                Migrate JSON files to SQLite
  version                Print version`)
}
