package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// ErrNotFound is returned when a quest does not exist in the database.
var ErrNotFound = errors.New("state: quest not found")

type State struct {
	QuestName        string   `json:"quest_name"`
	TaskID           string   `json:"task_id"`
	TeamName         string   `json:"team_name"`
	Phase            string   `json:"phase"`
	GatePending      bool     `json:"gate_pending"`
	GateID           *string  `json:"gate_id"`
	LembasCompleted  bool     `json:"lembas_completed"`
	MetadataUpdated  bool     `json:"metadata_updated"`
	AutoApproveGates []string `json:"auto_approve_gates"`
	Held             bool     `json:"held"`
	HeldReason       *string  `json:"held_reason"`
}

// phaseOrder is the canonical quest lifecycle: four phases with a gate
// leaving each of the first three. Review is terminal — no gate leaves it,
// and the quest ends when the PR is opened and the task is marked complete
// (enforced by the completion-guard hook).
var phaseOrder = []string{"Research", "Plan", "Implement", "Review"}

// TerminalPhase is the last phase in the lifecycle. No gate leaves it.
const TerminalPhase = "Review"

// GatePhases returns the phases a gate can leave — every phase but the
// terminal one. These are the valid entries for gates.autoApprove.
func GatePhases() []string {
	gated := phaseOrder[:len(phaseOrder)-1]
	out := make([]string, len(gated))
	copy(out, gated)
	return out
}

func NextPhase(current string) (string, error) {
	for i, p := range phaseOrder {
		if p == current {
			if i+1 >= len(phaseOrder) {
				return "", fmt.Errorf("no phase after %s", current)
			}
			return phaseOrder[i+1], nil
		}
	}
	return "", fmt.Errorf("unknown phase: %s", current)
}

// IsEarlyPhase reports whether a phase forbids source writes. Research and
// Plan are read-and-think phases; Implement and Review write code.
func IsEarlyPhase(phase string) bool {
	return phase == "Research" || phase == "Plan"
}

// Phases returns the ordered quest phase names.
func Phases() []string {
	out := make([]string, len(phaseOrder))
	copy(out, phaseOrder)
	return out
}

// IsValidPhase reports whether p is a known quest phase.
func IsValidPhase(p string) bool {
	for _, phase := range phaseOrder {
		if phase == p {
			return true
		}
	}
	return false
}

// Load reads quest state from DB by quest name.
func Load(conn *sqlite.Conn, questName string) (*State, error) {
	var s State
	var found bool
	err := sqlitex.Execute(conn, `SELECT quest_name, task_id, team_name, phase,
		gate_pending, gate_id, lembas_completed, metadata_updated,
		held, held_reason, auto_approve, created_at, updated_at
		FROM quest_state WHERE quest_name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				s.QuestName = stmt.ColumnText(0)
				s.TaskID = stmt.ColumnText(1)
				s.TeamName = stmt.ColumnText(2)
				s.Phase = stmt.ColumnText(3)
				s.GatePending = stmt.ColumnInt(4) != 0
				if stmt.ColumnType(5) != sqlite.TypeNull {
					gid := stmt.ColumnText(5)
					s.GateID = &gid
				}
				s.LembasCompleted = stmt.ColumnInt(6) != 0
				s.MetadataUpdated = stmt.ColumnInt(7) != 0
				s.Held = stmt.ColumnInt(8) != 0
				if stmt.ColumnType(9) != sqlite.TypeNull {
					hr := stmt.ColumnText(9)
					s.HeldReason = &hr
				}
				if aa := stmt.ColumnText(10); aa != "" {
					if err := json.Unmarshal([]byte(aa), &s.AutoApproveGates); err != nil {
						return fmt.Errorf("unmarshal auto_approve: %w", err)
					}
				}
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("state: load %s: %w", questName, err)
	}
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, questName)
	}
	return &s, nil
}

// Upsert inserts or updates quest state.
func Upsert(conn *sqlite.Conn, s *State) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var autoApprove string
	if len(s.AutoApproveGates) > 0 {
		b, _ := json.Marshal(s.AutoApproveGates)
		autoApprove = string(b)
	}

	return sqlitex.Execute(conn, `INSERT INTO quest_state
		(quest_name, task_id, team_name, phase, gate_pending, gate_id,
		 lembas_completed, metadata_updated, held, held_reason, auto_approve,
		 created_at, updated_at)
		VALUES (:name, :task_id, :team, :phase, :gate_pending, :gate_id,
		 :lembas, :metadata, :held, :held_reason, :auto_approve, :now, :now)
		ON CONFLICT(quest_name) DO UPDATE SET
		 task_id=:task_id, team_name=:team, phase=:phase,
		 gate_pending=:gate_pending, gate_id=:gate_id,
		 lembas_completed=:lembas, metadata_updated=:metadata,
		 held=:held, held_reason=:held_reason, auto_approve=:auto_approve,
		 updated_at=:now`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":name":         s.QuestName,
				":task_id":      s.TaskID,
				":team":         s.TeamName,
				":phase":        s.Phase,
				":gate_pending": boolToInt(s.GatePending),
				":gate_id":      ptrToAny(s.GateID),
				":lembas":       boolToInt(s.LembasCompleted),
				":metadata":     boolToInt(s.MetadataUpdated),
				":held":         boolToInt(s.Held),
				":held_reason":  ptrToAny(s.HeldReason),
				":auto_approve": autoApprove,
				":now":          now,
			},
		})
}

// FindQuest returns the quest name registered for a given worktree root path.
//
// Matching is done on canonicalized paths. Quest rows are canonicalized on
// write, but a row registered by an older version (or by hand) may hold a
// relative or symlinked path, while hooks always look up a resolved git
// top-level — a raw string comparison silently misses those and the quest
// reads as "no quest here", which is exactly the state that used to be
// mistaken for a lead session. The raw stored value is still accepted as a
// fallback so a path that cannot be resolved at all still matches itself.
//
// fellowship_quests.worktree carries a UNIQUE index (schema v2): at most one
// row can hold a given worktree, so re-registering a worktree under a
// different quest (fellowship.upsertQuest) clears it from whichever row held
// it before inserting or updating. Rows are still scanned newest first —
// ORDER BY rowid DESC — purely as a defensive tiebreaker for a store that
// predates the migration and may still have duplicate worktree values; on a
// migrated store there is only ever one match.
func FindQuest(conn *sqlite.Conn, worktreeRoot string) (string, error) {
	if worktreeRoot == "" {
		return "", nil
	}
	target := CanonicalWorktree(worktreeRoot)
	var name string
	err := sqlitex.Execute(conn,
		`SELECT name, worktree FROM fellowship_quests
		 WHERE worktree IS NOT NULL AND worktree != ''
		 ORDER BY rowid DESC`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				if name != "" {
					return nil
				}
				stored := stmt.ColumnText(1)
				if stored == worktreeRoot || CanonicalWorktree(stored) == target {
					name = stmt.ColumnText(0)
				}
				return nil
			},
		})
	return name, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func ptrToAny(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
