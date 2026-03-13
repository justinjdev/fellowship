package autopsy

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestCreateAndScan(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		id, err := Create(conn, &CreateInput{
			Quest: "q1", Phase: "Implement", Trigger: "recovery",
			Files: []string{"auth.go"}, Modules: []string{"auth"},
			WhatFailed: "tests failed", Tags: []string{"flaky"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		matches, err := Scan(conn, ScanOptions{Files: []string{"auth.go"}}, 90)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1, got %d", len(matches))
		}
		return nil
	})
}

func TestCreate_ValidInput(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		id, err := Create(conn, &CreateInput{
			Quest:      "quest-1",
			Task:       "Add auth endpoint",
			Phase:      "Implement",
			Trigger:    "recovery",
			Files:      []string{"src/auth/jwt.go"},
			Modules:    []string{"auth"},
			WhatFailed: "Middleware caches tokens",
			Resolution: "Added cache invalidation",
			Tags:       []string{"caching"},
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		// Verify we can scan it back
		matches, err := Scan(conn, ScanOptions{Tags: []string{"caching"}}, 90)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		a := matches[0]
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
		if len(a.Files) != 1 || a.Files[0] != "src/auth/jwt.go" {
			t.Errorf("files = %v, want [src/auth/jwt.go]", a.Files)
		}
		if len(a.Modules) != 1 || a.Modules[0] != "auth" {
			t.Errorf("modules = %v, want [auth]", a.Modules)
		}
		return nil
	})
}

func TestCreate_MissingQuest(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Trigger:    "recovery",
			WhatFailed: "something",
		})
		if err == nil {
			t.Error("expected error for missing quest")
		}
		return nil
	})
}

func TestCreate_InvalidTrigger(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Quest:      "quest-1",
			Trigger:    "invalid",
			WhatFailed: "something",
		})
		if err == nil {
			t.Error("expected error for invalid trigger")
		}
		return nil
	})
}

func TestCreate_MissingWhatFailed(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Quest:   "quest-1",
			Trigger: "recovery",
		})
		if err == nil {
			t.Error("expected error for missing what_failed")
		}
		return nil
	})
}

func TestCreate_NilInput(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
		return nil
	})
}

func TestScan_MatchByFile(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Quest:      "quest-1",
			Trigger:    "recovery",
			Files:      []string{"src/auth/jwt.go"},
			WhatFailed: "auth issue",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Same directory should match
		matches, err := Scan(conn, ScanOptions{Files: []string{"src/auth/middleware.go"}}, 90)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(matches) != 1 {
			t.Errorf("expected 1 match (same directory), got %d", len(matches))
		}
		return nil
	})
}

func TestScan_MatchByModule(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Quest:      "quest-1",
			Trigger:    "recovery",
			Modules:    []string{"auth"},
			WhatFailed: "auth issue",
		})
		if err != nil {
			t.Fatal(err)
		}

		matches, err := Scan(conn, ScanOptions{Modules: []string{"auth"}}, 90)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(matches) != 1 {
			t.Errorf("expected 1 match, got %d", len(matches))
		}
		return nil
	})
}

func TestScan_MatchByTag(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Quest:      "quest-1",
			Trigger:    "recovery",
			Tags:       []string{"caching", "auth"},
			WhatFailed: "cache issue",
		})
		if err != nil {
			t.Fatal(err)
		}

		matches, err := Scan(conn, ScanOptions{Tags: []string{"caching"}}, 90)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(matches) != 1 {
			t.Errorf("expected 1 match, got %d", len(matches))
		}
		return nil
	})
}

func TestScan_NoMatch(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Create(conn, &CreateInput{
			Quest:      "quest-1",
			Trigger:    "recovery",
			Modules:    []string{"auth"},
			WhatFailed: "auth issue",
		})
		if err != nil {
			t.Fatal(err)
		}

		matches, err := Scan(conn, ScanOptions{Modules: []string{"billing"}}, 90)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(matches) != 0 {
			t.Errorf("expected 0 matches, got %d", len(matches))
		}
		return nil
	})
}

func TestScan_RequiresFilter(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Scan(conn, ScanOptions{}, 90)
		if err == nil {
			t.Error("expected error when no filters provided")
		}
		return nil
	})
}

func TestScan_ExcludesExpired(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Insert an autopsy with an already-expired expires_at
		err := sqlitex.Execute(conn,
			`INSERT INTO autopsies (timestamp, quest, trigger_type, what_failed, expires_at)
			 VALUES (datetime('now', '-100 days'), 'old-quest', 'recovery', 'old failure', datetime('now', '-10 days'))`,
			nil)
		if err != nil {
			t.Fatal(err)
		}
		oldID := conn.LastInsertRowID()

		// Add a module so we can search for it
		err = sqlitex.Execute(conn,
			`INSERT INTO autopsy_modules (autopsy_id, module) VALUES (?, 'auth')`,
			&sqlitex.ExecOptions{Args: []any{oldID}})
		if err != nil {
			t.Fatal(err)
		}

		matches, err := Scan(conn, ScanOptions{Modules: []string{"auth"}}, 90)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(matches) != 0 {
			t.Errorf("expired autopsy should be excluded, got %d matches", len(matches))
		}
		return nil
	})
}

func TestInfer_FromRespawns(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Set up fellowship_quests row
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, status, respawns)
			 VALUES ('quest-respawned', 'Fix login flow', 'active', 2)`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Set up quest_state (needed for FK in quest_phases/quest_files)
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_state (quest_name, created_at, updated_at)
			 VALUES ('quest-respawned', datetime('now'), datetime('now'))`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Add phase history
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_phases (quest_name, phase, completed_at)
			 VALUES ('quest-respawned', 'Implement', datetime('now'))`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Add files touched
		for _, f := range []string{"src/auth/login.go", "src/auth/session.go"} {
			err = sqlitex.Execute(conn,
				`INSERT INTO quest_files (quest_name, file_path) VALUES ('quest-respawned', ?)`,
				&sqlitex.ExecOptions{Args: []any{f}})
			if err != nil {
				t.Fatal(err)
			}
		}

		id, err := Infer(conn, "quest-respawned")
		if err != nil {
			t.Fatalf("Infer failed: %v", err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		// Verify the autopsy
		matches, err := Scan(conn, ScanOptions{Files: []string{"src/auth/login.go"}}, 90)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		a := matches[0]
		if a.Trigger != "recovery" {
			t.Errorf("trigger = %q, want recovery", a.Trigger)
		}
		if a.Quest != "quest-respawned" {
			t.Errorf("quest = %q", a.Quest)
		}
		if len(a.Files) != 2 {
			t.Errorf("files = %v, want 2 files", a.Files)
		}
		return nil
	})
}

func TestInfer_FromRejection(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Set up fellowship_quests
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, status, respawns)
			 VALUES ('quest-rejected', 'Add billing', 'active', 0)`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Set up quest_state (for FK)
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_state (quest_name, created_at, updated_at)
			 VALUES ('quest-rejected', datetime('now'), datetime('now'))`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Add gate rejection
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_gates (quest_name, phase, action, timestamp, reason)
			 VALUES ('quest-rejected', 'Plan', 'rejected', datetime('now'), 'Plan doesn''t account for tax calculation')`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Add phase
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_phases (quest_name, phase, completed_at)
			 VALUES ('quest-rejected', 'Plan', datetime('now'))`, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Add files
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_files (quest_name, file_path) VALUES ('quest-rejected', 'src/billing/charge.go')`, nil)
		if err != nil {
			t.Fatal(err)
		}

		id, err := Infer(conn, "quest-rejected")
		if err != nil {
			t.Fatalf("Infer failed: %v", err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		matches, err := Scan(conn, ScanOptions{Files: []string{"src/billing/charge.go"}}, 90)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].Trigger != "rejection" {
			t.Errorf("trigger = %q, want rejection", matches[0].Trigger)
		}
		if matches[0].WhatFailed != "Plan doesn't account for tax calculation" {
			t.Errorf("what_failed = %q", matches[0].WhatFailed)
		}
		return nil
	})
}

func TestInfer_FromAbandonment(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, status, respawns)
			 VALUES ('quest-abandoned', 'Migrate DB', 'cancelled', 0)`, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = sqlitex.Execute(conn,
			`INSERT INTO quest_state (quest_name, created_at, updated_at)
			 VALUES ('quest-abandoned', datetime('now'), datetime('now'))`, nil)
		if err != nil {
			t.Fatal(err)
		}

		id, err := Infer(conn, "quest-abandoned")
		if err != nil {
			t.Fatalf("Infer failed: %v", err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		matches, err := Scan(conn, ScanOptions{Modules: []string{"quest-abandoned"}}, 90)
		if err != nil {
			t.Fatal(err)
		}
		// No files means no modules, so search by the quest name directly
		// Actually, let's just verify via a tag/module-less scan won't work;
		// instead query directly
		_ = matches

		// Verify the autopsy was created by looking at the DB directly
		var trigger string
		sqlitex.Execute(conn,
			`SELECT trigger_type FROM autopsies WHERE id = ?`,
			&sqlitex.ExecOptions{
				Args: []any{id},
				ResultFunc: func(stmt *sqlite.Stmt) error {
					trigger = stmt.ColumnText(0)
					return nil
				},
			})
		if trigger != "abandonment" {
			t.Errorf("trigger = %q, want abandonment", trigger)
		}
		return nil
	})
}

func TestInfer_NoFailureSignals(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, status, respawns)
			 VALUES ('quest-ok', 'Add feature', 'active', 0)`, nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Infer(conn, "quest-ok")
		if err == nil {
			t.Error("expected error when no failure signals found")
		}
		return nil
	})
}

func TestInfer_QuestNotFound(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		_, err := Infer(conn, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent quest")
		}
		return nil
	})
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
