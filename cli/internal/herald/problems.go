package herald

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

// Severity represents the severity level of a detected problem.
type Severity string

const (
	Warning  Severity = "warning"
	Critical Severity = "critical"
)

// Problem represents a detected issue with a quest.
type Problem struct {
	Quest    string   `json:"quest"`
	Type     string   `json:"type"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// DetectProblems scans the database for potential quest issues.
func DetectProblems(conn *db.Conn) ([]Problem, error) {
	var problems []Problem

	// Query all active quests (not Complete).
	type questInfo struct {
		questName   string
		phase       string
		gatePending bool
		gateID      string
	}

	var quests []questInfo
	if err := sqlitex.Execute(conn,
		`SELECT quest_name, phase, gate_pending, gate_id FROM quest_state WHERE phase != 'Complete'`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				quests = append(quests, questInfo{
					questName:   stmt.ColumnText(0),
					phase:       stmt.ColumnText(1),
					gatePending: stmt.ColumnInt(2) != 0,
					gateID:      stmt.ColumnText(3),
				})
				return nil
			},
		},
	); err != nil {
		return nil, fmt.Errorf("detect problems: query quests: %w", err)
	}

	for _, qs := range quests {
		// Stalled detection: gate pending for too long
		if qs.gatePending && qs.gateID != "" {
			if ts := extractTimestamp(qs.gateID); ts > 0 {
				age := time.Since(time.Unix(ts, 0))
				if age > 10*time.Minute {
					problems = append(problems, Problem{
						Quest:    qs.questName,
						Type:     "stalled",
						Severity: Warning,
						Message:  fmt.Sprintf("Gate pending for %s", formatDuration(age)),
					})
				}
			}
		}

		// Zombie detection: no recent activity
		var lastTimestamp string
		if err := sqlitex.Execute(conn,
			`SELECT timestamp FROM herald WHERE quest = ? ORDER BY id DESC LIMIT 1`,
			&sqlitex.ExecOptions{
				Args: []any{qs.questName},
				ResultFunc: func(stmt *sqlite.Stmt) error {
					lastTimestamp = stmt.ColumnText(0)
					return nil
				},
			},
		); err != nil {
			return nil, fmt.Errorf("detect problems: query herald for %s: %w", qs.questName, err)
		}
		if lastTimestamp != "" {
			lastTime, err := time.Parse(time.RFC3339, lastTimestamp)
			if err == nil {
				age := time.Since(lastTime)
				if age > 15*time.Minute {
					problems = append(problems, Problem{
						Quest:    qs.questName,
						Type:     "zombie",
						Severity: Critical,
						Message:  fmt.Sprintf("No activity for %s", formatDuration(age)),
					})
				}
			}
		}

		// Struggling detection: multiple rejections in same phase
		var rejections int
		if err := sqlitex.Execute(conn,
			`SELECT count(*) FROM herald WHERE quest = ? AND type = ? AND phase = ?`,
			&sqlitex.ExecOptions{
				Args: []any{qs.questName, string(GateRejected), qs.phase},
				ResultFunc: func(stmt *sqlite.Stmt) error {
					rejections = stmt.ColumnInt(0)
					return nil
				},
			},
		); err != nil {
			return nil, fmt.Errorf("detect problems: query rejections for %s: %w", qs.questName, err)
		}
		if rejections >= 2 {
			problems = append(problems, Problem{
				Quest:    qs.questName,
				Type:     "struggling",
				Severity: Warning,
				Message:  fmt.Sprintf("Gate rejected %d times in %s phase", rejections, qs.phase),
			})
		}
	}

	return problems, nil
}

func extractTimestamp(gateID string) int64 {
	parts := strings.Split(gateID, "-")
	if len(parts) < 2 {
		return 0
	}
	ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0
	}
	return ts
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
