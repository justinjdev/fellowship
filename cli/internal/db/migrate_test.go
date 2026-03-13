package db

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// writeFellowshipState writes a fellowship-state.json fixture.
func writeFellowshipState(t *testing.T, dir string) {
	t.Helper()
	fs := fellowshipStateJSON{
		Version:    1,
		Name:       "test-fellowship",
		MainRepo:   "/tmp/repo",
		BaseBranch: "main",
		Quests: []fellowshipQuestJSON{
			{Name: "q1", TaskDescription: "build auth", Worktree: "/tmp/wt1", Branch: "feat/q1", TaskID: "task-1"},
		},
		Scouts: []fellowshipScoutJSON{
			{Name: "s1", Question: "how does auth work?", TaskID: "task-s1"},
		},
		Companies: []companyJSON{
			{Name: "team-a", Quests: []string{"q1"}, Scouts: []string{"s1"}},
		},
		CreatedAt: "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(fs)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "fellowship-state.json"), data, 0o644)
}

// writeQuestState writes a quest-state.json fixture.
func writeQuestState(t *testing.T, dir string) {
	t.Helper()
	gateID := "gate-onboard-12345"
	qs := questStateJSON{
		Version:         1,
		QuestName:       "q1",
		TaskID:          "task-1",
		TeamName:        "team-a",
		Phase:           "Implement",
		GatePending:     false,
		GateID:          &gateID,
		LembasCompleted: true,
		MetadataUpdated: false,
		Held:            false,
	}
	data, _ := json.Marshal(qs)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "quest-state.json"), data, 0o644)
}

// writeQuestTome writes a quest-tome.json fixture.
func writeQuestTome(t *testing.T, dir string) {
	t.Helper()
	qt := questTomeJSON{
		Version:   1,
		QuestName: "q1",
		Task:      "build auth",
		PhasesCompleted: []phaseRecJSON{
			{Phase: "Onboard", CompletedAt: "2026-01-01T00:10:00Z", DurationS: 60},
		},
		GateHistory: []gateEventJSON{
			{Phase: "Onboard", Action: "approved", Timestamp: "2026-01-01T00:10:00Z"},
		},
		FilesTouched: []string{"auth/login.go", "auth/login_test.go"},
		Respawns:     1,
		Status:       "active",
	}
	data, _ := json.Marshal(qt)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "quest-tome.json"), data, 0o644)
}

// writeQuestErrands writes a quest-errands.json fixture.
func writeQuestErrands(t *testing.T, dir string) {
	t.Helper()
	qe := questErrandsJSON{
		Version:   1,
		QuestName: "q1",
		Task:      "build auth",
		Items: []errandJSON{
			{
				ID: "w-001", Description: "implement login", Status: "pending",
				Phase: "Implement", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				DependsOn: []string{"w-000"},
			},
		},
	}
	data, _ := json.Marshal(qe)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "quest-errands.json"), data, 0o644)
}

// writeHerald writes a quest-herald.jsonl fixture.
func writeHerald(t *testing.T, dir string) {
	t.Helper()
	lines := []heraldLineJSON{
		{Timestamp: "2026-01-01T00:05:00Z", Quest: "q1", Type: "gate_submitted", Phase: "Onboard", Detail: ""},
		{Timestamp: "2026-01-01T00:10:00Z", Quest: "q1", Type: "gate_approved", Phase: "Onboard", Detail: "looks good"},
	}
	var sb strings.Builder
	for _, l := range lines {
		data, _ := json.Marshal(l)
		sb.Write(data)
		sb.WriteByte('\n')
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "quest-herald.jsonl"), []byte(sb.String()), 0o644)
}

// writeBulletin writes a bulletin.jsonl fixture.
func writeBulletin(t *testing.T, dir string) {
	t.Helper()
	lines := []bulletinLineJSON{
		{Timestamp: "2026-01-01T01:00:00Z", Quest: "q1", Topic: "auth", Files: []string{"auth/login.go"}, Discovery: "needs error handling"},
	}
	var sb strings.Builder
	for _, l := range lines {
		data, _ := json.Marshal(l)
		sb.Write(data)
		sb.WriteByte('\n')
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "bulletin.jsonl"), []byte(sb.String()), 0o644)
}

// writeAutopsy writes an autopsy JSON fixture.
func writeAutopsy(t *testing.T, dir string, name string) {
	t.Helper()
	a := autopsyJSON{
		Version:    1,
		Timestamp:  "2026-01-02T00:00:00Z",
		Quest:      "q1",
		Task:       "build auth",
		Phase:      "Implement",
		Trigger:    "recovery",
		Files:      []string{"auth/login.go"},
		Modules:    []string{"auth"},
		WhatFailed: "tests",
		Tags:       []string{"flaky"},
		ExpiresAt:  "2026-04-02T00:00:00Z",
	}
	data, _ := json.Marshal(a)
	autopsyDir := filepath.Join(dir, "autopsies")
	os.MkdirAll(autopsyDir, 0o755)
	os.WriteFile(filepath.Join(autopsyDir, name), data, 0o644)
}

func TestMigrateJSON(t *testing.T) {
	// Override execCommand so we don't need real git
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := t.TempDir()
	mainDataDir := filepath.Join(tmpDir, ".fellowship")
	wtDir := filepath.Join(tmpDir, "worktree-q1")
	wtDataDir := filepath.Join(wtDir, ".fellowship")

	// Set up git worktree mock: return main + worktree
	execCommand = func(name string, args ...string) *exec.Cmd {
		out := "worktree " + tmpDir + "\n\nworktree " + wtDir + "\n\n"
		return exec.Command("echo", "-n", out)
	}

	// Write all 7 fixture types:
	// Main .fellowship/: fellowship-state.json, bulletin.jsonl, autopsies/*.json
	writeFellowshipState(t, mainDataDir)
	writeBulletin(t, mainDataDir)
	writeAutopsy(t, mainDataDir, "autopsy-001.json")

	// Also create a .lock sidecar to verify deletion
	os.WriteFile(filepath.Join(mainDataDir, "fellowship-state.json.lock"), []byte("lock"), 0o644)

	// Worktree .fellowship/: quest-state.json, quest-tome.json, quest-errands.json, quest-herald.jsonl
	writeQuestState(t, wtDataDir)
	writeQuestTome(t, wtDataDir)
	writeQuestErrands(t, wtDataDir)
	writeHerald(t, wtDataDir)

	// Run migration
	d := OpenTest(t)
	err := MigrateJSON(d, tmpDir)
	if err != nil {
		t.Fatalf("MigrateJSON: %v", err)
	}

	// Verify all data migrated correctly
	d.WithConn(t.Context(), func(conn *Conn) error {
		// 1. Fellowship
		var name, mainRepo, baseBranch string
		sqlitex.Execute(conn,
			`SELECT name, main_repo, base_branch FROM fellowship WHERE id = 1`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					name = stmt.ColumnText(0)
					mainRepo = stmt.ColumnText(1)
					baseBranch = stmt.ColumnText(2)
					return nil
				},
			})
		assertEqual(t, "fellowship.name", "test-fellowship", name)
		assertEqual(t, "fellowship.main_repo", "/tmp/repo", mainRepo)
		assertEqual(t, "fellowship.base_branch", "main", baseBranch)

		// 2. Fellowship quests
		var questName, taskDesc, branch string
		sqlitex.Execute(conn,
			`SELECT name, task_description, branch FROM fellowship_quests WHERE name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					questName = stmt.ColumnText(0)
					taskDesc = stmt.ColumnText(1)
					branch = stmt.ColumnText(2)
					return nil
				},
			})
		assertEqual(t, "quest.name", "q1", questName)
		assertEqual(t, "quest.task_description", "build auth", taskDesc)
		assertEqual(t, "quest.branch", "feat/q1", branch)

		// 3. Fellowship scouts
		var scoutName, question string
		sqlitex.Execute(conn,
			`SELECT name, question FROM fellowship_scouts WHERE name = 's1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					scoutName = stmt.ColumnText(0)
					question = stmt.ColumnText(1)
					return nil
				},
			})
		assertEqual(t, "scout.name", "s1", scoutName)
		assertEqual(t, "scout.question", "how does auth work?", question)

		// 4. Companies and members
		var companyCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM company_members WHERE company_name = 'team-a'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					companyCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "company_members.count", 2, companyCount)

		// 5. Quest state
		var phase string
		var lembasCompleted int
		sqlitex.Execute(conn,
			`SELECT phase, lembas_completed FROM quest_state WHERE quest_name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					phase = stmt.ColumnText(0)
					lembasCompleted = stmt.ColumnInt(1)
					return nil
				},
			})
		assertEqual(t, "quest_state.phase", "Implement", phase)
		assertEqual(t, "quest_state.lembas_completed", 1, lembasCompleted)

		// 6. Quest phases (from tome)
		var phaseCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM quest_phases WHERE quest_name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					phaseCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "quest_phases.count", 1, phaseCount)

		// 7. Quest gates (from tome)
		var gateCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM quest_gates WHERE quest_name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					gateCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "quest_gates.count", 1, gateCount)

		// 8. Quest files (from tome)
		var fileCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM quest_files WHERE quest_name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					fileCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "quest_files.count", 2, fileCount)

		// 9. Respawns updated in fellowship_quests
		var respawns int
		var status string
		sqlitex.Execute(conn,
			`SELECT respawns, status FROM fellowship_quests WHERE name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					respawns = stmt.ColumnInt(0)
					status = stmt.ColumnText(1)
					return nil
				},
			})
		assertEqual(t, "fellowship_quests.respawns", 1, respawns)
		assertEqual(t, "fellowship_quests.status", "active", status)

		// 10. Errands
		var errandCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM errands WHERE quest_name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					errandCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "errands.count", 1, errandCount)

		// 11. Errand deps
		var depCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM errand_deps WHERE quest_name = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					depCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "errand_deps.count", 1, depCount)

		// 12. Herald events
		var heraldCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM herald WHERE quest = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					heraldCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "herald.count", 2, heraldCount)

		// 13. Bulletin entries
		var bulletinCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM bulletin WHERE quest = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					bulletinCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "bulletin.count", 1, bulletinCount)

		// 14. Bulletin files
		var bfCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM bulletin_files`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					bfCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "bulletin_files.count", 1, bfCount)

		// 15. Autopsies
		var autopsyCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM autopsies WHERE quest = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					autopsyCount = stmt.ColumnInt(0)
					return nil
				},
			})
		assertEqual(t, "autopsies.count", 1, autopsyCount)

		// Verify trigger_type mapping (JSON "trigger" -> DB "trigger_type")
		var triggerType string
		sqlitex.Execute(conn,
			`SELECT trigger_type FROM autopsies WHERE quest = 'q1'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					triggerType = stmt.ColumnText(0)
					return nil
				},
			})
		assertEqual(t, "autopsies.trigger_type", "recovery", triggerType)

		// 16. Autopsy files/modules/tags
		var afCount, amCount, atCount int
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM autopsy_files`,
			&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error { afCount = stmt.ColumnInt(0); return nil }})
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM autopsy_modules`,
			&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error { amCount = stmt.ColumnInt(0); return nil }})
		sqlitex.Execute(conn,
			`SELECT COUNT(*) FROM autopsy_tags`,
			&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error { atCount = stmt.ColumnInt(0); return nil }})
		assertEqual(t, "autopsy_files.count", 1, afCount)
		assertEqual(t, "autopsy_modules.count", 1, amCount)
		assertEqual(t, "autopsy_tags.count", 1, atCount)

		return nil
	})

	// Verify backup directory created with correct structure
	backupDir := filepath.Join(mainDataDir, "backup")
	assertFileExists(t, filepath.Join(backupDir, "main", "fellowship-state.json"))
	assertFileExists(t, filepath.Join(backupDir, "main", "bulletin.jsonl"))
	assertFileExists(t, filepath.Join(backupDir, "main", "autopsies", "autopsy-001.json"))
	assertFileExists(t, filepath.Join(backupDir, "worktree-q1", "quest-state.json"))
	assertFileExists(t, filepath.Join(backupDir, "worktree-q1", "quest-tome.json"))
	assertFileExists(t, filepath.Join(backupDir, "worktree-q1", "quest-errands.json"))
	assertFileExists(t, filepath.Join(backupDir, "worktree-q1", "quest-herald.jsonl"))

	// Verify original JSON files deleted
	assertFileNotExists(t, filepath.Join(mainDataDir, "fellowship-state.json"))
	assertFileNotExists(t, filepath.Join(mainDataDir, "bulletin.jsonl"))
	assertFileNotExists(t, filepath.Join(mainDataDir, "autopsies", "autopsy-001.json"))
	assertFileNotExists(t, filepath.Join(wtDataDir, "quest-state.json"))
	assertFileNotExists(t, filepath.Join(wtDataDir, "quest-tome.json"))
	assertFileNotExists(t, filepath.Join(wtDataDir, "quest-errands.json"))
	assertFileNotExists(t, filepath.Join(wtDataDir, "quest-herald.jsonl"))

	// Verify .lock sidecar deleted
	assertFileNotExists(t, filepath.Join(mainDataDir, "fellowship-state.json.lock"))
}

func TestMigrateJSON_NoFiles(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := t.TempDir()
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "worktree "+tmpDir+"\n\n")
	}

	d := OpenTest(t)
	err := MigrateJSON(d, tmpDir)
	if err == nil {
		t.Fatal("expected error for no files, got nil")
	}
	if !strings.Contains(err.Error(), "no JSON files found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertEqual[T comparable](t *testing.T, label string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", label, want, got)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to be deleted: %s", path)
	}
}
