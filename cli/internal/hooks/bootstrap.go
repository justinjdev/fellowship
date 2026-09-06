package hooks

import (
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"zombiezen.com/go/sqlite"
)

// QuestHasHistory reports whether anything has ever been recorded for a quest:
// a gate in its history, or an event in the log.
//
// It is what separates the two states a missing quest_state row can mean. The
// lead has registered this worktree as a quest but the teammate has not run
// `fellowship init` yet — the bootstrap window, where blocking would deadlock
// the quest before it starts — looks exactly like a quest whose row was
// deleted to shake off a pending gate. Nothing else distinguishes them from
// inside the store, so history does: a quest that has submitted a gate or
// logged an event is past its bootstrap, and a row missing under it is a
// destroyed row, not a fresh one.
func QuestHasHistory(conn *sqlite.Conn, questName string) (bool, error) {
	gates, err := history.LoadGates(conn, questName)
	if err != nil {
		return false, err
	}
	if len(gates) > 0 {
		return true, nil
	}
	logged, err := events.Read(conn, questName, 1)
	if err != nil {
		return false, err
	}
	return len(logged) > 0, nil
}
