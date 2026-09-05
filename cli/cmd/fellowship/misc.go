// Reporting commands (status, eagles, dashboard, herald, company, tome), the
// JSON migration entry point, and the small path helpers they share.
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

	"github.com/justinjdev/fellowship/cli/internal/company"
	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/eagles"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/status"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

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
	threshold := fs.Int("threshold", 10, "Gate pending timeout in minutes")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

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

	// Open browser — a nicety; the URL is printed above either way.
	var opener *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		opener = exec.Command("open", url)
	case "linux":
		opener = exec.Command("xdg-open", url)
	}
	if opener != nil {
		if err := opener.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: could not open a browser (%v) — visit %s\n", err, url)
		}
	}

	if err := http.ListenAndServe(addr, srv); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	return 0
}

func runHerald(d *db.DB, args []string) int {
	ctx := context.Background()
	if len(args) > 0 && args[0] == "post" {
		return runHeraldPost(d, args[1:])
	}

	fs := flag.NewFlagSet("herald", flag.ExitOnError)
	problems := fs.Bool("problems", false, "Show only detected problems")
	quest := fs.String("quest", "", "Show tidings for one quest only")
	limit := fs.Int("limit", 20, "Maximum tidings to show (0 for all)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	if *problems {
		var detected []herald.Problem
		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			var err error
			detected, err = herald.DetectProblems(conn)
			return err
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
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
		if *quest != "" {
			evts, err = herald.Read(conn, *quest, *limit)
		} else {
			evts, err = herald.ReadAll(conn, *limit)
		}
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

// runHeraldPost records a tiding. It exists so agents (notably the palantir
// monitor) can append structured activity to the herald without hand-rolling
// JSON lines.
func runHeraldPost(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("herald post", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest the tiding is about (required)")
	tidingType := fs.String("type", "", "Tiding type (required)")
	phase := fs.String("phase", "", "Quest phase")
	detail := fs.String("detail", "", "Detail text (required)")
	fs.Parse(args)

	if *quest == "" || *tidingType == "" || *detail == "" {
		fmt.Fprintln(os.Stderr, `usage: fellowship herald post --quest <name> --type <type> --detail "TEXT" [--phase PHASE]`)
		return 1
	}

	tt, ok := herald.ValidType(*tidingType)
	if !ok {
		fmt.Fprintf(os.Stderr, "fellowship: invalid tiding type %q (valid: %s)\n", *tidingType, strings.Join(herald.Types(), ", "))
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return herald.Announce(conn, herald.Tiding{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     *quest,
			Type:      tt,
			Phase:     *phase,
			Detail:    *detail,
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Recorded %s tiding for %s.\n", tt, *quest)
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
		fmt.Fprintln(os.Stderr, "usage: fellowship tome show [--quest <name>] [--dir <worktree>] [--json]")
		return 1
	}

	fs := flag.NewFlagSet("tome show", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name (default: auto-detect from worktree)")
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args[1:])

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	questName := *quest
	if questName == "" {
		questName = resolveDirQuest(d, *dir)
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

// gitRootOrCwd returns the working tree root, falling back to the process
// working directory outside a repo.
func gitRootOrCwd() string {
	cwd, _ := os.Getwd()
	return gitRootFrom(cwd)
}

// listQuestWorktrees returns the canonicalized roots of all git worktrees for
// the repo containing dir, excluding the main worktree. Used by the lead's
// cd-guard to recognize quest worktrees created OUTSIDE the main tree (not just
// the legacy .claude/worktrees location). Returns nil if git is unavailable.
func listQuestWorktrees(dir string) []string {
	worktrees, err := gitutil.ListWorktrees(dir)
	if err != nil {
		return nil
	}
	mainRoot := ""
	if mr, err := gitutil.MainRepoRoot(dir); err == nil {
		mainRoot = hooks.CanonicalPath(mr)
	}
	var paths []string
	for _, wt := range worktrees {
		p := hooks.CanonicalPath(strings.TrimSpace(wt))
		if p == "" || p == mainRoot {
			continue
		}
		paths = append(paths, p)
	}
	return paths
}

// gitRootFrom returns the git root for a given directory, or dir itself when
// git cannot answer (dir is not in a repo, or git is unavailable).
func gitRootFrom(dir string) string {
	root, err := gitutil.TopLevel(dir)
	if err != nil {
		return dir
	}
	return root
}

// resolveDirQuest returns the quest name registered for dir, or for the
// process working directory when dir is empty. Resolution mirrors the hooks:
// canonicalize to the git root and look that up in fellowship state. A raw
// path match is the fallback, so --dir also accepts a worktree path recorded
// exactly as it was passed to `state add-quest`.
func resolveDirQuest(d *db.DB, dir string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	var questName string
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		n, err := state.FindQuest(conn, gitRootFrom(dir))
		if err != nil {
			return err
		}
		if n != "" {
			questName = n
			return nil
		}
		questName, err = state.FindQuest(conn, dir)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: could not resolve the quest for %q: %v\n", dir, err)
		return ""
	}
	return questName
}

// checkDir validates a --dir value. The state database is resolved from the
// process working directory and every worktree of a repo shares one database,
// so a --dir pointing into a different repository would silently operate on
// the wrong state. Reject that rather than write to the wrong fellowship.
// When git cannot answer, the check passes — it is a guard, not a gate.
func checkDir(dir string) error {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("--dir %q is not a directory", dir)
	}
	cwd, _ := os.Getwd()
	want, wantErr := gitutil.MainRepoRoot(cwd)
	got, gotErr := gitutil.MainRepoRoot(dir)
	if wantErr != nil || gotErr != nil {
		return nil
	}
	if hooks.CanonicalPath(want) != hooks.CanonicalPath(got) {
		return fmt.Errorf("--dir %q belongs to a different repository than the current directory", dir)
	}
	return nil
}

// jsonFilesExist checks whether legacy JSON state files exist in the .fellowship
// directory, indicating a migration is needed.
func jsonFilesExist(fromDir string) bool {
	mainRepo, err := gitutil.MainRepoRoot(fromDir)
	if err != nil {
		return false
	}
	// The legacy files only ever lived in ".fellowship", before dataDir was
	// configurable, so this one is deliberately not datadir.Resolve.
	dataDir := filepath.Join(mainRepo, ".fellowship")
	for _, name := range []string{"fellowship-state.json", "quest-state.json"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err == nil {
			return true
		}
	}
	return false
}

// runMigrate resolves the main repo, opens a DB, and migrates JSON files.
func runMigrate() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	mainRepo, err := gitutil.MainRepoRoot(cwd)
	if err != nil {
		return err
	}
	d, err := db.Open(cwd)
	if err != nil {
		return err
	}
	defer d.Close()
	return db.MigrateJSON(d, mainRepo)
}
