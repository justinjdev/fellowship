package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestMetadataTrack(t *testing.T) {
	cases := []struct {
		name         string
		phase        string // the quest's current phase
		input        *HookInput
		wantRecorded bool
		wantNotice   bool
	}{
		{
			name:         "the quest's own phase",
			phase:        "Research",
			input:        &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{Phase: "Research"}}},
			wantRecorded: true,
		},
		{
			name:       "a phase the quest is not in",
			phase:      "Research",
			input:      &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{Phase: "Implement"}}},
			wantNotice: true,
		},
		{
			name:       "a name that is not a phase at all",
			phase:      "Research",
			input:      &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{Phase: "Onboard"}}},
			wantNotice: true,
		},
		{
			name:  "metadata with no phase",
			phase: "Research",
			input: &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{}}},
		},
		{
			name:  "no metadata",
			phase: "Research",
			input: &HookInput{ToolInput: ToolInput{Status: "in_progress"}},
		},
		{
			name:  "no input",
			phase: "Research",
			input: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state.State{Phase: c.phase}
			recorded, notice := MetadataTrack(s, c.input)
			if recorded != c.wantRecorded {
				t.Errorf("recorded = %v, want %v", recorded, c.wantRecorded)
			}
			if s.MetadataUpdated != c.wantRecorded {
				t.Errorf("MetadataUpdated = %v, want %v", s.MetadataUpdated, c.wantRecorded)
			}
			if (notice != "") != c.wantNotice {
				t.Errorf("notice = %q, want notice: %v", notice, c.wantNotice)
			}
		})
	}
}
