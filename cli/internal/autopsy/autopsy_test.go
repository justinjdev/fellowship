package autopsy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // Pin HOME so datadir.Name() returns default
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, datadir.DefaultName, autopsyDir), 0755)
	return dir
}

func TestCreate_ValidInput(t *testing.T) {
	repo := setupTestRepo(t)
	input := &CreateInput{
		Quest:      "quest-1",
		Task:       "Add auth endpoint",
		Phase:      "Implement",
		Trigger:    "recovery",
		Files:      []string{"src/auth/jwt.go"},
		Modules:    []string{"auth"},
		WhatFailed: "Middleware caches tokens",
		Resolution: "Added cache invalidation",
		Tags:       []string{"caching"},
	}

	path, err := Create(repo, input)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading autopsy file: %v", err)
	}

	var a Autopsy
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parsing autopsy: %v", err)
	}

	if a.Version != 1 {
		t.Errorf("version = %d, want 1", a.Version)
	}
	if a.Quest != "quest-1" {
		t.Errorf("quest = %q, want %q", a.Quest, "quest-1")
	}
	if a.Trigger != "recovery" {
		t.Errorf("trigger = %q, want %q", a.Trigger, "recovery")
	}
	if a.WhatFailed != "Middleware caches tokens" {
		t.Errorf("what_failed = %q", a.WhatFailed)
	}
	if len(a.Tags) != 1 || a.Tags[0] != "caching" {
		t.Errorf("tags = %v, want [caching]", a.Tags)
	}
}

func TestCreate_MissingQuest(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := Create(repo, &CreateInput{
		Trigger:    "recovery",
		WhatFailed: "something",
	})
	if err == nil {
		t.Error("expected error for missing quest")
	}
}

func TestCreate_InvalidTrigger(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := Create(repo, &CreateInput{
		Quest:      "quest-1",
		Trigger:    "invalid",
		WhatFailed: "something",
	})
	if err == nil {
		t.Error("expected error for invalid trigger")
	}
}

func TestCreate_MissingWhatFailed(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := Create(repo, &CreateInput{
		Quest:   "quest-1",
		Trigger: "recovery",
	})
	if err == nil {
		t.Error("expected error for missing what_failed")
	}
}

func TestCreate_NilInput(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := Create(repo, nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestCreate_NilSlicesDefaultToEmpty(t *testing.T) {
	repo := setupTestRepo(t)
	path, err := Create(repo, &CreateInput{
		Quest:      "quest-1",
		Trigger:    "recovery",
		WhatFailed: "something",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading autopsy file: %v", err)
	}
	var a Autopsy
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parsing autopsy: %v", err)
	}

	if a.Files == nil {
		t.Error("files should be empty slice, not nil")
	}
	if a.Modules == nil {
		t.Error("modules should be empty slice, not nil")
	}
	if a.Tags == nil {
		t.Error("tags should be empty slice, not nil")
	}
}

func TestScan_MatchByFile(t *testing.T) {
	repo := setupTestRepo(t)
	Create(repo, &CreateInput{
		Quest:      "quest-1",
		Trigger:    "recovery",
		Files:      []string{"src/auth/jwt.go"},
		WhatFailed: "auth issue",
	})

	matches, err := Scan(repo, ScanOptions{Files: []string{"src/auth/middleware.go"}}, 90)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match (same directory), got %d", len(matches))
	}
}

func TestScan_MatchByModule(t *testing.T) {
	repo := setupTestRepo(t)
	Create(repo, &CreateInput{
		Quest:      "quest-1",
		Trigger:    "recovery",
		Modules:    []string{"auth"},
		WhatFailed: "auth issue",
	})

	matches, err := Scan(repo, ScanOptions{Modules: []string{"auth"}}, 90)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestScan_MatchByTag(t *testing.T) {
	repo := setupTestRepo(t)
	Create(repo, &CreateInput{
		Quest:      "quest-1",
		Trigger:    "recovery",
		Tags:       []string{"caching", "auth"},
		WhatFailed: "cache issue",
	})

	matches, err := Scan(repo, ScanOptions{Tags: []string{"caching"}}, 90)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestScan_NoMatch(t *testing.T) {
	repo := setupTestRepo(t)
	Create(repo, &CreateInput{
		Quest:      "quest-1",
		Trigger:    "recovery",
		Modules:    []string{"auth"},
		WhatFailed: "auth issue",
	})

	matches, err := Scan(repo, ScanOptions{Modules: []string{"billing"}}, 90)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestScan_PrunesExpired(t *testing.T) {
	repo := setupTestRepo(t)

	// Write an autopsy with an old timestamp
	dir := filepath.Join(repo, datadir.DefaultName, autopsyDir)
	old := &Autopsy{
		Version:    1,
		Timestamp:  time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339),
		Quest:      "old-quest",
		Trigger:    "recovery",
		Modules:    []string{"auth"},
		Files:      []string{},
		Tags:       []string{},
		WhatFailed: "old failure",
	}
	data, _ := json.MarshalIndent(old, "", "  ")
	os.WriteFile(filepath.Join(dir, "old-autopsy.json"), data, 0644)

	matches, err := Scan(repo, ScanOptions{Modules: []string{"auth"}}, 90)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expired autopsy should be pruned, got %d matches", len(matches))
	}

	// Verify file was deleted
	if _, err := os.Stat(filepath.Join(dir, "old-autopsy.json")); !os.IsNotExist(err) {
		t.Error("expired autopsy file should be deleted")
	}
}

func TestScan_RequiresFilter(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := Scan(repo, ScanOptions{}, 90)
	if err == nil {
		t.Error("expected error when no filters provided")
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	repo := t.TempDir() // no autopsies dir
	matches, err := Scan(repo, ScanOptions{Modules: []string{"auth"}}, 90)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty dir, got %d", len(matches))
	}
}

func TestInfer_FromRespawns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	worktree := t.TempDir()
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, datadir.DefaultName, autopsyDir), 0755)

	// Write a tome with respawns
	tomeDir := filepath.Join(worktree, datadir.DefaultName)
	os.MkdirAll(tomeDir, 0755)
	qt := &tome.QuestTome{
		Version:   1,
		QuestName: "quest-respawned",
		Task:      "Fix login flow",
		Status:    "active",
		PhasesCompleted: []tome.PhaseRecord{
			{Phase: "Implement", CompletedAt: time.Now().UTC().Format(time.RFC3339)},
		},
		GateHistory:  []tome.GateEvent{},
		FilesTouched: []string{"src/auth/login.go", "src/auth/session.go"},
		Respawns:     2,
	}
	tome.Save(filepath.Join(tomeDir, "quest-tome.json"), qt)

	path, err := Infer(worktree, repo)
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading autopsy file: %v", err)
	}
	var a Autopsy
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parsing autopsy: %v", err)
	}

	if a.Trigger != "recovery" {
		t.Errorf("trigger = %q, want recovery", a.Trigger)
	}
	if a.Quest != "quest-respawned" {
		t.Errorf("quest = %q", a.Quest)
	}
	if len(a.Files) != 2 {
		t.Errorf("files = %v, want 2 files", a.Files)
	}
}

func TestInfer_FromRejection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	worktree := t.TempDir()
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, datadir.DefaultName, autopsyDir), 0755)

	// Write tome
	tomeDir := filepath.Join(worktree, datadir.DefaultName)
	os.MkdirAll(tomeDir, 0755)
	qt := &tome.QuestTome{
		Version:         1,
		QuestName:       "quest-rejected",
		Task:            "Add billing",
		Status:          "active",
		PhasesCompleted: []tome.PhaseRecord{{Phase: "Plan", CompletedAt: time.Now().UTC().Format(time.RFC3339)}},
		GateHistory:     []tome.GateEvent{},
		FilesTouched:    []string{"src/billing/charge.go"},
	}
	tome.Save(filepath.Join(tomeDir, "quest-tome.json"), qt)

	// Write herald with rejection
	herald.Announce(worktree, herald.Tiding{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Quest:     "quest-rejected",
		Type:      herald.GateRejected,
		Phase:     "Plan",
		Detail:    "Plan doesn't account for tax calculation",
	})

	path, err := Infer(worktree, repo)
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading autopsy file: %v", err)
	}
	var a Autopsy
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parsing autopsy: %v", err)
	}

	if a.Trigger != "rejection" {
		t.Errorf("trigger = %q, want rejection", a.Trigger)
	}
	if a.WhatFailed != "Plan doesn't account for tax calculation" {
		t.Errorf("what_failed = %q", a.WhatFailed)
	}
}

func TestInfer_FromAbandonment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	worktree := t.TempDir()
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, datadir.DefaultName, autopsyDir), 0755)

	tomeDir := filepath.Join(worktree, datadir.DefaultName)
	os.MkdirAll(tomeDir, 0755)
	qt := &tome.QuestTome{
		Version:         1,
		QuestName:       "quest-abandoned",
		Task:            "Migrate DB",
		Status:          "cancelled",
		PhasesCompleted: []tome.PhaseRecord{},
		GateHistory:     []tome.GateEvent{},
		FilesTouched:    []string{},
	}
	tome.Save(filepath.Join(tomeDir, "quest-tome.json"), qt)

	path, err := Infer(worktree, repo)
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading autopsy file: %v", err)
	}
	var a Autopsy
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parsing autopsy: %v", err)
	}

	if a.Trigger != "abandonment" {
		t.Errorf("trigger = %q, want abandonment", a.Trigger)
	}
}

func TestInfer_NoFailureSignals(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	worktree := t.TempDir()
	repo := t.TempDir()

	tomeDir := filepath.Join(worktree, datadir.DefaultName)
	os.MkdirAll(tomeDir, 0755)
	qt := &tome.QuestTome{
		Version:         1,
		QuestName:       "quest-ok",
		Task:            "Add feature",
		Status:          "active",
		PhasesCompleted: []tome.PhaseRecord{},
		GateHistory:     []tome.GateEvent{},
		FilesTouched:    []string{},
	}
	tome.Save(filepath.Join(tomeDir, "quest-tome.json"), qt)

	_, err := Infer(worktree, repo)
	if err == nil {
		t.Error("expected error when no failure signals found")
	}
}

func TestMatchesFilters_FilePrefix(t *testing.T) {
	a := &Autopsy{Files: []string{"src/auth/jwt.go"}}

	// Same directory should match
	if !matchesFilters(a, ScanOptions{Files: []string{"src/auth/middleware.go"}}) {
		t.Error("same directory should match")
	}

	// Parent prefix should match
	if !matchesFilters(a, ScanOptions{Files: []string{"src/auth/"}}) {
		t.Error("parent prefix should match")
	}

	// Different directory should not match
	if matchesFilters(a, ScanOptions{Files: []string{"src/billing/charge.go"}}) {
		t.Error("different directory should not match")
	}
}

func TestInferModules(t *testing.T) {
	// All files start with "src", so we expect a single "src" module
	modules := inferModules([]string{"src/auth/jwt.go", "src/auth/session.go", "src/billing/charge.go"})
	if len(modules) != 1 || modules[0] != "src" {
		t.Errorf("expected [src], got %v", modules)
	}

	// Different top-level dirs produce separate modules, sorted
	modules = inferModules([]string{"auth/jwt.go", "billing/charge.go"})
	if len(modules) != 2 || modules[0] != "auth" || modules[1] != "billing" {
		t.Errorf("expected [auth billing], got %v", modules)
	}
}

func TestSanitize(t *testing.T) {
	if got := sanitize("quest with spaces"); got != "quest-with-spaces" {
		t.Errorf("sanitize spaces: got %q", got)
	}
	if got := sanitize("quest/with/slashes"); got != "quest-with-slashes" {
		t.Errorf("sanitize slashes: got %q", got)
	}
	long := "this-is-a-very-long-quest-name-that-exceeds-the-forty-character-limit"
	if got := sanitize(long); len(got) != 40 {
		t.Errorf("sanitize long: got length %d, want 40", len(got))
	}
}

