package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}
	return m
}

func preToolUse(t *testing.T, m map[string]any) []any {
	t.Helper()
	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing or wrong type: %T", m["hooks"])
	}
	pre, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatalf("PreToolUse missing or wrong type: %T", hooks["PreToolUse"])
	}
	return pre
}

func TestEnsureWorktreeGuardHook_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	changed, err := EnsureWorktreeGuardHook(dir)
	if err != nil {
		t.Fatalf("EnsureWorktreeGuardHook failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true when creating a new settings.json")
	}
	if !hookCommandPresent(preToolUse(t, readSettings(t, dir)), worktreeGuardCommand) {
		t.Error("worktree-guard command not registered")
	}
}

func TestEnsureWorktreeGuardHook_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := EnsureWorktreeGuardHook(dir); err != nil {
		t.Fatalf("first install failed: %v", err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))

	changed, err := EnsureWorktreeGuardHook(dir)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if changed {
		t.Error("expected changed=false on a repeat install")
	}
	after, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if string(before) != string(after) {
		t.Error("settings.json should be byte-identical after an idempotent call")
	}

	// Exactly one guard matcher — no duplication.
	count := 0
	for _, m := range preToolUse(t, readSettings(t, dir)) {
		if hookCommandPresent([]any{m}, worktreeGuardCommand) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one guard matcher, got %d", count)
	}
}

func TestEnsureWorktreeGuardHook_PreservesExistingSettingsAndHooks(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	existing := `{
	  "permissions": {"allow": ["Read"]},
	  "hooks": {
	    "PreToolUse": [
	      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo other"}]}
	    ],
	    "PostToolUse": [
	      {"matcher": "Skill", "hooks": [{"type": "command", "command": "echo post"}]}
	    ]
	  }
	}`
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(existing), 0o644)

	changed, err := EnsureWorktreeGuardHook(dir)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true when adding the guard to an existing file")
	}

	m := readSettings(t, dir)
	if _, ok := m["permissions"]; !ok {
		t.Error("unrelated top-level key 'permissions' was dropped")
	}
	hooks := m["hooks"].(map[string]any)
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("PostToolUse hooks were dropped")
	}
	pre := preToolUse(t, m)
	if !hookCommandPresent(pre, worktreeGuardCommand) {
		t.Error("guard command not registered")
	}
	if !hookCommandPresent(pre, "echo other") {
		t.Error("pre-existing PreToolUse hook was dropped")
	}
	// Guard is prepended so it runs first.
	if !hookCommandPresent([]any{pre[0]}, worktreeGuardCommand) {
		t.Error("guard should be the first PreToolUse matcher")
	}
}

func TestEnsureWorktreeGuardHook_MalformedHooksIsNotClobbered(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	original := `{"hooks": "not-an-object"}`
	path := filepath.Join(claudeDir, "settings.local.json")
	os.WriteFile(path, []byte(original), 0o644)

	changed, err := EnsureWorktreeGuardHook(dir)
	if err == nil {
		t.Fatal("expected an error for malformed hooks, got nil")
	}
	if changed {
		t.Error("changed must be false when refusing a malformed file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Error("malformed settings.json must be left untouched")
	}
}

func TestEnsureWorktreeGuardHook_CommandMatchesHooksJSON(t *testing.T) {
	// Guards against drift from plugin/hooks/hooks.json.
	if !strings.Contains(worktreeGuardCommand, "fellowship hook worktree-guard") {
		t.Errorf("guard command %q should invoke `fellowship hook worktree-guard`", worktreeGuardCommand)
	}
}
