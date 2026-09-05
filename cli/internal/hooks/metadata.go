package hooks

import (
	"fmt"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// MetadataTrack records that the task's phase metadata was updated — one of the
// two prerequisites a gate submission checks.
//
// The phase in the payload has to be the quest's own current phase. It used to
// be enough for the field to be non-empty, so `TaskUpdate(metadata: {phase:
// "anything"})` satisfied the prerequisite: a typo, a stale value left over
// from the previous phase, or a name that is not a phase at all all counted as
// "the teammate updated its metadata for this phase".
//
// Returns whether the prerequisite was recorded, and a line to show the
// teammate when it was not — a silently ignored update is worse than a refused
// one, since the gate then fails later for a reason nothing explained.
func MetadataTrack(s *state.State, input *HookInput) (recorded bool, notice string) {
	if input == nil || input.ToolInput.Metadata == nil {
		return false, ""
	}
	phase := input.ToolInput.Metadata.Phase
	if phase == "" {
		return false, ""
	}
	if !state.IsValidPhase(phase) {
		return false, fmt.Sprintf(
			"fellowship: task metadata names %q, which is not a quest phase — not recording the metadata prerequisite.", phase)
	}
	if phase != s.Phase {
		return false, fmt.Sprintf(
			"fellowship: task metadata names phase %q but this quest is in %s — not recording the metadata prerequisite.", phase, s.Phase)
	}
	s.MetadataUpdated = true
	return true, ""
}
