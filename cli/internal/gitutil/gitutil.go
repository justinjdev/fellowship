package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunGit executes a git command in the given directory and returns stdout.
func RunGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListWorktrees parses `git worktree list --porcelain` and returns worktree paths.
func ListWorktrees(gitRoot string) ([]string, error) {
	out, err := RunGit(gitRoot, "worktree", "list", "--porcelain")
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
	out, err := RunGit(dir, "rev-parse", "--show-toplevel")
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
	out, err := RunGit(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	gitCommon := strings.TrimSpace(out)
	// --git-common-dir may answer with a path relative to dir, so dir itself
	// must be absolute before joining — otherwise the result stays relative
	// to whatever the caller's working directory happens to be, and every
	// caller that compares or stores this value (quest lookup, worktree
	// dedup, checkDir) would need to know to canonicalize it first.
	if !filepath.IsAbs(gitCommon) {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("resolving %q: %w", dir, err)
		}
		gitCommon = filepath.Join(absDir, gitCommon)
	}
	// The main repo root is the parent of the shared .git directory.
	return filepath.Dir(filepath.Clean(gitCommon)), nil
}
