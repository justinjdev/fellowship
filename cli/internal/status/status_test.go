package status

import (
	"context"
	"testing"

	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

func TestClassifyQuest(t *testing.T) {
	tests := []struct {
		name string
		q    QuestInfo
		want string
	}{
		{
			name: "merged quest is complete",
			q:    QuestInfo{Merged: true},
			want: "complete",
		},
		{
			name: "merged takes priority over checkpoint",
			q:    QuestInfo{Merged: true, HasCheckpoint: true},
			want: "complete",
		},
		{
			name: "checkpoint without merge is resumable",
			q:    QuestInfo{HasCheckpoint: true},
			want: "resumable",
		},
		{
			name: "no merge no checkpoint is stale",
			q:    QuestInfo{},
			want: "stale",
		},
		{
			name: "uncommitted changes alone is stale",
			q:    QuestInfo{HasUncommitted: true},
			want: "stale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyQuest(tt.q)
			if got != tt.want {
				t.Errorf("ClassifyQuest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMergedBranches(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "filters to fellowship prefix only",
			input: "  main\n  fellowship/quest-1\n  feature/other\n  fellowship/quest-2\n",
			want:  []string{"fellowship/quest-1", "fellowship/quest-2"},
		},
		{
			name:  "handles star prefix for current branch",
			input: "* fellowship/quest-active\n  fellowship/quest-done\n  main\n",
			want:  []string{"fellowship/quest-active", "fellowship/quest-done"},
		},
		{
			name:  "empty input returns empty slice",
			input: "",
			want:  []string{},
		},
		{
			name:  "no fellowship branches returns empty slice",
			input: "  main\n  develop\n  feature/foo\n",
			want:  []string{},
		},
		{
			name:  "handles extra whitespace",
			input: "    fellowship/quest-1   \n",
			want:  []string{"fellowship/quest-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMergedBranches(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseMergedBranches() returned %d items, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseMergedBranches()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestScanLoadsFellowshipFromDB(t *testing.T) {
	d := db.OpenTest(t)

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		// Insert fellowship row.
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship (id, version, name, main_repo, base_branch, created_at)
			 VALUES (1, '1', 'test-fellowship', '/tmp/repo', 'main', '2025-01-01T00:00:00Z')`,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		// Scan should pick up fellowship info even with no quests.
		// gitRoot is a fake path — merged branch detection will fail silently.
		result, err := Scan(conn, "/tmp/nonexistent-repo")
		if err != nil {
			t.Fatal(err)
		}

		if result.Fellowship == nil {
			t.Fatal("expected Fellowship to be set")
		}
		if result.Fellowship.Name != "test-fellowship" {
			t.Errorf("Fellowship.Name = %q, want %q", result.Fellowship.Name, "test-fellowship")
		}
		if result.Fellowship.CreatedAt != "2025-01-01T00:00:00Z" {
			t.Errorf("Fellowship.CreatedAt = %q, want %q", result.Fellowship.CreatedAt, "2025-01-01T00:00:00Z")
		}
		if len(result.Quests) != 0 {
			t.Errorf("expected 0 quests, got %d", len(result.Quests))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScanNoFellowship(t *testing.T) {
	d := db.OpenTest(t)

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		result, err := Scan(conn, "/tmp/nonexistent-repo")
		if err != nil {
			t.Fatal(err)
		}

		if result.Fellowship != nil {
			t.Error("expected Fellowship to be nil when no fellowship row exists")
		}
		if len(result.Quests) != 0 {
			t.Errorf("expected 0 quests, got %d", len(result.Quests))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScanQuestsFromDB(t *testing.T) {
	d := db.OpenTest(t)

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		// Insert fellowship.
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship (id, version, name, main_repo, base_branch, created_at)
			 VALUES (1, '1', 'test', '/tmp/repo', 'main', '2025-01-01T00:00:00Z')`,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		// Insert a quest into fellowship_quests.
		err = sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, worktree, branch, task_id, status)
			 VALUES ('quest-1', 'Fix the bug', '/tmp/wt/quest-1', 'fellowship/quest-1', 'task-abc', 'active')`,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		// Insert matching quest_state.
		err = sqlitex.Execute(conn,
			`INSERT INTO quest_state (quest_name, task_id, team_name, phase, gate_pending, created_at, updated_at)
			 VALUES ('quest-1', 'task-abc', 'team-a', 'Implement', 1, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		result, err := Scan(conn, "/tmp/nonexistent-repo")
		if err != nil {
			t.Fatal(err)
		}

		if len(result.Quests) != 1 {
			t.Fatalf("expected 1 quest, got %d", len(result.Quests))
		}

		q := result.Quests[0]
		if q.Name != "quest-1" {
			t.Errorf("Name = %q, want %q", q.Name, "quest-1")
		}
		if q.TaskDescription != "Fix the bug" {
			t.Errorf("TaskDescription = %q, want %q", q.TaskDescription, "Fix the bug")
		}
		if q.Worktree != "/tmp/wt/quest-1" {
			t.Errorf("Worktree = %q, want %q", q.Worktree, "/tmp/wt/quest-1")
		}
		if q.Branch != "fellowship/quest-1" {
			t.Errorf("Branch = %q, want %q", q.Branch, "fellowship/quest-1")
		}
		if q.Phase != "Implement" {
			t.Errorf("Phase = %q, want %q", q.Phase, "Implement")
		}
		if !q.GatePending {
			t.Error("expected GatePending to be true")
		}
		// Classification should be "stale" (no checkpoint file, not merged)
		if q.Classification != "stale" {
			t.Errorf("Classification = %q, want %q", q.Classification, "stale")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScanQuestWithoutState(t *testing.T) {
	d := db.OpenTest(t)

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		// Insert fellowship.
		err := sqlitex.Execute(conn,
			`INSERT INTO fellowship (id, version, name, main_repo, base_branch, created_at)
			 VALUES (1, '1', 'test', '/tmp/repo', 'main', '2025-01-01T00:00:00Z')`,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		// Insert a quest in fellowship_quests but NO quest_state row.
		err = sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, worktree, branch, task_id, status)
			 VALUES ('quest-orphan', 'Orphan task', '/tmp/wt/orphan', 'fellowship/quest-orphan', 'task-xyz', 'active')`,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		result, err := Scan(conn, "/tmp/nonexistent-repo")
		if err != nil {
			t.Fatal(err)
		}

		if len(result.Quests) != 1 {
			t.Fatalf("expected 1 quest, got %d", len(result.Quests))
		}

		q := result.Quests[0]
		if q.Name != "quest-orphan" {
			t.Errorf("Name = %q, want %q", q.Name, "quest-orphan")
		}
		// Phase should be empty string from COALESCE when no quest_state row.
		if q.Phase != "" {
			t.Errorf("Phase = %q, want empty string", q.Phase)
		}
		if q.GatePending {
			t.Error("expected GatePending to be false")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
