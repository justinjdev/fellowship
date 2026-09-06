package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// The store is the enforcement state. A session that can hand-edit it can
// rewrite its own phase, clear its own gate, or name itself the lead — so the
// data directory's write exemption stops at the store file.

func TestStoreWriteGuard(t *testing.T) {
	cases := []struct {
		name      string
		input     *HookInput
		wantBlock bool
	}{
		{
			name:      "the store itself",
			input:     &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/.fellowship/fellowship.db"}},
			wantBlock: true,
		},
		{
			name:      "the write-ahead log",
			input:     &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/.fellowship/fellowship.db-wal"}},
			wantBlock: true,
		},
		{
			name:      "the shared-memory file",
			input:     &HookInput{ToolName: "Edit", ToolInput: ToolInput{FilePath: "/repo/.fellowship/fellowship.db-shm"}},
			wantBlock: true,
		},
		{
			name:      "a store under a configured data directory",
			input:     &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/queststate/fellowship.db"}},
			wantBlock: true,
		},
		{
			name:      "a notebook aimed at the store",
			input:     &HookInput{ToolName: "NotebookEdit", ToolInput: ToolInput{NotebookPath: "/repo/.fellowship/fellowship.db"}},
			wantBlock: true,
		},
		{
			name:  "an ordinary coordination file",
			input: &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/.fellowship/notes.md"}},
		},
		{
			name:  "an ordinary source file",
			input: &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/src/main.go"}},
		},
		{
			name:  "a Bash command",
			input: &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "ls"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StoreWriteGuard(c.input); got.Block != c.wantBlock {
				t.Errorf("StoreWriteGuard block = %v, want %v", got.Block, c.wantBlock)
			}
		})
	}
}

// gate-guard exempts the data directory during Research and Plan; the store is
// blocked in every phase all the same.
func TestGateGuard_BlocksStoreWriteInEveryPhase(t *testing.T) {
	for _, phase := range state.Phases() {
		s := &state.State{QuestName: "alpha", Phase: phase}
		input := &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/.fellowship/fellowship.db"}}
		if result := GateGuard(s, input, GuardParams{}); !result.Block {
			t.Errorf("phase %s: a write to the store must be blocked", phase)
		}
	}
}

// The isolation guard's coordination-path exemption stops at the store too: a
// teammate that could write it would simply name itself the lead.
func TestIsolationGuard_StoreIsNotACoordinationPath(t *testing.T) {
	p := IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/.fellowship/fellowship.db",
		DataDirName:      ".fellowship",
		SessionID:        "teammate",
		LeadSessionID:    "lead",
	}
	if result := IsolationGuard(p); !result.Block {
		t.Error("a teammate writing the store in the main tree must be blocked")
	}

	p.FilePath = "/repo/.fellowship/notes.md"
	if result := IsolationGuard(p); result.Block {
		t.Errorf("an ordinary coordination write must still be allowed: %s", result.Message)
	}
}
