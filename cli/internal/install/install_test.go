package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	err := Install(dir, "/path/to/fellowship")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	path := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["hooks"]; !ok {
		t.Error("hooks key missing")
	}
}

func TestInstall_MergesWithExisting(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"allowedTools":["Read"],"permissions":{"allow":true}}`), 0644)

	err := Install(dir, "/path/to/fellowship")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["hooks"]; !ok {
		t.Error("hooks key missing after merge")
	}
	if _, ok := m["allowedTools"]; !ok {
		t.Error("allowedTools lost during merge")
	}
	if _, ok := m["permissions"]; !ok {
		t.Error("permissions lost during merge")
	}
}

func TestUninstall_RemovesHooksKeepsRest(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"hooks":{"PreToolUse":[]},"allowedTools":["Read"]}`), 0644)

	err := Uninstall(dir)
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["hooks"]; ok {
		t.Error("hooks key should be removed")
	}
	if _, ok := m["allowedTools"]; !ok {
		t.Error("allowedTools should be preserved")
	}
}

func TestUninstall_RemovesFileWhenHooksOnly(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)
	path := filepath.Join(claudeDir, "settings.json")
	os.WriteFile(path, []byte(`{"hooks":{"PreToolUse":[]}}`), 0644)

	Uninstall(dir)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be removed when hooks were the only key")
	}
}

func TestUninstall_NoopsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	err := Uninstall(dir)
	if err != nil {
		t.Fatalf("Uninstall should noop, got: %v", err)
	}
}

func TestInstall_HookCommandsUseBinPath(t *testing.T) {
	dir := t.TempDir()
	Install(dir, "/custom/path/fellowship")
	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	s := string(data)
	if !strings.Contains(s, "/custom/path/fellowship") {
		t.Error("hook commands should use the provided bin path")
	}
}
