package tome

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type QuestTome struct {
	QuestName       string        `json:"quest_name"`
	PhasesCompleted []PhaseRecord `json:"phases_completed"`
	GateHistory     []GateEvent   `json:"gate_history"`
	FilesTouched    []string      `json:"files_touched"`
	Respawns        int           `json:"respawns"`
	Status          string        `json:"status"` // "active", "completed", "failed"
	Task            string        `json:"task"`
}

type PhaseRecord struct {
	Phase       string `json:"phase"`
	CompletedAt string `json:"completed_at"`
	DurationS   int    `json:"duration_s,omitempty"`
}

type GateEvent struct {
	Phase     string `json:"phase"`
	Action    string `json:"action"` // "submitted", "approved", "rejected", "skipped"
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

// RecordPhase inserts a phase completion record into quest_phases.
func RecordPhase(conn *sqlite.Conn, questName, phase string, durationS int) error {
	return sqlitex.Execute(conn,
		`INSERT INTO quest_phases (quest_name, phase, completed_at, duration_s)
		 VALUES (:quest, :phase, :now, :dur)`,
		&sqlitex.ExecOptions{Named: map[string]any{
			":quest": questName,
			":phase": phase,
			":now":   time.Now().UTC().Format(time.RFC3339),
			":dur":   durationS,
		}})
}

// RecordGate inserts a gate event into quest_gates.
func RecordGate(conn *sqlite.Conn, questName, phase, action, reason string) error {
	return sqlitex.Execute(conn,
		`INSERT INTO quest_gates (quest_name, phase, action, timestamp, reason)
		 VALUES (:quest, :phase, :action, :now, :reason)`,
		&sqlitex.ExecOptions{Named: map[string]any{
			":quest":  questName,
			":phase":  phase,
			":action": action,
			":now":    time.Now().UTC().Format(time.RFC3339),
			":reason": reason,
		}})
}

// RecordFiles inserts file paths into quest_files, ignoring duplicates.
func RecordFiles(conn *sqlite.Conn, questName string, files []string) error {
	for _, f := range files {
		if err := sqlitex.Execute(conn,
			`INSERT OR IGNORE INTO quest_files (quest_name, file_path) VALUES (:quest, :file)`,
			&sqlitex.ExecOptions{Named: map[string]any{":quest": questName, ":file": f}},
		); err != nil {
			return err
		}
	}
	return nil
}

// RecordSkippedPhases records multiple phases as skipped with a reason.
func RecordSkippedPhases(conn *sqlite.Conn, questName string, phases []string, reason string) error {
	for _, p := range phases {
		if err := RecordPhase(conn, questName, p, 0); err != nil {
			return err
		}
		if err := RecordGate(conn, questName, p, "skipped", reason); err != nil {
			return err
		}
	}
	return nil
}

// LoadPhases returns all phase records for a quest, ordered by insertion.
func LoadPhases(conn *sqlite.Conn, questName string) ([]PhaseRecord, error) {
	var phases []PhaseRecord
	err := sqlitex.Execute(conn,
		`SELECT phase, completed_at, duration_s FROM quest_phases WHERE quest_name = :quest ORDER BY id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				phases = append(phases, PhaseRecord{
					Phase:       stmt.ColumnText(0),
					CompletedAt: stmt.ColumnText(1),
					DurationS:   stmt.ColumnInt(2),
				})
				return nil
			},
		})
	return phases, err
}

// LoadGates returns all gate events for a quest, ordered by insertion.
func LoadGates(conn *sqlite.Conn, questName string) ([]GateEvent, error) {
	var gates []GateEvent
	err := sqlitex.Execute(conn,
		`SELECT phase, action, timestamp, reason FROM quest_gates WHERE quest_name = :quest ORDER BY id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				gates = append(gates, GateEvent{
					Phase:     stmt.ColumnText(0),
					Action:    stmt.ColumnText(1),
					Timestamp: stmt.ColumnText(2),
					Reason:    stmt.ColumnText(3),
				})
				return nil
			},
		})
	return gates, err
}

// LoadFiles returns all file paths for a quest.
func LoadFiles(conn *sqlite.Conn, questName string) ([]string, error) {
	var files []string
	err := sqlitex.Execute(conn,
		`SELECT file_path FROM quest_files WHERE quest_name = :quest ORDER BY file_path`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				files = append(files, stmt.ColumnText(0))
				return nil
			},
		})
	return files, err
}

// Load assembles a QuestTome from the database for the given quest.
// Returns a zero-value tome if no data exists (equivalent to old LoadOrCreate).
func Load(conn *sqlite.Conn, questName string) (*QuestTome, error) {
	phases, err := LoadPhases(conn, questName)
	if err != nil {
		return nil, fmt.Errorf("tome: load phases: %w", err)
	}
	gates, err := LoadGates(conn, questName)
	if err != nil {
		return nil, fmt.Errorf("tome: load gates: %w", err)
	}
	files, err := LoadFiles(conn, questName)
	if err != nil {
		return nil, fmt.Errorf("tome: load files: %w", err)
	}

	// Load status/task/respawns from fellowship_quests.
	var status, task string
	var respawns int
	_ = sqlitex.Execute(conn,
		`SELECT status, task_description, respawns FROM fellowship_quests WHERE name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				status = stmt.ColumnText(0)
				task = stmt.ColumnText(1)
				respawns = stmt.ColumnInt(2)
				return nil
			},
		})
	if status == "" {
		status = "active"
	}

	// Ensure non-nil slices for JSON serialization.
	if phases == nil {
		phases = []PhaseRecord{}
	}
	if gates == nil {
		gates = []GateEvent{}
	}
	if files == nil {
		files = []string{}
	}

	return &QuestTome{
		QuestName:       questName,
		PhasesCompleted: phases,
		GateHistory:     gates,
		FilesTouched:    files,
		Status:          status,
		Task:            task,
		Respawns:        respawns,
	}, nil
}

// SetStatus updates the quest status in fellowship_quests.
func SetStatus(conn *sqlite.Conn, questName, status string) error {
	return sqlitex.Execute(conn,
		`UPDATE fellowship_quests SET status = :status WHERE name = :quest`,
		&sqlitex.ExecOptions{Named: map[string]any{":quest": questName, ":status": status}})
}
