// Reporting commands (status, dashboard), the JSON migration entry point,
// and the small path helpers the other cmd/fellowship files share.
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

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/status"
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

// extractJSONFlag pulls a "--json" (or "-json") token out of args wherever it
// appears and reports whether it was present, returning the remaining
// arguments in order. It exists for subcommands that mix a positional
// argument with --json, since Go's flag package stops recognizing flags at
// the first positional one — so "<name> --json" would otherwise silently
// leave --json unparsed.
func extractJSONFlag(args []string) (found bool, rest []string) {
	for _, a := range args {
		if a == "--json" || a == "-json" {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return found, rest
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
