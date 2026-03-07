package hook

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quest-hook.json")

	now := time.Now().UTC().Format(time.RFC3339)
	h := &QuestHook{
		Version:   1,
		QuestName: "test-quest",
		Task:      "fix the bug",
		Items: []WorkItem{
			{
				ID:          "w-001",
				Description: "write tests",
				Status:      Pending,
				Phase:       "Implement",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := Save(path, h); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.QuestName != h.QuestName {
		t.Errorf("QuestName = %q, want %q", loaded.QuestName, h.QuestName)
	}
	if loaded.Task != h.Task {
		t.Errorf("Task = %q, want %q", loaded.Task, h.Task)
	}
	if len(loaded.Items) != 1 {
		t.Fatalf("Items count = %d, want 1", len(loaded.Items))
	}
	if loaded.Items[0].ID != "w-001" {
		t.Errorf("Item ID = %q, want %q", loaded.Items[0].ID, "w-001")
	}
	if loaded.Items[0].Status != Pending {
		t.Errorf("Item Status = %q, want %q", loaded.Items[0].Status, Pending)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quest-hook.json")
	os.WriteFile(path, []byte{}, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/quest-hook.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAddItemSequentialIDs(t *testing.T) {
	h := &QuestHook{
		Version:   1,
		QuestName: "test",
		Task:      "task",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	id1 := AddItem(h, "first item", "Implement")
	if id1 != "w-001" {
		t.Errorf("first ID = %q, want %q", id1, "w-001")
	}

	id2 := AddItem(h, "second item", "Implement")
	if id2 != "w-002" {
		t.Errorf("second ID = %q, want %q", id2, "w-002")
	}

	id3 := AddItem(h, "third item", "Review")
	if id3 != "w-003" {
		t.Errorf("third ID = %q, want %q", id3, "w-003")
	}

	if len(h.Items) != 3 {
		t.Errorf("Items count = %d, want 3", len(h.Items))
	}

	if h.Items[0].Phase != "Implement" {
		t.Errorf("Item 0 Phase = %q, want %q", h.Items[0].Phase, "Implement")
	}
	if h.Items[2].Phase != "Review" {
		t.Errorf("Item 2 Phase = %q, want %q", h.Items[2].Phase, "Review")
	}
}

func TestUpdateStatus(t *testing.T) {
	h := &QuestHook{
		Version:   1,
		QuestName: "test",
		Task:      "task",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	AddItem(h, "item one", "Implement")
	AddItem(h, "item two", "Implement")

	if err := UpdateStatus(h, "w-001", Active); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if h.Items[0].Status != Active {
		t.Errorf("Status = %q, want %q", h.Items[0].Status, Active)
	}

	if err := UpdateStatus(h, "w-001", Done); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if h.Items[0].Status != Done {
		t.Errorf("Status = %q, want %q", h.Items[0].Status, Done)
	}

	// Item not found
	err := UpdateStatus(h, "w-999", Done)
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
}

func TestProgress(t *testing.T) {
	h := &QuestHook{
		Version:   1,
		QuestName: "test",
		Task:      "task",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	AddItem(h, "item one", "")
	AddItem(h, "item two", "")
	AddItem(h, "item three", "")

	done, total := Progress(h)
	if done != 0 || total != 3 {
		t.Errorf("Progress = %d/%d, want 0/3", done, total)
	}

	UpdateStatus(h, "w-001", Done)
	done, total = Progress(h)
	if done != 1 || total != 3 {
		t.Errorf("Progress = %d/%d, want 1/3", done, total)
	}

	UpdateStatus(h, "w-002", Done)
	UpdateStatus(h, "w-003", Done)
	done, total = Progress(h)
	if done != 3 || total != 3 {
		t.Errorf("Progress = %d/%d, want 3/3", done, total)
	}
}

func TestPendingItemsDependencyResolution(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	h := &QuestHook{
		Version:   1,
		QuestName: "test",
		Task:      "task",
		Items: []WorkItem{
			{ID: "w-001", Description: "foundation", Status: Pending, CreatedAt: now, UpdatedAt: now},
			{ID: "w-002", Description: "depends on foundation", Status: Pending, DependsOn: []string{"w-001"}, CreatedAt: now, UpdatedAt: now},
			{ID: "w-003", Description: "independent", Status: Pending, CreatedAt: now, UpdatedAt: now},
			{ID: "w-004", Description: "depends on two", Status: Blocked, DependsOn: []string{"w-001", "w-003"}, CreatedAt: now, UpdatedAt: now},
			{ID: "w-005", Description: "already done", Status: Done, CreatedAt: now, UpdatedAt: now},
			{ID: "w-006", Description: "already active", Status: Active, CreatedAt: now, UpdatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Initially: w-001 (no deps), w-003 (no deps) are pending with met deps
	// w-002 depends on w-001 (pending) -> not returned
	// w-004 depends on w-001 (pending) and w-003 (pending) -> not returned
	// w-005 is done, w-006 is active -> not returned
	pending := PendingItems(h)
	if len(pending) != 2 {
		t.Fatalf("PendingItems count = %d, want 2", len(pending))
	}
	ids := map[string]bool{}
	for _, p := range pending {
		ids[p.ID] = true
	}
	if !ids["w-001"] || !ids["w-003"] {
		t.Errorf("expected w-001 and w-003, got %v", ids)
	}

	// Mark w-001 as done -> w-002 should now be available
	h.Items[0].Status = Done
	pending = PendingItems(h)
	pendingIDs := map[string]bool{}
	for _, p := range pending {
		pendingIDs[p.ID] = true
	}
	if !pendingIDs["w-002"] {
		t.Error("w-002 should be pending after w-001 is done")
	}
	if !pendingIDs["w-003"] {
		t.Error("w-003 should still be pending")
	}
	// w-004 depends on w-001 (done) and w-003 (pending) -> still not available
	if pendingIDs["w-004"] {
		t.Error("w-004 should not be pending (w-003 still pending)")
	}

	// Mark w-003 as done -> w-004 should now be available
	h.Items[2].Status = Done
	pending = PendingItems(h)
	pendingIDs = map[string]bool{}
	for _, p := range pending {
		pendingIDs[p.ID] = true
	}
	if !pendingIDs["w-004"] {
		t.Error("w-004 should be pending after all deps are done")
	}
}

func TestNextIDSequence(t *testing.T) {
	h := &QuestHook{
		Version:   1,
		QuestName: "test",
		Task:      "task",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if id := NextID(h); id != "w-001" {
		t.Errorf("NextID empty = %q, want %q", id, "w-001")
	}

	h.Items = append(h.Items, WorkItem{ID: "w-001"})
	if id := NextID(h); id != "w-002" {
		t.Errorf("NextID after 1 = %q, want %q", id, "w-002")
	}

	// Add 8 more to test padding
	for i := 0; i < 8; i++ {
		h.Items = append(h.Items, WorkItem{ID: fmt.Sprintf("w-%03d", i+2)})
	}
	if id := NextID(h); id != "w-010" {
		t.Errorf("NextID after 9 = %q, want %q", id, "w-010")
	}
}

func TestFindHookNoFile(t *testing.T) {
	dir := t.TempDir()
	// Create a tmp dir but no hook file
	os.MkdirAll(filepath.Join(dir, "tmp"), 0755)

	path, err := FindHook(dir)
	if err != nil {
		t.Fatalf("FindHook: %v", err)
	}
	if path != "" {
		t.Errorf("FindHook = %q, want empty", path)
	}
}
