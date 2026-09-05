package health

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
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

// QuestHealth holds the health assessment for a single quest. This is the one
// classification both `fellowship health` and `fellowship events --problems`
// read — the latter translates it into its own Problem shape rather than
// recomputing thresholds independently.
type QuestHealth struct {
	Name           string      `json:"name"`
	Worktree       string      `json:"worktree"`
	Phase          string      `json:"phase"`
	Health         HealthState `json:"health"`
	GatePendingSec int         `json:"gate_pending_sec,omitempty"`
	HasCheckpoint  bool        `json:"has_checkpoint"`
	LastActivity   string      `json:"last_activity"` // ISO 8601
	Action         string      `json:"action"`        // recommended action: "none", "nudge", "respawn"

	// Struggling is orthogonal to Health: a quest can be actively Working and
	// still be struggling (repeated rejections in its current phase), so it's
	// reported alongside rather than folded into the Health chain.
	Struggling     bool `json:"struggling,omitempty"`
	RejectionCount int  `json:"rejection_count,omitempty"`
}

// HealthReport holds the full health scan result.
type HealthReport struct {
	Timestamp string        `json:"timestamp"`
	Quests    []QuestHealth `json:"quests"`
	Problems  int           `json:"problems"` // count of non-working/non-complete
}

// Options configures the health scan.
type Options struct {
	GateThreshold        time.Duration // how long a gate can be pending before "stalled"
	ZombieTimeout        time.Duration // how long since last file change before "zombie"
	StrugglingRejections int           // gate rejections in the same phase before "struggling"
	Now                  time.Time     // injectable clock for testing
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		GateThreshold:        10 * time.Minute,
		ZombieTimeout:        15 * time.Minute,
		StrugglingRejections: 2,
	}
}

// Sweep scans all quests in the database and classifies their health.
func Sweep(conn *db.Conn, opts Options) (*HealthReport, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.StrugglingRejections <= 0 {
		opts.StrugglingRejections = DefaultOptions().StrugglingRejections
	}

	// Load all quest states from quest_state table.
	states, err := listAllQuests(conn)
	if err != nil {
		return nil, fmt.Errorf("health: list quests: %w", err)
	}

	// Review is terminal, so a finished quest is not distinguishable by
	// phase — the fellowship entry's status is what says the quest is done.
	finished, err := finishedQuests(conn)
	if err != nil {
		return nil, fmt.Errorf("health: list finished quests: %w", err)
	}

	// quest_state carries no worktree column — join fellowship_quests by name
	// so each report entry names the worktree it is about, the way the
	// dashboard and `health` table output need to.
	worktrees, err := questWorktrees(conn)
	if err != nil {
		return nil, fmt.Errorf("health: list quest worktrees: %w", err)
	}

	report := &HealthReport{
		Timestamp: opts.Now.UTC().Format(time.RFC3339),
		Quests:    []QuestHealth{},
	}

	for _, s := range states {
		qh := classifyQuest(conn, s, finished[s.QuestName], opts)
		qh.Worktree = worktrees[s.QuestName]
		if (qh.Health != Working && qh.Health != Complete) || qh.Struggling {
			report.Problems++
		}
		report.Quests = append(report.Quests, qh)
	}

	return report, nil
}

// questWorktrees maps quest name to its registered worktree path, from
// fellowship_quests.
func questWorktrees(conn *db.Conn) (map[string]string, error) {
	worktrees := map[string]string{}
	err := sqlitex.Execute(conn,
		`SELECT name, worktree FROM fellowship_quests`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				worktrees[stmt.ColumnText(0)] = stmt.ColumnText(1)
				return nil
			},
		})
	if err != nil {
		return nil, err
	}
	return worktrees, nil
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

// finishedQuests returns the set of quest names whose fellowship entry is no
// longer active — completed or cancelled work that needs no health chasing.
func finishedQuests(conn *db.Conn) (map[string]bool, error) {
	finished := map[string]bool{}
	err := sqlitex.Execute(conn,
		`SELECT name FROM fellowship_quests WHERE status IN ('completed', 'cancelled')`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				finished[stmt.ColumnText(0)] = true
				return nil
			},
		})
	return finished, err
}

// classifyQuest examines a quest's state and events to determine health.
func classifyQuest(conn *db.Conn, s *state.State, finished bool, opts Options) QuestHealth {
	qh := QuestHealth{
		Name:   s.QuestName,
		Phase:  s.Phase,
		Action: "none",
	}

	// Finished quests are always healthy.
	if finished {
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

	// Struggling is orthogonal to the Health chain below — a quest can be
	// actively Working and still be repeatedly rejected in its current phase.
	qh.RejectionCount = rejectionCount(conn, s.QuestName, s.Phase)
	qh.Struggling = qh.RejectionCount >= opts.StrugglingRejections

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

	// Check for zombie: use updated_at from quest_state and event timestamps.
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

// lastActivity returns the most recent timestamp from events for a quest, or
// falls back to the quest_state updated_at. It queries the underlying event
// log table directly rather than importing the events package, so health
// carries no dependency on it — events depends on health for classification
// instead.
func lastActivity(conn *db.Conn, s *state.State) string {
	var timestamp string
	sqlitex.Execute(conn,
		`SELECT timestamp FROM herald WHERE quest = :name ORDER BY id DESC LIMIT 1`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": s.QuestName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				timestamp = stmt.ColumnText(0)
				return nil
			},
		})
	if timestamp != "" {
		return timestamp
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

// rejectionCount returns how many gate_rejected events a quest has recorded
// in its current phase — the "struggling" signal. It uses the literal event
// type string rather than the events package's constant for the same reason
// lastActivity queries the table directly: keeping health free of a
// dependency on events.
func rejectionCount(conn *db.Conn, questName, phase string) int {
	var count int
	sqlitex.Execute(conn,
		`SELECT count(*) FROM herald WHERE quest = :name AND type = 'gate_rejected' AND phase = :phase`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName, ":phase": phase},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		})
	return count
}

// hasCheckpoint checks if the quest has a checkpoint by looking for
// a lembas_completed event, which indicates checkpoint creation.
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

// FormatTable returns a human-readable table of the health report.
func FormatTable(report *HealthReport) string {
	var sb strings.Builder
	sb.WriteString("Fellowship Health Report\n")
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
