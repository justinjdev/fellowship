package state

import "errors"

// The gate state machine.
//
// Every gate transition in the CLI — the lead's `gate approve` / `gate reject`,
// a company batch approval, the auto-approve path inside the gate-submit hook,
// and the resets performed by `init` and `state clean-worktrees` — mutates the
// same handful of fields in lockstep: gate_pending, gate_id, phase, and the two
// prerequisite flags. Those rules used to be spelled out again at each call
// site, which is how the auto-approve path ended up leaving gate_id behind.
// They live here now, as pure functions over a *State, so the rules have one
// home and can be table-tested without a database.

var (
	// ErrNoGatePending reports an approve/reject on a quest with no gate awaiting a decision.
	ErrNoGatePending = errors.New("no gate pending")
	// ErrGatePending reports a submit while a gate is already awaiting a decision.
	ErrGatePending = errors.New("gate already pending")
	// ErrHeld reports a submit on a quest the lead has put on hold.
	ErrHeld = errors.New("quest is held")
	// ErrNilState reports a transition attempted on a nil state.
	ErrNilState = errors.New("nil quest state")
)

// Transition records the endpoints of an approved gate: the phase the quest
// left and the phase it entered. Callers use it for tome entries and heralds.
type Transition struct {
	Prev string
	Next string
}

// Approve clears a pending gate and advances the quest to the next phase.
//
// It requires a pending gate and a phase that has a successor. On success the
// gate id is cleared (the gate is decided, so nothing references it any more)
// and both prerequisite flags are reset, because lembas and the metadata update
// are per-phase and the quest has just entered a new one. On failure the state
// is left untouched.
func Approve(s *State) (prevPhase, nextPhase string, err error) {
	if s == nil {
		return "", "", ErrNilState
	}
	if !s.GatePending {
		return "", "", ErrNoGatePending
	}
	next, err := NextPhase(s.Phase)
	if err != nil {
		return "", "", err
	}
	prev := s.Phase
	s.GatePending = false
	s.Phase = next
	s.GateID = nil
	s.LembasCompleted = false
	s.MetadataUpdated = false
	return prev, next, nil
}

// Reject clears a pending gate without advancing the phase, unblocking the
// teammate to address the lead's feedback in the phase it is already in. The
// prerequisite flags are deliberately kept: the work that satisfied them was
// not undone by the rejection.
func Reject(s *State) error {
	if s == nil {
		return ErrNilState
	}
	if !s.GatePending {
		return ErrNoGatePending
	}
	s.GatePending = false
	s.GateID = nil
	return nil
}

// Submit marks a gate as awaiting the lead's decision under the given gate id.
//
// It requires that no gate is already pending and that the quest is not held.
// Whether the phase is one a gate may leave (and whether its prerequisites are
// met) is the gate-submit hook's business — Submit only owns the state fields.
func Submit(s *State, gateID string) error {
	if s == nil {
		return ErrNilState
	}
	if s.GatePending {
		return ErrGatePending
	}
	if s.Held {
		return ErrHeld
	}
	if gateID == "" {
		return errors.New("empty gate id")
	}
	s.GatePending = true
	s.GateID = &gateID
	return nil
}

// Reset clears the gate and prerequisite flags, keeping the phase. It is what
// `fellowship init` does to an existing quest and what `state clean-worktrees`
// does to a quest left pending by a worktree that no longer exists. Hold state
// is not a gate flag and is left alone; callers that mean to release a hold
// clear it explicitly.
func Reset(s *State) {
	if s == nil {
		return
	}
	s.GatePending = false
	s.GateID = nil
	s.LembasCompleted = false
	s.MetadataUpdated = false
}
