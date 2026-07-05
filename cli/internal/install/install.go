// Package install registers fellowship's project-level hooks into a repo's
// .claude/settings.local.json. Teammate sessions spawned via the Agent tool do
// NOT inherit plugin hooks, so the worktree-guard must live in a project
// settings file the teammate's session reads. settings.local.json is used
// because it is git-ignored: it never touches git history and never shows up as
// an untracked file. The lead copies it into each worktree at spawn.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// These mirror the worktree-guard PreToolUse hook in plugin/hooks/hooks.json.
// Keep them in sync with that file.
const (
	worktreeGuardMatcher = "Edit|Write|NotebookEdit"
	worktreeGuardCommand = "${HOME}/.claude/fellowship/bin/fellowship hook worktree-guard"
	worktreeGuardTimeout = 5
)

// EnsureWorktreeGuardHook merges the worktree-guard PreToolUse hook into the
// project's .claude/settings.local.json, preserving every other setting and
// hook. It is idempotent: if the hook command is already registered it returns
// (false, nil) without touching the file, and returns (true, nil) when it
// writes a change. A settings file whose "hooks"/"PreToolUse" values have an
// unexpected shape is reported as an error and left untouched rather than
// clobbered.
func EnsureWorktreeGuardHook(projectDir string) (bool, error) {
	claudeDir := filepath.Join(projectDir, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.local.json")

	settings := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &settings); err != nil {
				return false, fmt.Errorf("parsing %s: %w", settingsPath, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	hooks, preToolUse, err := hooksAndPreToolUse(settings)
	if err != nil {
		return false, err
	}
	if hookCommandPresent(preToolUse, worktreeGuardCommand) {
		return false, nil
	}

	entry := map[string]any{
		"matcher": worktreeGuardMatcher,
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": worktreeGuardCommand,
				"timeout": worktreeGuardTimeout,
			},
		},
	}
	// Prepend so the guard runs before other PreToolUse hooks, matching the
	// ordering in plugin/hooks/hooks.json.
	hooks["PreToolUse"] = append([]any{entry}, preToolUse...)

	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return false, fmt.Errorf("creating %s: %w", claudeDir, err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')

	tmp := settingsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, settingsPath); err != nil {
		return false, fmt.Errorf("renaming %s: %w", tmp, err)
	}
	return true, nil
}

// hooksAndPreToolUse returns the settings["hooks"] object (creating it if
// absent) and its "PreToolUse" array (empty if absent). It errors if either
// existing value has an unexpected type, so a malformed file is never silently
// overwritten.
func hooksAndPreToolUse(settings map[string]any) (map[string]any, []any, error) {
	hooksAny, ok := settings["hooks"]
	if !ok || hooksAny == nil {
		hooksAny = map[string]any{}
		settings["hooks"] = hooksAny
	}
	hooks, ok := hooksAny.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf(`"hooks" is not an object`)
	}
	preAny, ok := hooks["PreToolUse"]
	if !ok || preAny == nil {
		return hooks, []any{}, nil
	}
	pre, ok := preAny.([]any)
	if !ok {
		return nil, nil, fmt.Errorf(`"hooks.PreToolUse" is not an array`)
	}
	return hooks, pre, nil
}

// hookCommandPresent reports whether any PreToolUse matcher already registers
// the given hook command.
func hookCommandPresent(preToolUse []any, command string) bool {
	for _, m := range preToolUse {
		matcher, ok := m.(map[string]any)
		if !ok {
			continue
		}
		entries, ok := matcher["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range entries {
			hook, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hook["command"].(string); ok && cmd == command {
				return true
			}
		}
	}
	return false
}
