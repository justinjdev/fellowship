package hooks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
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
	// TargetTopLevel is the git working-tree root the TARGET FILE lives in,
	// resolved from the file's own path rather than from the session's working
	// directory. A teammate sitting in its own worktree can still name an
	// absolute path in the main tree, and where the write LANDS is what makes
	// it a mis-placement. "" when git could not answer, in which case the
	// session's top-level stands in.
	TargetTopLevel string
	// ToolName is the PreToolUse tool name (Edit, Write, NotebookEdit, ...).
	ToolName string
	// FilePath is the absolute, cleaned path to the tool's target file.
	FilePath string
	// DataDirName is the fellowship data directory name configured for the MAIN
	// repo (datadir.Resolve(mainRoot), e.g. ".fellowship" or a user override).
	// Writes under it are coordination state, allowed in the main tree for a
	// session standing there — the store file excepted.
	DataDirName string
	// SessionID is the Claude Code session id from the hook payload, or "" if
	// the payload carried none.
	SessionID string
	// AgentID is the subagent id from the hook payload, "" for the main
	// conversation. A teammate spawned with the Agent tool runs in-process
	// and carries the lead's session id, so it is the agent id — not the
	// session id — that tells the lead's own writes from a teammate's.
	AgentID string
	// LeadSessionID is the session id `fellowship state init` recorded in the
	// store's lead row, or "" when no lead is recorded.
	LeadSessionID string
	// SessionIsRegisteredQuest reports that the session's git top-level is
	// registered as a quest worktree in fellowship state. Combined with a write
	// that lands in the main tree, it is a positive mis-placement: either a
	// quest provisioned into the main working tree, or a teammate reaching into
	// it from its own worktree.
	SessionIsRegisteredQuest bool
}

// IsolationGuard is the fail-OPEN backstop for worktree isolation: it blocks
// only on a positive mis-placement detection, and every uncertainty (no
// fellowship active, unresolved paths, non-mutating tool, an unidentifiable
// writer) allows. During an active fellowship, a quest teammate's source writes
// must land inside its own git worktree; a write that lands in the MAIN
// worktree from a session that is not the lead has had isolation skipped, and
// is blocked — whether the session was dropped into the main tree or reached it
// by absolute path from its own worktree. Writes inside the teammate's own
// worktree, non-mutating tools, and the lead's own session are never blocked.
// This is defense-in-depth: lead-created `isolation: "worktree"` is the primary
// guarantee.
func IsolationGuard(p IsolationParams) HookResult {
	// Inert unless a fellowship is active — installing the guard is always safe.
	if !p.FellowshipActive {
		return HookResult{}
	}
	// Only source-mutating tools are guarded; Bash/git are left to Gandalf.
	if !isSourceMutatingTool(p.ToolName) {
		return HookResult{}
	}
	if p.FilePath == "" {
		return HookResult{}
	}
	// Where the write LANDS decides, not where the session happens to stand.
	// The guard used to return here unless the session's own top-level was the
	// main root, so a teammate in its worktree could write any absolute path in
	// the main tree and never be looked at.
	targetRoot := p.TargetTopLevel
	if targetRoot == "" {
		targetRoot = p.SessionTopLevel
	}
	if !samePath(targetRoot, p.MainRoot) {
		return HookResult{}
	}
	rel, ok := relWithin(p.MainRoot, p.FilePath)
	if !ok {
		// Target lives outside the main worktree — not our concern.
		return HookResult{}
	}
	// The coordination-path exemption is scoped to the session's own tree. A
	// teammate in its worktree has no business hand-editing the MAIN tree's
	// .git, .claude or data directory; only a session standing in the main tree
	// (the lead, or the unidentifiable writer rule 4 lets through) gets it.
	if samePath(p.SessionTopLevel, p.MainRoot) && isCoordinationPath(rel, p.DataDirName) {
		return HookResult{}
	}

	// Everything above says "a source write is happening in the main working
	// tree". That alone is NOT a mis-placement: the main tree is the lead's own
	// workspace, and blocking it blocked the lead out of its own repo. Who is
	// writing decides, in this order:
	//
	//  1. The session that ran `fellowship state init`, in its own
	//     conversation (no agent id), is the lead — allow.
	//  2. A session registered as a quest worktree, writing here, is a
	//     teammate: either provisioned into the main tree, or reaching into it
	//     from its own worktree — block, with or without session ids.
	//  3. A subagent (the payload carries an agent id), or a known session
	//     that is not the recorded lead, during an active fellowship, is a
	//     teammate in the wrong tree — block. The lead's own conversation is
	//     the only thing that writes source in the main tree.
	//  4. Otherwise the writer cannot be identified. The guard is a fail-open
	//     backstop behind lead-provisioned isolation, so it allows.
	if p.AgentID == "" && IsLeadSession(p.SessionID, p.LeadSessionID) {
		return HookResult{}
	}
	if p.SessionIsRegisteredQuest {
		if samePath(p.SessionTopLevel, p.MainRoot) {
			return blockMainTreeWrite(rel, "this worktree is registered as a quest but resolves to the main working tree")
		}
		return blockMainTreeWrite(rel, "this session is a registered quest worktree writing into the main working tree")
	}
	if p.AgentID != "" {
		return blockMainTreeWrite(rel,
			"this write comes from a subagent (agent "+p.AgentID+"), and only the lead's own conversation writes in the main tree")
	}
	if p.SessionID != "" && p.LeadSessionID != "" {
		return blockMainTreeWrite(rel,
			"this session is not the lead recorded by \"fellowship state init\" — if it is, re-record it with \"fellowship state init --claim-lead\"")
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
// The store is the one thing inside the data directory that is never exempt:
// it is the enforcement state, not a coordination file, and a session that can
// write it can name itself the lead.
func isCoordinationPath(rel, dataDirName string) bool {
	if datadir.IsStorePath(rel) {
		return false
	}
	first := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	if first == ".git" || first == ".claude" {
		return true
	}
	return dataDirName != "" && first == dataDirName
}
