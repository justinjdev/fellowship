// `fellowship state ...`: the lead-side commands that create a fellowship and
// register the quests, scouts and groups in it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/install"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

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
		// Deprecated alias for add-group, kept working for one release.
		fmt.Fprintln(os.Stderr, `fellowship: "state add-company" is deprecated, use "state add-group" instead`)
		return runStateAddGroup(d, args[1:])
	case "add-group":
		return runStateAddGroup(d, args[1:])
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
	skipHookInstall := fs.Bool("skip-hook-install", false, "Do not register the worktree-guard hook in .claude/settings.local.json")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state init --name <name> [--base-branch BRANCH] [--skip-hook-install]")
		return 1
	}

	// Everything `state init` records — the fellowship row's main_repo, the
	// lead, the settings file the teammates inherit — belongs to the MAIN
	// worktree, which is where the store lives. Resolving the session's own
	// top-level instead put them in whatever worktree the command was run
	// from, where no hook would ever look for them.
	root := mainRepoRootOrCwd()

	// Check for existing fellowship to warn about overwrite.
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		if existing, err := fellowship.LoadFellowship(conn); err == nil {
			fmt.Fprintf(os.Stderr, "fellowship: warning: overwriting existing fellowship (name=%q, quests=%d)\n",
				existing.Name, len(existing.Quests))
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: warning: could not check for an existing fellowship: %v\n", err)
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return fellowship.InitFellowship(conn, *name, root, *baseBranch)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Fellowship %q initialized\n", *name)

	// Record which Claude Code session this is. The main working tree is the
	// lead's own workspace, so worktree-guard needs to know which session may
	// write there; nothing in the git topology distinguishes the lead from a
	// teammate that was mis-placed into the main tree. It goes in the store,
	// not in a file under the data directory: the guards exempt that directory,
	// so a marker there was forgeable by the sessions it identified.
	// Best-effort — without a recorded lead the guard falls back to its
	// narrower rule.
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return state.RecordLead(conn, root, state.CurrentSessionID())
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: warning: could not record the lead session: %v\n", err)
	}

	// Register the worktree-guard hook in the project's .claude/settings.local.json.
	// Teammate sessions do NOT inherit plugin hooks, so this settings file is
	// what makes a session enforce isolation. settings.local.json is git-ignored,
	// so this touches no git history and leaves no untracked file — the lead
	// copies it into each worktree at spawn (see the fellowship skill).
	// Best-effort — a hook-install hiccup must not fail init.
	if !*skipHookInstall {
		installWorktreeGuardHook(root)
	}
	return 0
}

// installWorktreeGuardHook merges the worktree-guard hook into the project's
// git-ignored .claude/settings.local.json (idempotent, preserving existing
// settings). The lead copies that file into each worktree at spawn. Any failure
// is a warning, never fatal — the hook is defense-in-depth behind
// lead-provisioned isolation and the teammate self-check.
func installWorktreeGuardHook(root string) {
	changed, err := install.EnsureWorktreeGuardHook(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: warning: could not register worktree-guard hook: %v\n", err)
		return
	}
	if changed {
		fmt.Println("Registered worktree-guard hook in .claude/settings.local.json.")
	}
	fmt.Println("Copy .claude/settings.local.json into each quest worktree at spawn so teammates inherit the guard.")
}

func runStateAddQuest(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state add-quest", flag.ExitOnError)
	name := fs.String("name", "", "Quest name (required)")
	task := fs.String("task", "", "Task description (required)")
	branch := fs.String("branch", "", "Branch name")
	worktree := fs.String("worktree", "", "Worktree path")
	taskID := fs.String("task-id", "", "Task ID")
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" || *task == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-quest --name <name> --task \"<desc>\" [--branch BRANCH] [--worktree PATH] [--task-id ID] [--dir DIR]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return fellowship.AddQuest(conn, fellowship.QuestEntry{
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
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" || *question == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-scout --name <name> --question \"<question>\" [--task-id ID] [--dir DIR]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return fellowship.AddScout(conn, fellowship.ScoutEntry{
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

func runStateAddGroup(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("state add-group", flag.ExitOnError)
	name := fs.String("name", "", "Group name (required)")
	quests := fs.String("quests", "", "Comma-separated quest names")
	scouts := fs.String("scouts", "", "Comma-separated scout names")
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-group --name <name> [--quests q1,q2] [--scouts s1,s2] [--dir DIR]")
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
		return fellowship.AddGroup(conn, *name, questList, scoutList)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added group %q\n", *name)
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
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state update-quest --name <name> [--worktree PATH] [--branch BRANCH] [--task-id ID] [--status STATUS] [--dir DIR]")
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
		return fellowship.UpdateQuest(conn, *name, updates)
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
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	// Output is always JSON; --json is accepted (and a no-op) so callers don't
	// need to special-case this command among the others that gate JSON
	// behind the flag.
	fs.Bool("json", false, "Output as JSON (default; accepted for consistency)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	var s *fellowship.FellowshipState
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		s, err = fellowship.LoadFellowship(conn)
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
		name       string
		wasPending bool
		wasHeld    bool
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
			state.Reset(s)
			// Reset owns the gate flags; a hold is separate state and this
			// command's whole job is to release both.
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
