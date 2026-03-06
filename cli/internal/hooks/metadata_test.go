package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestMetadataTrack_SetsFlag(t *testing.T) {
	s := &state.State{MetadataUpdated: false}
	input := &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{Phase: "Research"}}}
	MetadataTrack(s, input)
	if !s.MetadataUpdated {
		t.Error("should set metadata_updated")
	}
}

func TestMetadataTrack_IgnoresNonPhaseMetadata(t *testing.T) {
	s := &state.State{MetadataUpdated: false}
	input := &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{}}}
	MetadataTrack(s, input)
	if s.MetadataUpdated {
		t.Error("should not set metadata_updated without phase")
	}
}

func TestMetadataTrack_IgnoresNoMetadata(t *testing.T) {
	s := &state.State{MetadataUpdated: false}
	input := &HookInput{ToolInput: ToolInput{Status: "in_progress"}}
	MetadataTrack(s, input)
	if s.MetadataUpdated {
		t.Error("should not set metadata_updated without metadata")
	}
}
