package eagles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
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
	GateThreshold  time.Duration // how long a gate can be pending before "stalled"
	ZombieTimeout  time.Duration // how long since last file change before "zombie"
	Now            time.Time     // injectable clock for testing
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
	}
}

// Sweep scans all quest worktrees and classifies their health.
func Sweep(gitRoot string, opts Options) (*EaglesReport, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	worktrees, err := gitutil.ListWorktrees(gitRoot)
	if err != nil {
		return nil, err
	}

	report := &EaglesReport{
		Timestamp: opts.Now.UTC().Format(time.RFC3339),
		Quests:    []QuestHealth{},
	}

	for _, wt := range worktrees {
		qh, err := classifyQuest(wt, opts)
		if err != nil {
			// Skip worktrees without quest state
			continue
		}
		if qh.Health != Working && qh.Health != Complete {
			report.Problems++
		}
		report.Quests = append(report.Quests, *qh)
	}

	return report, nil
}

// classifyQuest examines a single worktree and returns its health.
func classifyQuest(worktree string, opts Options) (*QuestHealth, error) {
	questStatePath := filepath.Join(worktree, datadir.Name(), "quest-state.json")
	s, err := state.Load(questStatePath)
	if err != nil {
		return nil, err
	}

	hasCheckpoint := gitutil.FileExists(filepath.Join(worktree, datadir.Name(), "checkpoint.md"))
	lastActivity := latestModTime(worktree)

	qh := &QuestHealth{
		Name:          s.QuestName,
		Worktree:      worktree,
		Phase:         s.Phase,
		HasCheckpoint: hasCheckpoint,
		LastActivity:  lastActivity.UTC().Format(time.RFC3339),
	}

	// Classify health
	switch {
	case s.Phase == "Complete":
		qh.Health = Complete
		qh.Action = "none"

	case s.GatePending && s.GateID != nil:
		pendingSec := gitutil.GateAge(*s.GateID, opts.Now)
		qh.GatePendingSec = pendingSec
		if time.Duration(pendingSec)*time.Second >= opts.GateThreshold {
			qh.Health = Stalled
			qh.Action = "nudge"
		} else {
			qh.Health = Working
			qh.Action = "none"
		}

	case s.GatePending:
		// Gate pending but no gate ID — treat as stalled
		qh.Health = Stalled
		qh.Action = "nudge"

	case opts.Now.Sub(lastActivity) >= opts.ZombieTimeout && s.Phase != "Onboard":
		qh.Health = Zombie
		if hasCheckpoint {
			qh.Action = "respawn"
		} else {
			qh.Action = "nudge"
		}

	case s.Phase == "Onboard" && s.QuestName == "":
		qh.Health = Idle
		qh.Action = "none"

	default:
		qh.Health = Working
		qh.Action = "none"
	}

	return qh, nil
}


// latestModTime walks the worktree (excluding .git, data dir, and node_modules) to find the most
// recently modified file.
func latestModTime(worktree string) time.Time {
	var latest time.Time
	filepath.Walk(worktree, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip .git, fellowship data dir (internal state), and node_modules directories
		name := info.Name()
		if info.IsDir() && (name == ".git" || name == datadir.Name() || name == "node_modules") {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}

// WriteReport writes the eagles report to the data directory in the git root.
func WriteReport(gitRoot string, report *EaglesReport) error {
	dir := filepath.Join(gitRoot, datadir.Name())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
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

