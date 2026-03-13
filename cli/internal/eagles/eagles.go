package eagles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// HealthState represents the health classification of a quest.
type HealthState string

const (
	Working  HealthState = "working"  // Active, making progress
	Stalled  HealthState = "stalled"  // Gate pending too long (configurable threshold)
	Zombie   HealthState = "zombie"   // Has checkpoint but no recent file changes
	Idle     HealthState = "idle"     // No work assigned
	Complete HealthState = "complete" // Quest finished
)

// QuestHealth holds the health assessment for a single quest.
type QuestHealth struct {
	Name           string      `json:"name"`
	Worktree       string      `json:"worktree"`
	Phase          string      `json:"phase"`
	Health         HealthState `json:"health"`
	GatePendingSec int         `json:"gate_pending_sec,omitempty"`
	HasCheckpoint  bool        `json:"has_checkpoint"`
	LastActivity   string      `json:"last_activity"` // ISO 8601
	Action         string      `json:"action"`        // recommended action: "none", "nudge", "respawn"
}

// EaglesReport holds the full eagles scan result.
type EaglesReport struct {
	Timestamp string        `json:"timestamp"`
	Quests    []QuestHealth `json:"quests"`
	Problems  int           `json:"problems"` // count of non-working/non-complete
}

// Options configures the eagles scan.
type Options struct {
	GateThreshold time.Duration // how long a gate can be pending before "stalled"
	ZombieTimeout time.Duration // how long since last file change before "zombie"
	Now           time.Time     // injectable clock for testing
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
	}
}

// Sweep scans all quests in the database and classifies their health.
func Sweep(conn *db.Conn, opts Options) (*EaglesReport, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	// Load all quest states from quest_state table.
	states, err := listAllQuests(conn)
	if err != nil {
		return nil, fmt.Errorf("eagles: list quests: %w", err)
	}

	report := &EaglesReport{
		Timestamp: opts.Now.UTC().Format(time.RFC3339),
		Quests:    []QuestHealth{},
	}

	for _, s := range states {
		qh := classifyQuest(conn, s, opts)
		if qh.Health != Working && qh.Health != Complete {
			report.Problems++
		}
		report.Quests = append(report.Quests, qh)
	}

	return report, nil
}

// listAllQuests returns all quest states from the database.
func listAllQuests(conn *db.Conn) ([]*state.State, error) {
	var states []*state.State
	err := sqlitex.Execute(conn,
		`SELECT quest_name, task_id, team_name, phase,
			gate_pending, gate_id, lembas_completed, metadata_updated,
			held, held_reason, auto_approve
			FROM quest_state ORDER BY quest_name`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				s := &state.State{
					QuestName:       stmt.ColumnText(0),
					TaskID:          stmt.ColumnText(1),
					TeamName:        stmt.ColumnText(2),
					Phase:           stmt.ColumnText(3),
					GatePending:     stmt.ColumnInt(4) != 0,
					LembasCompleted: stmt.ColumnInt(6) != 0,
					MetadataUpdated: stmt.ColumnInt(7) != 0,
					Held:            stmt.ColumnInt(8) != 0,
				}
				if stmt.ColumnType(5) != sqlite.TypeNull {
					gid := stmt.ColumnText(5)
					s.GateID = &gid
				}
				if stmt.ColumnType(9) != sqlite.TypeNull {
					hr := stmt.ColumnText(9)
					s.HeldReason = &hr
				}
				if aa := stmt.ColumnText(10); aa != "" {
					json.Unmarshal([]byte(aa), &s.AutoApproveGates)
				}
				states = append(states, s)
				return nil
			},
		})
	if err != nil {
		return nil, err
	}
	return states, nil
}

// classifyQuest examines a quest's state and herald tidings to determine health.
func classifyQuest(conn *db.Conn, s *state.State, opts Options) QuestHealth {
	qh := QuestHealth{
		Name:   s.QuestName,
		Phase:  s.Phase,
		Action: "none",
	}

	// Complete quests are always healthy.
	if s.Phase == "Complete" {
		qh.Health = Complete
		qh.LastActivity = lastActivity(conn, s)
		return qh
	}

	// Idle: no quest name assigned (onboarding placeholder).
	if s.QuestName == "" {
		qh.Health = Idle
		qh.LastActivity = lastActivity(conn, s)
		return qh
	}

	// Check for stalled gates.
	if s.GatePending {
		if s.GateID != nil {
			age := gitutil.GateAge(*s.GateID, opts.Now)
			qh.GatePendingSec = age
			if age >= int(opts.GateThreshold.Seconds()) {
				qh.Health = Stalled
				qh.Action = "nudge"
				qh.LastActivity = lastActivity(conn, s)
				return qh
			}
		} else {
			// Gate pending with no ID — assume stalled (cannot determine age).
			qh.Health = Stalled
			qh.Action = "nudge"
			qh.LastActivity = lastActivity(conn, s)
			return qh
		}
	}

	// Check for zombie: use updated_at from quest_state and herald timestamps.
	lastAct := lastActivity(conn, s)
	qh.LastActivity = lastAct

	if lastAct != "" {
		if t, err := time.Parse(time.RFC3339, lastAct); err == nil {
			if opts.Now.Sub(t) > opts.ZombieTimeout {
				qh.Health = Zombie
				qh.HasCheckpoint = hasCheckpoint(conn, s.QuestName)
				if qh.HasCheckpoint {
					qh.Action = "respawn"
				} else {
					qh.Action = "nudge"
				}
				return qh
			}
		}
	}

	qh.Health = Working
	return qh
}

// lastActivity returns the most recent timestamp from herald tidings for a quest,
// or falls back to the quest_state updated_at.
func lastActivity(conn *db.Conn, s *state.State) string {
	tidings, err := herald.Read(conn, s.QuestName, 1)
	if err == nil && len(tidings) > 0 {
		return tidings[0].Timestamp
	}

	// Fall back to updated_at from quest_state.
	var updatedAt string
	sqlitex.Execute(conn,
		`SELECT updated_at FROM quest_state WHERE quest_name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": s.QuestName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				updatedAt = stmt.ColumnText(0)
				return nil
			},
		})
	return updatedAt
}

// hasCheckpoint checks if the quest has a checkpoint by looking for
// a lembas_completed herald tiding, which indicates checkpoint creation.
func hasCheckpoint(conn *db.Conn, questName string) bool {
	var found bool
	sqlitex.Execute(conn,
		`SELECT 1 FROM herald WHERE quest = :name AND type = 'lembas_completed' LIMIT 1`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				return nil
			},
		})
	return found
}

// WriteReport writes the eagles report to the data directory in the git root.
func WriteReport(gitRoot string, report *EaglesReport) error {
	dir := filepath.Join(gitRoot, datadir.Name())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating data dir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(dir, "eagles-report.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming report: %w", err)
	}
	return nil
}

// FormatTable returns a human-readable table of the eagles report.
func FormatTable(report *EaglesReport) string {
	var sb strings.Builder
	sb.WriteString("Fellowship Eagles Report\n")
	sb.WriteString(strings.Repeat("\u2501", 80) + "\n")
	sb.WriteString(fmt.Sprintf("%-20s \u2502 %-10s \u2502 %-8s \u2502 %-8s \u2502 %s\n",
		"Quest", "Phase", "Health", "Action", "Last Activity"))
	sb.WriteString(strings.Repeat("\u2500", 80) + "\n")

	for _, q := range report.Quests {
		name := q.Name
		if name == "" {
			name = filepath.Base(q.Worktree)
		}
		sb.WriteString(fmt.Sprintf("%-20s \u2502 %-10s \u2502 %-8s \u2502 %-8s \u2502 %s\n",
			name, q.Phase, q.Health, q.Action, q.LastActivity))
	}

	sb.WriteString(strings.Repeat("\u2500", 80) + "\n")
	sb.WriteString(fmt.Sprintf("Problems: %d\n", report.Problems))
	return sb.String()
}
