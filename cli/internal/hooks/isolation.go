package hooks

import (
	"fmt"
	"path/filepath"
	"strings"
)

// IsolationParams carries the facts the isolation guard needs, already resolved
// by the caller (git top-levels, fellowship-active flag, absolute file path).
// Keeping the decision pure makes it testable without spawning git or opening a DB.
type IsolationParams struct {
	// FellowshipActive is true when a fellowship is initialized in the main repo.
	FellowshipActive bool
	// MainRoot is the absolute path to the main worktree root (parent of the
	// shared git common dir).
	MainRoot string
	// SessionTopLevel is the absolute path to the current session's git top-level
	// (`git rev-parse --show-toplevel`).
	SessionTopLevel string
	// ToolName is the PreToolUse tool name (Edit, Write, NotebookEdit, ...).
	ToolName string
	// FilePath is the absolute, cleaned path to the tool's target file.
	FilePath string
	// DataDirName is the configured fellowship data directory name
	// (datadir.Name(), e.g. ".fellowship" or a user override). Writes under it
	// are coordination state, always allowed even in the main tree.
	DataDirName string
}

// IsolationGuard is the fail-closed backstop for worktree isolation. During an
// active fellowship, a quest teammate must operate inside its own git worktree.
// If a session whose top-level IS the main worktree root tries to Edit/Write a
// source file there, the teammate was mis-placed (isolation was skipped) and the
// write is blocked. Teammates in their own worktree, and non-mutating tools, are
// never blocked. This is defense-in-depth: lead-created `isolation: "worktree"`
// is the primary guarantee.
func IsolationGuard(p IsolationParams) HookResult {
	// Inert unless a fellowship is active — installing the guard is always safe.
	if !p.FellowshipActive {
		return HookResult{}
	}
	// Only source-mutating tools are guarded; Bash/git are left to Gandalf.
	if !isSourceMutatingTool(p.ToolName) {
		return HookResult{}
	}
	// The whole point: a session correctly in its own worktree is never blocked.
	if !samePath(p.SessionTopLevel, p.MainRoot) {
		return HookResult{}
	}
	if p.FilePath == "" {
		return HookResult{}
	}
	rel, ok := relWithin(p.MainRoot, p.FilePath)
	if !ok {
		// Target lives outside the main worktree — not our concern.
		return HookResult{}
	}
	if isCoordinationPath(rel, p.DataDirName) {
		return HookResult{}
	}
	return HookResult{
		Block: true,
		Message: fmt.Sprintf(
			"worktree-guard: refusing to write '%s' in the MAIN working tree during an active fellowship; quest work belongs in your isolated worktree",
			filepath.ToSlash(rel),
		),
	}
}

// isSourceMutatingTool reports whether the tool writes files we protect.
func isSourceMutatingTool(name string) bool {
	switch name {
	case "Edit", "Write", "NotebookEdit":
		return true
	default:
		return false
	}
}

// samePath compares two paths for equality after cleaning.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// relWithin returns the path of target relative to root and whether target is
// inside root. A target equal to or outside root returns ok=false.
func relWithin(root, target string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

// isCoordinationPath reports whether a root-relative path lives under a
// coordination or git-metadata directory that is exempt from the guard. Gandalf
// legitimately manages these even in the main tree. The data directory is
// user-configurable (datadir.Name), so the caller passes its resolved name
// rather than assuming the ".fellowship" default; .git and .claude are fixed.
func isCoordinationPath(rel, dataDirName string) bool {
	first := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	if first == ".git" || first == ".claude" {
		return true
	}
	return dataDirName != "" && first == dataDirName
}
