package dashboard

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestInitAndLoadFellowship(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		err := InitFellowship(conn, "test-fellowship", "/tmp/repo", "main")
		if err != nil {
			t.Fatal(err)
		}
		fs, err := LoadFellowship(conn)
		if err != nil {
			t.Fatal(err)
		}
		if fs.Name != "test-fellowship" {
			t.Errorf("Name = %q, want %q", fs.Name, "test-fellowship")
		}
		if fs.MainRepo != "/tmp/repo" {
			t.Errorf("MainRepo = %q, want %q", fs.MainRepo, "/tmp/repo")
		}
		if fs.BaseBranch != "main" {
			t.Errorf("BaseBranch = %q, want %q", fs.BaseBranch, "main")
		}
		return nil
	})
}

func TestAddQuest(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "f1", "/tmp", "main")
		AddQuest(conn, QuestEntry{
			Name: "q1", TaskDescription: "build auth", Worktree: "/tmp/wt/q1", Branch: "feat/q1",
		})
		quests, _ := ListQuests(conn)
		if len(quests) != 1 {
			t.Fatalf("expected 1, got %d", len(quests))
		}
		if quests[0].Name != "q1" {
			t.Errorf("Name = %q, want %q", quests[0].Name, "q1")
		}
		if quests[0].TaskDescription != "build auth" {
			t.Errorf("TaskDescription = %q, want %q", quests[0].TaskDescription, "build auth")
		}
		return nil
	})
}

func TestAddAndRemoveScout(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "f1", "/tmp", "main")
		AddScout(conn, ScoutEntry{Name: "s1", Question: "how?", TaskID: "t1"})
		scouts, _ := ListScouts(conn)
		if len(scouts) != 1 {
			t.Fatalf("expected 1 scout, got %d", len(scouts))
		}
		if scouts[0].Name != "s1" {
			t.Errorf("Name = %q, want %q", scouts[0].Name, "s1")
		}

		RemoveScout(conn, "s1")
		scouts, _ = ListScouts(conn)
		if len(scouts) != 0 {
			t.Errorf("expected 0 scouts after remove, got %d", len(scouts))
		}
		return nil
	})
}

func TestAddCompany(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "f1", "/tmp", "main")
		AddQuest(conn, QuestEntry{Name: "q1", Worktree: "/tmp/wt/q1"})
		AddScout(conn, ScoutEntry{Name: "s1", Question: "why?"})
		AddCompany(conn, "team-alpha", []string{"q1"}, []string{"s1"})

		companies, _ := ListCompanies(conn)
		if len(companies) != 1 {
			t.Fatalf("expected 1 company, got %d", len(companies))
		}
		if companies[0].Name != "team-alpha" {
			t.Errorf("Name = %q, want %q", companies[0].Name, "team-alpha")
		}
		if len(companies[0].Quests) != 1 || companies[0].Quests[0] != "q1" {
			t.Errorf("Quests = %v, want [q1]", companies[0].Quests)
		}
		if len(companies[0].Scouts) != 1 || companies[0].Scouts[0] != "s1" {
			t.Errorf("Scouts = %v, want [s1]", companies[0].Scouts)
		}
		return nil
	})
}

func TestUpdateQuest(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "f1", "/tmp", "main")
		AddQuest(conn, QuestEntry{Name: "q1", Worktree: "/tmp/wt/q1", Status: "active"})
		UpdateQuest(conn, "q1", map[string]any{"status": "completed"})

		quests, _ := ListQuests(conn)
		if len(quests) != 1 {
			t.Fatalf("expected 1, got %d", len(quests))
		}
		if quests[0].Status != "completed" {
			t.Errorf("Status = %q, want %q", quests[0].Status, "completed")
		}
		return nil
	})
}

func TestRemoveQuest(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "f1", "/tmp", "main")
		AddQuest(conn, QuestEntry{Name: "q1", Worktree: "/tmp/wt/q1"})
		RemoveQuest(conn, "q1")
		quests, _ := ListQuests(conn)
		if len(quests) != 0 {
			t.Errorf("expected 0 quests after remove, got %d", len(quests))
		}
		return nil
	})
}

func TestSaveFellowship_RoundTrip(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "test-fellowship", "/path/to/repo", "main")

		original := &FellowshipState{
			Version:    1,
			Name:       "test-fellowship",
			CreatedAt:  "2025-01-15T10:30:00Z",
			MainRepo:   "/path/to/repo",
			BaseBranch: "main",
			Quests: []QuestEntry{
				{Name: "quest-1", TaskDescription: "do stuff", Worktree: "/tmp/wt", Branch: "fellowship/quest-1", TaskID: "t1"},
			},
			Scouts: []ScoutEntry{
				{Name: "scout-1", Question: "how does X work?", TaskID: "t2"},
			},
			Companies: []CompanyEntry{
				{Name: "company-1", Quests: []string{"quest-1"}, Scouts: []string{"scout-1"}},
			},
		}

		if err := SaveFellowship(conn, original); err != nil {
			t.Fatalf("SaveFellowship() error: %v", err)
		}

		loaded, err := LoadFellowship(conn)
		if err != nil {
			t.Fatalf("LoadFellowship() error: %v", err)
		}

		if loaded.Name != original.Name {
			t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
		}
		if loaded.MainRepo != original.MainRepo {
			t.Errorf("MainRepo = %q, want %q", loaded.MainRepo, original.MainRepo)
		}
		if len(loaded.Quests) != 1 {
			t.Fatalf("len(Quests) = %d, want 1", len(loaded.Quests))
		}
		if loaded.Quests[0].TaskDescription != "do stuff" {
			t.Errorf("Quests[0].TaskDescription = %q, want %q", loaded.Quests[0].TaskDescription, "do stuff")
		}
		if loaded.Quests[0].Branch != "fellowship/quest-1" {
			t.Errorf("Quests[0].Branch = %q, want %q", loaded.Quests[0].Branch, "fellowship/quest-1")
		}
		if len(loaded.Scouts) != 1 {
			t.Fatalf("len(Scouts) = %d, want 1", len(loaded.Scouts))
		}
		if loaded.Scouts[0].Question != "how does X work?" {
			t.Errorf("Scouts[0].Question = %q, want %q", loaded.Scouts[0].Question, "how does X work?")
		}
		if len(loaded.Companies) != 1 {
			t.Fatalf("len(Companies) = %d, want 1", len(loaded.Companies))
		}
		if loaded.Companies[0].Name != "company-1" {
			t.Errorf("Companies[0].Name = %q, want %q", loaded.Companies[0].Name, "company-1")
		}
		return nil
	})
}

func TestLoadFellowship_NotInitialized(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		_, err := LoadFellowship(conn)
		if err == nil {
			t.Fatal("expected error for uninitialized fellowship, got nil")
		}
		return nil
	})
}

func TestQuestEntryStatus_Default(t *testing.T) {
	q := QuestEntry{Name: "test"}
	if got := QuestEntryStatus(q); got != "active" {
		t.Errorf("QuestEntryStatus() = %q, want %q", got, "active")
	}
}

func TestQuestEntryStatus_Explicit(t *testing.T) {
	for _, status := range []string{"active", "completed", "cancelled"} {
		q := QuestEntry{Name: "test", Status: status}
		if got := QuestEntryStatus(q); got != status {
			t.Errorf("QuestEntryStatus(%q) = %q, want %q", status, got, status)
		}
	}
}

func TestDiscoverQuests_NoFellowship(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		status, err := DiscoverQuests(conn)
		if err != nil {
			t.Fatalf("DiscoverQuests() error: %v", err)
		}
		if len(status.Quests) != 0 {
			t.Errorf("expected 0 quests, got %d", len(status.Quests))
		}
		return nil
	})
}

func TestDiscoverQuests_WithQuestState(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "test-fellowship", "/tmp/repo", "main")
		AddQuest(conn, QuestEntry{
			Name: "quest-auth", Worktree: "/tmp/wt/quest-auth", Branch: "feat/auth",
		})

		// Insert quest_state row
		state.Upsert(conn, &state.State{
			QuestName: "quest-auth",
			Phase:     "Implement",
		})

		status, err := DiscoverQuests(conn)
		if err != nil {
			t.Fatalf("DiscoverQuests() error: %v", err)
		}
		if status.Name != "test-fellowship" {
			t.Errorf("Name = %q, want %q", status.Name, "test-fellowship")
		}
		if len(status.Quests) != 1 {
			t.Fatalf("len(Quests) = %d, want 1", len(status.Quests))
		}
		q := status.Quests[0]
		if q.Name != "quest-auth" {
			t.Errorf("Quest.Name = %q, want %q", q.Name, "quest-auth")
		}
		if q.Phase != "Implement" {
			t.Errorf("Quest.Phase = %q, want %q", q.Phase, "Implement")
		}
		if q.Worktree != "/tmp/wt/quest-auth" {
			t.Errorf("Quest.Worktree = %q, want %q", q.Worktree, "/tmp/wt/quest-auth")
		}
		return nil
	})
}

func TestDiscoverQuests_CompletedNoQuestState(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "test-fellowship", "/tmp/repo", "main")
		AddQuest(conn, QuestEntry{
			Name: "quest-done", Worktree: "/tmp/wt/done", Status: "completed",
		})

		// No quest_state row — should appear as synthetic Complete entry
		status, err := DiscoverQuests(conn)
		if err != nil {
			t.Fatalf("DiscoverQuests() error: %v", err)
		}
		if len(status.Quests) != 1 {
			t.Fatalf("len(Quests) = %d, want 1", len(status.Quests))
		}
		q := status.Quests[0]
		if q.Phase != "Complete" {
			t.Errorf("Quest.Phase = %q, want %q", q.Phase, "Complete")
		}
		if q.Status != "completed" {
			t.Errorf("Quest.Status = %q, want %q", q.Status, "completed")
		}
		return nil
	})
}

func TestDiscoverQuests_ActiveNoQuestStateSkipped(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		InitFellowship(conn, "test-fellowship", "/tmp/repo", "main")
		AddQuest(conn, QuestEntry{
			Name: "quest-active", Worktree: "/tmp/wt/active",
		})

		// No quest_state row, active status — should be skipped
		status, err := DiscoverQuests(conn)
		if err != nil {
			t.Fatalf("DiscoverQuests() error: %v", err)
		}
		if len(status.Quests) != 0 {
			t.Errorf("expected 0 quests (active with no quest_state should be skipped), got %d", len(status.Quests))
		}
		return nil
	})
}
