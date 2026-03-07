package herald

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

type questState struct {
	QuestName  string  `json:"quest_name"`
	Phase      string  `json:"phase"`
	GatePending bool   `json:"gate_pending"`
	GateID     *string `json:"gate_id"`
}

// DetectProblems scans worktrees for potential issues.
func DetectProblems(dirs []string) []Problem {
	var problems []Problem

	for _, dir := range dirs {
		statePath := filepath.Join(dir, "tmp", "quest-state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var qs questState
		if err := json.Unmarshal(data, &qs); err != nil {
			continue
		}

		questName := qs.QuestName
		if questName == "" {
			questName = filepath.Base(dir)
		}

		// Stalled detection: gate pending for too long
		if qs.GatePending && qs.GateID != nil {
			if ts := extractTimestamp(*qs.GateID); ts > 0 {
				age := time.Since(time.Unix(ts, 0))
				if age > 10*time.Minute {
					problems = append(problems, Problem{
						Quest:    questName,
						Type:     "stalled",
						Severity: Warning,
						Message:  fmt.Sprintf("Gate pending for %s", formatDuration(age)),
					})
				}
			}
		}

		// Zombie detection: quest not Complete, no recent activity
		if qs.Phase != "Complete" {
			tidings, err := Read(dir, 0)
			if err == nil && len(tidings) > 0 {
				last := tidings[len(tidings)-1]
				lastTime, err := time.Parse(time.RFC3339, last.Timestamp)
				if err == nil {
					age := time.Since(lastTime)
					if age > 15*time.Minute {
						problems = append(problems, Problem{
							Quest:    questName,
							Type:     "zombie",
							Severity: Critical,
							Message:  fmt.Sprintf("No activity for %s", formatDuration(age)),
						})
					}
				}
			}
		}

		// Struggling detection: multiple rejections in same phase
		tidings, err := Read(dir, 0)
		if err == nil {
			rejections := 0
			for _, t := range tidings {
				if t.Type == GateRejected && t.Phase == qs.Phase {
					rejections++
				}
			}
			if rejections >= 2 {
				problems = append(problems, Problem{
					Quest:    questName,
					Type:     "struggling",
					Severity: Warning,
					Message:  fmt.Sprintf("Gate rejected %d times in %s phase", rejections, qs.Phase),
				})
			}
		}
	}

	return problems
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
