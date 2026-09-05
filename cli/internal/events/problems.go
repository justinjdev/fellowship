package events

import (
	"fmt"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/health"
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

// Options configures DetectProblems. Now is exposed so tests can inject a
// fixed clock instead of depending on wall-clock offsets; the zero value
// defaults to time.Now(), exactly as health.DefaultOptions does — and,
// unlike before, that resolved value is what both health.Sweep and this
// package's own zombie-age formatting actually use (previously they only
// agreed when Now happened to be its zero value, which produced a
// nonsensical "no activity for" duration for every zombie).
type Options struct {
	Now time.Time
}

// DetectProblems scans the database for potential quest issues. It is a view
// over the health package's sweep — the same classification behind
// `fellowship health` — translated into the Problem shape `fellowship
// events --problems` and the dashboard expect, rather than a second set of
// thresholds. opts is variadic so existing callers are unaffected; pass one
// to inject Now.
func DetectProblems(conn *db.Conn, opts ...Options) ([]Problem, error) {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}

	// Resolve the clock once so classification (inside Sweep) and the
	// message formatting below agree on "now".
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	hOpts := health.DefaultOptions()
	hOpts.Now = now
	report, err := health.Sweep(conn, hOpts)
	if err != nil {
		return nil, fmt.Errorf("detect problems: %w", err)
	}

	var problems []Problem
	for _, qh := range report.Quests {
		switch qh.Health {
		case health.Stalled:
			problems = append(problems, Problem{
				Quest:    qh.Name,
				Type:     "stalled",
				Severity: Warning,
				Message:  fmt.Sprintf("Gate pending for %s", formatDuration(time.Duration(qh.GatePendingSec)*time.Second)),
			})
		case health.Zombie:
			problems = append(problems, Problem{
				Quest:    qh.Name,
				Type:     "zombie",
				Severity: Critical,
				Message:  fmt.Sprintf("No activity for %s", formatDuration(activityAge(qh.LastActivity, now))),
			})
		}
		// Struggling is orthogonal to Health (see health.QuestHealth), so it's
		// checked independently of the switch above.
		if qh.Struggling {
			problems = append(problems, Problem{
				Quest:    qh.Name,
				Type:     "struggling",
				Severity: Warning,
				Message:  fmt.Sprintf("Gate rejected %d times in %s phase", qh.RejectionCount, qh.Phase),
			})
		}
	}

	return problems, nil
}

// activityAge returns how long ago the given ISO 8601 timestamp was, relative
// to now. An unparseable or empty timestamp reports zero rather than erroring
// — the zombie classification itself already required a valid timestamp.
func activityAge(lastActivity string, now time.Time) time.Duration {
	t, err := time.Parse(time.RFC3339, lastActivity)
	if err != nil {
		return 0
	}
	return now.Sub(t)
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
