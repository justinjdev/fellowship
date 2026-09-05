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
	// SessionID is the Claude Code session id from the hook payload, or "" if
	// the payload carried none.
	SessionID string
	// LeadSessionID is the session id `fellowship state init` recorded in the
	// lead marker, or "" when there is no marker or it holds no id.
	LeadSessionID string
	// SessionIsRegisteredQuest reports that the session's git top-level is
	// registered as a quest worktree in fellowship state. Combined with a
	// top-level that IS the main root, it is a positive mis-placement: a quest
	// was provisioned in the main working tree.
	SessionIsRegisteredQuest bool
}

// IsolationGuard is the fail-OPEN backstop for worktree isolation: it blocks
// only on a positive mis-placement detection, and every uncertainty (no
// fellowship active, unresolved paths, non-mutating tool, an unidentifiable
// writer) allows. During an active fellowship, a quest teammate must operate
// inside its own git worktree; a session whose top-level IS the main worktree
// root and that is not the lead has had isolation skipped, and its source
// writes are blocked. Teammates in their own worktree, non-mutating tools, and
// the lead's own session are never blocked. This is defense-in-depth:
// lead-created `isolation: "worktree"` is the primary guarantee.
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

	// Everything above says "a source write is happening in the main working
	// tree". That alone is NOT a mis-placement: the main tree is the lead's own
	// workspace, and blocking it blocked the lead out of its own repo. Who is
	// writing decides, in this order:
	//
	//  1. The session that ran `fellowship state init` is the lead — allow.
	//  2. A session whose top-level is BOTH the main root and a registered
	//     quest worktree is a quest provisioned into the main tree — block,
	//     with or without session ids.
	//  3. A known session that is not the recorded lead, during an active
	//     fellowship, is a teammate in the wrong tree — block.
	//  4. Otherwise the writer cannot be identified. The guard is a fail-open
	//     backstop behind lead-provisioned isolation, so it allows.
	if p.SessionID != "" && p.SessionID == p.LeadSessionID {
		return HookResult{}
	}
	if p.SessionIsRegisteredQuest {
		return blockMainTreeWrite(rel, "this worktree is registered as a quest but resolves to the main working tree")
	}
	if p.SessionID != "" && p.LeadSessionID != "" {
		marker := "lead marker in the fellowship data directory"
		if p.DataDirName != "" {
			marker = p.DataDirName + "/lead marker"
		}
		return blockMainTreeWrite(rel, fmt.Sprintf(
			"this session is not the lead recorded by \"fellowship state init\" — if it is, delete the %s", marker))
	}
	return HookResult{}
}

// blockMainTreeWrite builds the guard's refusal, naming the file and why the
// writer was taken for a mis-placed teammate rather than the lead.
func blockMainTreeWrite(rel, reason string) HookResult {
	return HookResult{
		Block: true,
		Message: fmt.Sprintf(
			"worktree-guard: refusing to write '%s' in the MAIN working tree during an active fellowship; quest work belongs in your isolated worktree (%s)",
			filepath.ToSlash(rel), reason,
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
