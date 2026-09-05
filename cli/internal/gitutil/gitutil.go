package gitutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunGit executes a git command in the given directory and returns stdout.
// It is RunGitContext with a background context: callers that work to a
// deadline — the hooks, whose whole invocation is bounded — should pass theirs.
func RunGit(dir string, args ...string) (string, error) {
	return RunGitContext(context.Background(), dir, args...)
}

// RunGitContext executes a git command under ctx, so a git call that hangs on a
// lock or a slow filesystem is cancelled with the caller rather than outliving
// it. A hook has about 5s before Claude Code kills it outright; the deadline it
// sets is only real if the subprocesses respect it.
func RunGitContext(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListWorktrees parses `git worktree list --porcelain` and returns worktree paths.
func ListWorktrees(gitRoot string) ([]string, error) {
	return ListWorktreesContext(context.Background(), gitRoot)
}

// ListWorktreesContext is ListWorktrees bounded by ctx.
func ListWorktreesContext(ctx context.Context, gitRoot string) ([]string, error) {
	out, err := RunGitContext(ctx, gitRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths, nil
}

// BranchForWorktree returns the current branch name for a worktree directory.
func BranchForWorktree(wtPath string) string {
	out, err := RunGit(wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// CheckUncommitted returns true if `git status --porcelain` produces any output.
func CheckUncommitted(wtPath string) bool {
	out, err := RunGit(wtPath, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// FileExists returns true if the path exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// GateAge parses a gate ID (format: "gate-<phase>-<unix-timestamp>") and returns
// the age in seconds relative to the given time. Returns 0 if the ID is unparseable.
func GateAge(gateID string, now time.Time) int {
	parts := strings.Split(gateID, "-")
	if len(parts) < 2 {
		return 0
	}
	ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0
	}
	gateTime := time.Unix(ts, 0)
	age := now.Sub(gateTime)
	if age < 0 {
		return 0
	}
	return int(age.Seconds())
}

// TopLevel returns the working-tree root for dir — `git rev-parse
// --show-toplevel`. In a linked worktree that is the worktree's own root, not
// the main repo's; use MainRepoRoot for that.
func TopLevel(dir string) (string, error) {
	return TopLevelContext(context.Background(), dir)
}

// TopLevelContext is TopLevel bounded by ctx.
func TopLevelContext(ctx context.Context, dir string) (string, error) {
	out, err := RunGitContext(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// MainRepoRoot returns the main repository root for dir, resolving through
// `git rev-parse --git-common-dir` so that any linked worktree maps back to the
// main worktree. The fellowship store, its data directory and the guards all
// key off this path, so every caller must compute it the same way.
func MainRepoRoot(dir string) (string, error) {
	return MainRepoRootContext(context.Background(), dir)
}

// MainRepoRootContext is MainRepoRoot bounded by ctx.
func MainRepoRootContext(ctx context.Context, dir string) (string, error) {
	out, err := RunGitContext(ctx, dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	gitCommon := strings.TrimSpace(out)
	// --git-common-dir may answer with a path relative to dir.
	if !filepath.IsAbs(gitCommon) {
		gitCommon = filepath.Join(dir, gitCommon)
	}
	// The main repo root is the parent of the shared .git directory.
	return filepath.Dir(filepath.Clean(gitCommon)), nil
}
