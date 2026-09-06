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

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
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
	claimLead := fs.Bool("claim-lead", false, "Record this session as the fellowship's lead (without --name, changes nothing else)")
	fs.Parse(args)

	if *name == "" && !*claimLead {
		fmt.Fprintln(os.Stderr, "usage: fellowship state init --name <name> [--base-branch BRANCH] [--skip-hook-install]\n       fellowship state init --claim-lead")
		return 1
	}

	// Everything `state init` records — the fellowship row's main_repo, the
	// lead, the settings file the teammates inherit — belongs to the MAIN
	// worktree, which is where the store lives. Resolving the session's own
	// top-level instead put them in whatever worktree the command was run
	// from, where no hook would ever look for them.
	root := mainRepoRootOrCwd()

	// --claim-lead re-records the lead. On its own it touches nothing else; it
	// is the way out of the degraded case: the lead's session id changed (a new
	// session in the main tree rather than a resumed one), so worktree-guard
	// now reads the lead as a mis-placed teammate and refuses its writes.
	//
	// The lead moved into the store precisely so the sessions it identifies
	// could not rewrite it. --claim-lead is a CLI door back into that row, so
	// it is only opened from the main working tree — the lead's own workspace.
	// A teammate stands in its worktree, where this refuses; reaching the main
	// tree at all is a violation worktree-guard already reports.
	if *claimLead && !sessionInMainWorktree(root) {
		fmt.Fprintf(os.Stderr, "fellowship: --claim-lead only runs in the main working tree (%s); the lead's session is the one that may record itself.\n", root)
		return 1
	}
	if *name == "" {
		return claimLeadSession(d, root)
	}

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

	leadAlreadyRecorded := false

	// Record which Claude Code session this is. The main working tree is the
	// lead's own workspace, so worktree-guard needs to know which session may
	// write there; nothing in the git topology distinguishes the lead from a
	// teammate that was mis-placed into the main tree. It goes in the store,
	// not in a file under the data directory: the guards exempt that directory,
	// so a marker there was forgeable by the sessions it identified.
	// Best-effort — without a recorded lead the guard falls back to its
	// narrower rule.
	//
	// A lead that is ALREADY recorded is not overwritten. `state init` was a
	// second door into the lead row: re-running it — which the skill does on
	// every `/fellowship`, and which any session can do — silently re-recorded
	// whoever ran it. Re-recording is `--claim-lead`'s job, and it says so.
	// Asking for it explicitly alongside --name does re-record, under exactly
	// the guards the standalone form runs under.
	if *claimLead {
		if code := claimLeadSession(d, root); code != 0 {
			return code
		}
	} else {
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			// A lead the store cannot be read for is not "no lead": treat the
			// failure as a failure rather than overwriting the row behind it.
			_, found, err := state.ReadLead(conn)
			if err != nil {
				return err
			}
			if found {
				leadAlreadyRecorded = true
				return nil
			}
			return state.RecordLead(conn, root, state.CurrentSessionID())
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: warning: could not record the lead session: %v\n", err)
		}
		if leadAlreadyRecorded {
			fmt.Fprintln(os.Stderr, "fellowship: a lead session is already recorded for this repo and was left alone. If this session is the lead, run \"fellowship state init --claim-lead\" from the main working tree.")
		} else {
			warnNoSessionID()
		}
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

// sessionInMainWorktree reports whether this process is standing in the main
// working tree of the repo rooted at mainRoot. Anything it cannot determine
// reads as true: the check exists to keep a teammate from naming itself the
// lead, not to refuse a lead whose git lookup failed.
func sessionInMainWorktree(mainRoot string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return true
	}
	top, err := gitutil.TopLevel(cwd)
	if err != nil {
		return true
	}
	return hooks.CanonicalPath(top) == hooks.CanonicalPath(mainRoot)
}

// claimLeadSession re-records the running session as the fellowship's lead,
// changing nothing else. `fellowship state init --claim-lead`.
func claimLeadSession(d *db.DB, root string) int {
	session := state.CurrentSessionID()

	// A session that is already working a quest is a teammate, and a teammate
	// may not name itself the lead — that is the forgery the lead row moved
	// into SQLite to stop, and --claim-lead is the one CLI door back into it.
	// The id comes from quest_state, recorded when the quest ran `fellowship
	// init`; a fellowship whose teammates had no session id in their
	// environment records none, and gate-guard's refusal of lead commands from
	// a quest worktree carries the case instead.
	isTeammate := false
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		isTeammate, err = state.SessionIsTeammate(conn, session)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: could not check this session against the registered quests (%v) — refusing to record it as the lead.\n", err)
		return 1
	}
	if isTeammate {
		fmt.Fprintf(os.Stderr, "fellowship: this session (%s) is recorded as a quest teammate, so it cannot also be the fellowship's lead. Run --claim-lead from the lead's own session.\n", session)
		return 1
	}

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.RecordLead(conn, root, session)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if session == "" {
		warnNoSessionID()
		fmt.Println("Recorded this session as the lead, with no session id to identify it by.")
		return 0
	}
	fmt.Printf("Recorded session %s as the fellowship's lead.\n", session)
	return 0
}

// warnNoSessionID says so when the lead was recorded without an id to identify
// it by. Claude Code exports CLAUDE_CODE_SESSION_ID to the commands it runs;
// without it (a plain shell, an older Claude Code) the lead is anonymous, and
// worktree-guard falls back to its narrower rule — the one that cannot tell the
// lead from a teammate dropped into the main tree.
func warnNoSessionID() {
	if state.CurrentSessionID() != "" {
		return
	}
	fmt.Fprintln(os.Stderr, "fellowship: warning: no CLAUDE_CODE_SESSION_ID in the environment — the lead was recorded without a session id, so worktree-guard cannot tell this session apart from a mis-placed teammate. Re-run \"fellowship state init --claim-lead\" from a Claude Code session to record one.")
}

// noticeLeadMismatch prints one line when a command runs in the MAIN working
// tree from a session that is not the fellowship's recorded lead.
//
// That is the degraded case worth naming out loud: the guard will refuse this
// session's source writes in the main tree, and the fix — re-recording the lead
// — is not something anyone would guess. Hooks are exempt (they decide, they do
// not advise) and so are the commands that would fix it.
//
// It is said only while a fellowship is actually RUNNING, the same predicate
// worktree-guard arms itself with: the lead row outlives the fellowship, so
// without that check every command in the main tree announced a refusal the
// guard would never make, forever after the last quest merged. The store read
// comes first for the same reason — it answers "is there anything to say"
// before the second git call is spent.
func noticeLeadMismatch(d *db.DB, cwd string, args []string) {
	if len(args) == 0 || args[0] == "hook" || storeCreatingCommand(args) {
		return
	}
	session := state.CurrentSessionID()
	if session == "" {
		return
	}
	ctx := context.Background()
	mainRoot, err := gitutil.MainRepoRoot(cwd)
	if err != nil {
		return
	}
	lead := ""
	running := false
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		lead = state.LeadSessionID(conn, mainRoot, datadir.Resolve(mainRoot))
		fs, err := fellowship.LoadFellowship(conn)
		if err != nil {
			return nil
		}
		running = fellowshipRunning(fs)
		return nil
	}); err != nil {
		return
	}
	if !running || lead == "" || lead == session {
		return
	}
	if hooks.CanonicalPath(gitRootFrom(ctx, cwd)) != hooks.CanonicalPath(mainRoot) {
		return // not in the main tree; this is a teammate's worktree
	}
	fmt.Fprintln(os.Stderr, "fellowship: this session is not the fellowship's recorded lead, so worktree-guard will refuse its source writes in the main tree. If this session IS the lead, re-record it with \"fellowship state init --claim-lead\".")
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
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" || *task == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-quest --name <name> --task \"<desc>\" [--branch BRANCH] [--worktree PATH] [--dir DIR]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return fellowship.AddQuest(conn, fellowship.QuestEntry{
			Name:            *name,
			TaskDescription: *task,
			Worktree:        *worktree,
			Branch:          *branch,
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
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" || *question == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state add-scout --name <name> --question \"<question>\" [--dir DIR]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return fellowship.AddScout(conn, fellowship.ScoutEntry{
			Name:     *name,
			Question: *question,
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
	statusFlag := fs.String("status", "", "Quest status (active, completed, cancelled)")
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *name == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship state update-quest --name <name> [--worktree PATH] [--branch BRANCH] [--status STATUS] [--dir DIR]")
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
