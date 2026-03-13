package state

import (
	"encoding/json"
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

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

var phaseOrder = []string{"Onboard", "Research", "Plan", "Implement", "Adversarial", "Review", "Complete"}

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

func IsEarlyPhase(phase string) bool {
	return phase == "Onboard" || phase == "Research" || phase == "Plan"
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
					json.Unmarshal([]byte(aa), &s.AutoApproveGates)
				}
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("state: load %s: %w", questName, err)
	}
	if !found {
		return nil, fmt.Errorf("state: quest %q not found", questName)
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

// Delete removes quest state by name.
func Delete(conn *sqlite.Conn, questName string) error {
	return sqlitex.Execute(conn,
		`DELETE FROM quest_state WHERE quest_name = :name`,
		&sqlitex.ExecOptions{Named: map[string]any{":name": questName}})
}

// FindQuest returns the quest name for a given worktree root path.
func FindQuest(conn *sqlite.Conn, worktreeRoot string) (string, error) {
	var name string
	err := sqlitex.Execute(conn,
		`SELECT name FROM fellowship_quests WHERE worktree = :wt`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":wt": worktreeRoot},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				name = stmt.ColumnText(0)
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
