package datadir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// resetCache resets the sync.Once cache so each test gets a fresh Name() call.
func resetCache(t *testing.T) {
	t.Helper()
	nameOnce = sync.Once{}
	cachedName = ""
	t.Cleanup(func() {
		nameOnce = sync.Once{}
		cachedName = ""
	})
}

func TestDefaultName(t *testing.T) {
	if DefaultName != ".fellowship" {
		t.Errorf("DefaultName = %q, want %q", DefaultName, ".fellowship")
	}
}

func TestName_NoConfigFile(t *testing.T) {
	resetCache(t)
	// Point to a non-existent config dir
	t.Setenv("HOME", t.TempDir())

	got := Name()
	if got != DefaultName {
		t.Errorf("Name() = %q, want %q", got, DefaultName)
	}
}

func TestName_ConfigWithDataDir(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("creating claude dir: %v", err)
	}

	cfg := map[string]string{"dataDir": ".my-custom-dir"}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), data, 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	got := Name()
	if got != ".my-custom-dir" {
		t.Errorf("Name() = %q, want %q", got, ".my-custom-dir")
	}
}

func TestName_RejectsPathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
	}{
		{"slash", "foo/bar"},
		{"backslash", "foo\\bar"},
		{"dot-dot", ".."},
		{"dot-dot-slash", "../etc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCache(t)
			home := t.TempDir()
			t.Setenv("HOME", home)

			claudeDir := filepath.Join(home, ".claude")
			if err := os.MkdirAll(claudeDir, 0755); err != nil {
				t.Fatalf("creating claude dir: %v", err)
			}
			cfg := map[string]string{"dataDir": tt.dataDir}
			data, _ := json.Marshal(cfg)
			if err := os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), data, 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			got := Name()
			if got != DefaultName {
				t.Errorf("Name() = %q, want %q (should reject path traversal)", got, DefaultName)
			}
		})
	}
}

func TestName_ConfigWithEmptyDataDir(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)

	cfg := map[string]string{"dataDir": ""}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), data, 0644)

	got := Name()
	if got != DefaultName {
		t.Errorf("Name() = %q, want %q", got, DefaultName)
	}
}

func TestName_ConfigWithNoDataDir(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)

	cfg := map[string]string{"somethingElse": "value"}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), data, 0644)

	got := Name()
	if got != DefaultName {
		t.Errorf("Name() = %q, want %q", got, DefaultName)
	}
}

func TestName_InvalidJSON(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), []byte("{invalid"), 0644)

	got := Name()
	if got != DefaultName {
		t.Errorf("Name() = %q, want %q", got, DefaultName)
	}
}

func TestName_ProjectConfigDataDir(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	// No user config — only project config.
	repoRoot := t.TempDir()
	fellowshipDir := filepath.Join(repoRoot, DefaultName)
	if err := os.MkdirAll(fellowshipDir, 0755); err != nil {
		t.Fatalf("creating fellowship dir: %v", err)
	}
	cfg := map[string]string{"dataDir": ".project-dir"}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(fellowshipDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("writing project config: %v", err)
	}

	orig := gitRootFunc
	t.Cleanup(func() { gitRootFunc = orig })
	gitRootFunc = func() (string, error) { return repoRoot, nil }

	got := Name()
	if got != ".project-dir" {
		t.Errorf("Name() = %q, want %q", got, ".project-dir")
	}
}

func TestName_UserOverridesProjectConfig(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Project config sets one value; user config overrides it.
	repoRoot := t.TempDir()
	fellowshipDir := filepath.Join(repoRoot, DefaultName)
	os.MkdirAll(fellowshipDir, 0755)
	projectCfg := map[string]string{"dataDir": ".project-dir"}
	data, _ := json.Marshal(projectCfg)
	os.WriteFile(filepath.Join(fellowshipDir, "config.json"), data, 0644)

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	userCfg := map[string]string{"dataDir": ".user-dir"}
	data, _ = json.Marshal(userCfg)
	os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), data, 0644)

	orig := gitRootFunc
	t.Cleanup(func() { gitRootFunc = orig })
	gitRootFunc = func() (string, error) { return repoRoot, nil }

	got := Name()
	if got != ".user-dir" {
		t.Errorf("Name() = %q, want %q (user should override project)", got, ".user-dir")
	}
}

func TestName_ProjectConfigNoGitRoot(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	orig := gitRootFunc
	t.Cleanup(func() { gitRootFunc = orig })
	gitRootFunc = func() (string, error) { return "", os.ErrNotExist }

	got := Name()
	if got != DefaultName {
		t.Errorf("Name() = %q, want %q (should fallback when no git root)", got, DefaultName)
	}
}

func TestName_ProjectConfigInvalidJSON(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	repoRoot := t.TempDir()
	fellowshipDir := filepath.Join(repoRoot, DefaultName)
	os.MkdirAll(fellowshipDir, 0755)
	os.WriteFile(filepath.Join(fellowshipDir, "config.json"), []byte("{invalid"), 0644)

	orig := gitRootFunc
	t.Cleanup(func() { gitRootFunc = orig })
	gitRootFunc = func() (string, error) { return repoRoot, nil }

	got := Name()
	if got != DefaultName {
		t.Errorf("Name() = %q, want %q (should fallback on invalid JSON)", got, DefaultName)
	}
}

func TestIsDataDirPath(t *testing.T) {
	resetCache(t)
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"absolute default", "/home/user/project/.fellowship/checkpoint.md", true},
		{"relative default", ".fellowship/quest-state.json", true},
		{"not data dir", "/home/user/project/src/main.go", false},
		{"partial match", "/home/user/project/fellowship/file.go", false},
		{"empty", "", false},
	}

	// Use default data dir for these tests
	t.Setenv("HOME", t.TempDir())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDataDirPath(tt.path)
			if got != tt.want {
				t.Errorf("IsDataDirPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFailuresExpiryDays_Default(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := FailuresExpiryDays("", 90); got != 90 {
		t.Errorf("FailuresExpiryDays = %d, want 90", got)
	}
}

func TestFailuresExpiryDays_Custom(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.WriteFile(filepath.Join(home, ".claude", "fellowship.json"), []byte(`{"failures":{"expiryDays":30}}`), 0644)

	if got := FailuresExpiryDays("", 90); got != 30 {
		t.Errorf("FailuresExpiryDays = %d, want 30", got)
	}
}

func TestFailuresExpiryDays_ProjectThenUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, DefaultName), 0755)
	os.WriteFile(filepath.Join(root, DefaultName, "config.json"), []byte(`{"failures":{"expiryDays":45}}`), 0644)

	if got := FailuresExpiryDays(root, 90); got != 45 {
		t.Errorf("project config: FailuresExpiryDays = %d, want 45", got)
	}

	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.WriteFile(filepath.Join(home, ".claude", "fellowship.json"), []byte(`{"failures":{"expiryDays":30}}`), 0644)
	if got := FailuresExpiryDays(root, 90); got != 30 {
		t.Errorf("user config should win: FailuresExpiryDays = %d, want 30", got)
	}
}

func TestIsDataDirPath_CustomDir(t *testing.T) {
	resetCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0755)
	cfg := map[string]string{"dataDir": ".custom"}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(claudeDir, "fellowship.json"), data, 0644)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"matches custom", "/project/.custom/state.json", true},
		{"relative custom", ".custom/state.json", true},
		{"default no match", "/project/.fellowship/state.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDataDirPath(tt.path)
			if got != tt.want {
				t.Errorf("IsDataDirPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	writeConfig := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		project string
		user    string
		want    string
	}{
		{name: "no config anywhere", want: DefaultName},
		{name: "project config wins over the default", project: `{"dataDir":".project-dir"}`, want: ".project-dir"},
		{name: "user config wins over the project", project: `{"dataDir":".project-dir"}`, user: `{"dataDir":".user-dir"}`, want: ".user-dir"},
		{name: "path-like names fall back to the default", project: `{"dataDir":"../escape"}`, want: DefaultName},
		{name: "malformed project config falls back", project: `{not json`, want: DefaultName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			root := t.TempDir()
			if tt.project != "" {
				writeConfig(t, filepath.Join(root, DefaultName, "config.json"), tt.project)
			}
			if tt.user != "" {
				writeConfig(t, filepath.Join(home, ".claude", "fellowship.json"), tt.user)
			}
			if got := Resolve(root); got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", root, got, tt.want)
			}
		})
	}

	// An unknown root is not an error: the default applies.
	t.Setenv("HOME", t.TempDir())
	if got := Resolve(""); got != DefaultName {
		t.Errorf("Resolve(\"\") = %q, want %q", got, DefaultName)
	}
}
