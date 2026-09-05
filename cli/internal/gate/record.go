// Package gate records the journal side of a gate decision: the history
// entries and events that accompany a state transition made by
// state.Approve/Reject/Submit.
//
// The state machine lives in the state package and owns the quest's fields;
// this package owns what gets written down about a transition. Keeping them
// apart means state stays free of the history and events dependencies
// (history imports state), while every approval path — the lead's `gate
// approve`, a group batch approval, and the gate-submit hook's auto-approve —
// records the same three things instead of each remembering its own subset.
package gate

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"

	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/history"
)

// RecordApproval writes the history and event records for a gate that has
// just been approved, moving the quest from prev to next. detail is the
// reason recorded in the history ("" for an ordinary approval); the events
// always describe the approval and the phase transition.
func RecordApproval(conn *sqlite.Conn, questName, prev, next, detail string) error {
	if err := history.RecordGate(conn, questName, prev, "approved", detail); err != nil {
		return fmt.Errorf("recording gate approval for %s: %w", questName, err)
	}
	if err := history.RecordPhase(conn, questName, prev, 0); err != nil {
		return fmt.Errorf("recording phase %s for %s: %w", prev, questName, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := events.Record(conn, events.Event{
		Timestamp: now, Quest: questName, Type: events.GateApproved,
		Phase: prev, Detail: fmt.Sprintf("Gate approved for %s", prev),
	}); err != nil {
		return fmt.Errorf("announcing gate approval for %s: %w", questName, err)
	}
	if err := events.Record(conn, events.Event{
		Timestamp: now, Quest: questName, Type: events.PhaseTransition,
		Phase: next, Detail: fmt.Sprintf("Phase advanced from %s to %s", prev, next),
	}); err != nil {
		return fmt.Errorf("announcing phase transition for %s: %w", questName, err)
	}
	return nil
}

// RecordRejection writes the history entry and event for a rejected gate.
// The quest stays in phase, so there is no phase record to write.
func RecordRejection(conn *sqlite.Conn, questName, phase, detail string) error {
	if err := history.RecordGate(conn, questName, phase, "rejected", detail); err != nil {
		return fmt.Errorf("recording gate rejection for %s: %w", questName, err)
	}
	if err := events.Record(conn, events.Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Quest:     questName, Type: events.GateRejected,
		Phase: phase, Detail: fmt.Sprintf("Gate rejected for %s", phase),
	}); err != nil {
		return fmt.Errorf("announcing gate rejection for %s: %w", questName, err)
	}
	return nil
}
