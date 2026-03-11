package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnqueueCommand(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".fellowship"), 0755)

	params, _ := json.Marshal(map[string]string{"task": "test task"})
	cmd, err := EnqueueCommand(dir, ActionSpawnQuest, params)
	if err != nil {
		t.Fatalf("EnqueueCommand failed: %v", err)
	}
	if cmd.ID == "" {
		t.Error("expected non-empty command ID")
	}
	if cmd.Status != StatusPending {
		t.Errorf("expected status pending, got %s", cmd.Status)
	}

	q, err := LoadCommandQueue(dir)
	if err != nil {
		t.Fatalf("LoadCommandQueue failed: %v", err)
	}
	if len(q.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(q.Commands))
	}
}

func TestLoadCommandQueueEmpty(t *testing.T) {
	dir := t.TempDir()
	q, err := LoadCommandQueue(dir)
	if err != nil {
		t.Fatalf("LoadCommandQueue failed: %v", err)
	}
	if len(q.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(q.Commands))
	}
}
