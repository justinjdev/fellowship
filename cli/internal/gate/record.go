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
	"zombiezen.com/go/sqlite/sqlitex"

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
	nowTime := time.Now().UTC()
	duration := phaseDurationSeconds(conn, questName, nowTime)
	if err := history.RecordPhase(conn, questName, prev, duration); err != nil {
		return fmt.Errorf("recording phase %s for %s: %w", prev, questName, err)
	}
	now := nowTime.Format(time.RFC3339)
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

// phaseDurationSeconds reports how long the quest spent in the phase it is
// leaving, measured from the previous phase's completed_at, or from the
// quest's creation if this is its first phase completion. It returns 0
// rather than fail the approval when a starting point can't be determined
// (no history yet and no readable created_at) or comes back after now.
func phaseDurationSeconds(conn *sqlite.Conn, questName string, now time.Time) int {
	start, err := phaseStartTime(conn, questName)
	if err != nil {
		return 0
	}
	d := now.Sub(start)
	if d < 0 {
		return 0
	}
	return int(d.Seconds())
}

// phaseStartTime returns when the quest entered the phase it is now
// leaving: the most recent quest_phases.completed_at, or quest_state.created_at
// if no phase has completed yet. state.State doesn't expose created_at (no
// caller needed it before this one), so it's read directly here rather than
// growing that struct for a single field one package uses.
func phaseStartTime(conn *sqlite.Conn, questName string) (time.Time, error) {
	phases, err := history.LoadPhases(conn, questName)
	if err != nil {
		return time.Time{}, err
	}
	if len(phases) > 0 {
		return time.Parse(time.RFC3339, phases[len(phases)-1].CompletedAt)
	}

	var createdAt string
	if err := sqlitex.Execute(conn,
		`SELECT created_at FROM quest_state WHERE quest_name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				createdAt = stmt.ColumnText(0)
				return nil
			},
		}); err != nil {
		return time.Time{}, err
	}
	if createdAt == "" {
		return time.Time{}, fmt.Errorf("gate: no created_at for quest %s", questName)
	}
	return time.Parse(time.RFC3339, createdAt)
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
