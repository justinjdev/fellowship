package hooks

import (
	"fmt"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// ConfirmPhase records that the teammate confirmed its current phase — one of
// the two prerequisites a gate submission checks. It is what `fellowship phase
// confirm --phase <phase>` runs; it replaced the task-metadata update the
// agent-teams API used to carry.
//
// The phase named has to be the quest's own current phase. A typo, a stale
// value left over from the previous phase, or a name that is not a phase at
// all must not count as "the teammate confirmed this phase" — and the command
// must never be a way to MOVE the phase, so a mismatch is refused, not applied.
//
// Returns whether the prerequisite was recorded, and a line to show the
// teammate when it was not — a silently ignored confirmation is worse than a
// refused one, since the gate then fails later for a reason nothing explained.
func ConfirmPhase(s *state.State, phase string) (recorded bool, notice string) {
	if s == nil || phase == "" {
		return false, "fellowship: no phase given — not recording the phase prerequisite."
	}
	if !state.IsValidPhase(phase) {
		return false, fmt.Sprintf(
			"fellowship: %q is not a quest phase — not recording the phase prerequisite.", phase)
	}
	if phase != s.Phase {
		return false, fmt.Sprintf(
			"fellowship: phase confirm names phase %q but this quest is in %s — not recording the phase prerequisite.", phase, s.Phase)
	}
	s.MetadataUpdated = true
	return true, ""
}
