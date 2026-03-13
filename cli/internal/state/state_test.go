package state_test

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestUpsertAndLoad(t *testing.T) {
	d := db.OpenTest(t)
	s := &state.State{
		QuestName: "quest-auth",
		Phase:     "Research",
	}

	d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := state.Upsert(conn, s); err != nil {
			t.Fatal(err)
		}

		loaded, err := state.Load(conn, "quest-auth")
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Phase != "Research" {
			t.Errorf("expected Research, got %s", loaded.Phase)
		}
		return nil
	})
}

func TestLoad_NotFound(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		_, err := state.Load(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent quest")
		}
		return nil
	})
}

func TestUpsert_Update(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		s := &state.State{QuestName: "q1", Phase: "Onboard"}
		state.Upsert(conn, s)

		s.Phase = "Research"
		s.GatePending = true
		state.Upsert(conn, s)

		loaded, _ := state.Load(conn, "q1")
		if loaded.Phase != "Research" {
			t.Errorf("expected Research, got %s", loaded.Phase)
		}
		if !loaded.GatePending {
			t.Error("expected GatePending true")
		}
		return nil
	})
}

func TestFindQuest(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		sqlitex.Execute(conn, `INSERT INTO fellowship_quests (name, worktree) VALUES ('quest-auth', '/tmp/wt/quest-auth')`, nil)

		name, err := state.FindQuest(conn, "/tmp/wt/quest-auth")
		if err != nil {
			t.Fatal(err)
		}
		if name != "quest-auth" {
			t.Errorf("expected quest-auth, got %s", name)
		}
		return nil
	})
}

func TestBoolIntConversion(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		s := &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: true,
			Held:        true,
		}
		state.Upsert(conn, s)

		loaded, _ := state.Load(conn, "q1")
		if !loaded.GatePending {
			t.Error("GatePending should be true")
		}
		if !loaded.Held {
			t.Error("Held should be true")
		}
		return nil
	})
}

func TestNextPhase(t *testing.T) {
	tests := []struct {
		current string
		want    string
		wantErr bool
	}{
		{"Onboard", "Research", false},
		{"Research", "Plan", false},
		{"Plan", "Implement", false},
		{"Implement", "Adversarial", false},
		{"Adversarial", "Review", false},
		{"Review", "Complete", false},
		{"Complete", "", true},
		{"InvalidPhase", "", true},
	}
	for _, tt := range tests {
		got, err := state.NextPhase(tt.current)
		if (err != nil) != tt.wantErr {
			t.Errorf("NextPhase(%q) error = %v, wantErr %v", tt.current, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("NextPhase(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestIsEarlyPhase(t *testing.T) {
	early := []string{"Onboard", "Research", "Plan"}
	late := []string{"Implement", "Adversarial", "Review", "Complete"}
	for _, p := range early {
		if !state.IsEarlyPhase(p) {
			t.Errorf("IsEarlyPhase(%q) should be true", p)
		}
	}
	for _, p := range late {
		if state.IsEarlyPhase(p) {
			t.Errorf("IsEarlyPhase(%q) should be false", p)
		}
	}
}
