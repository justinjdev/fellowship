package convoy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// ConvoyProgress returns aggregate progress for a convoy.
type ConvoyProgress struct {
	Name       string `json:"name"`
	Total      int    `json:"total"`
	Completed  int    `json:"completed"`
	InProgress int    `json:"in_progress"`
	Pending    int    `json:"pending_gates"` // quests with gate_pending
}

// phaseRank maps phases to a numeric rank for progress tracking.
var phaseRank = map[string]int{
	"Onboard":   0,
	"Research":  1,
	"Plan":      2,
	"Implement": 3,
	"Review":    4,
	"Complete":  5,
}

// CalculateProgress computes aggregate progress for a convoy given quest statuses.
func CalculateProgress(convoy dashboard.ConvoyEntry, quests []dashboard.QuestStatus) ConvoyProgress {
	progress := ConvoyProgress{
		Name:  convoy.Name,
		Total: len(convoy.Quests) + len(convoy.Scouts),
	}

	questByName := make(map[string]dashboard.QuestStatus)
	for _, q := range quests {
		questByName[q.Name] = q
	}

	for _, qName := range convoy.Quests {
		qs, ok := questByName[qName]
		if !ok {
			continue
		}
		if qs.Phase == "Complete" {
			progress.Completed++
		}
		if rank, ok := phaseRank[qs.Phase]; ok && rank >= 3 { // Implement+
			progress.InProgress++
		}
		if qs.GatePending {
			progress.Pending++
		}
	}

	return progress
}

// BatchApprove approves all pending gates within a convoy. It returns the names
// of quests that were approved and any errors encountered (non-fatal).
func BatchApprove(convoy dashboard.ConvoyEntry, fellowshipState *dashboard.FellowshipState) (approved []string, errs []error) {
	questWorktree := make(map[string]string)
	for _, q := range fellowshipState.Quests {
		questWorktree[q.Name] = q.Worktree
	}

	for _, qName := range convoy.Quests {
		wt, ok := questWorktree[qName]
		if !ok || wt == "" {
			continue
		}

		statePath := filepath.Join(wt, "tmp", "quest-state.json")
		st, err := state.Load(statePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading state for %s: %w", qName, err))
			continue
		}

		if !st.GatePending {
			continue
		}

		nextPhase, err := state.NextPhase(st.Phase)
		if err != nil {
			errs = append(errs, fmt.Errorf("advancing phase for %s: %w", qName, err))
			continue
		}

		st.GatePending = false
		st.Phase = nextPhase
		st.GateID = nil
		st.LembasCompleted = false
		st.MetadataUpdated = false

		if err := state.Save(statePath, st); err != nil {
			errs = append(errs, fmt.Errorf("saving state for %s: %w", qName, err))
			continue
		}

		approved = append(approved, qName)
	}

	return approved, errs
}

// List prints a summary of all convoys in the fellowship state.
func List(statePath string) error {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return err
	}

	if len(fs.Convoys) == 0 {
		fmt.Println("No convoys defined.")
		return nil
	}

	for _, c := range fs.Convoys {
		parts := []string{}
		if len(c.Quests) > 0 {
			parts = append(parts, fmt.Sprintf("%d quest(s)", len(c.Quests)))
		}
		if len(c.Scouts) > 0 {
			parts = append(parts, fmt.Sprintf("%d scout(s)", len(c.Scouts)))
		}
		summary := strings.Join(parts, ", ")
		if summary == "" {
			summary = "empty"
		}
		fmt.Printf("%-30s %s\n", c.Name, summary)
	}

	return nil
}

// Show prints detailed status for a single convoy.
func Show(statePath string, name string) error {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return err
	}

	var convoy *dashboard.ConvoyEntry
	for i := range fs.Convoys {
		if fs.Convoys[i].Name == name {
			convoy = &fs.Convoys[i]
			break
		}
	}
	if convoy == nil {
		return fmt.Errorf("convoy %q not found", name)
	}

	// Load quest statuses
	questWorktree := make(map[string]string)
	for _, q := range fs.Quests {
		questWorktree[q.Name] = q.Worktree
	}

	fmt.Printf("Convoy: %s\n", convoy.Name)
	fmt.Printf("Quests: %d  Scouts: %d\n\n", len(convoy.Quests), len(convoy.Scouts))

	if len(convoy.Quests) > 0 {
		for _, qName := range convoy.Quests {
			wt, ok := questWorktree[qName]
			if !ok || wt == "" {
				fmt.Printf("  %-25s (no worktree)\n", qName)
				continue
			}

			statePath := filepath.Join(wt, "tmp", "quest-state.json")
			st, err := state.Load(statePath)
			if err != nil {
				fmt.Printf("  %-25s (state unavailable)\n", qName)
				continue
			}

			gate := ""
			if st.GatePending {
				gate = " [GATE PENDING]"
			}
			fmt.Printf("  %-25s %-12s%s\n", qName, st.Phase, gate)
		}
	}

	if len(convoy.Scouts) > 0 {
		fmt.Println()
		for _, sName := range convoy.Scouts {
			fmt.Printf("  %-25s (scout)\n", sName)
		}
	}

	return nil
}

// Approve batch-approves all pending gates in a convoy.
func Approve(statePath string, name string) error {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return err
	}

	var convoy *dashboard.ConvoyEntry
	for i := range fs.Convoys {
		if fs.Convoys[i].Name == name {
			convoy = &fs.Convoys[i]
			break
		}
	}
	if convoy == nil {
		return fmt.Errorf("convoy %q not found", name)
	}

	approved, errs := BatchApprove(*convoy, fs)

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}

	if len(approved) == 0 {
		fmt.Println("No pending gates in convoy.")
		return nil
	}

	fmt.Printf("Approved %d gate(s):\n", len(approved))
	for _, name := range approved {
		fmt.Printf("  %s\n", name)
	}
	return nil
}

// FindConvoyForQuest returns the convoy name a quest belongs to, or "" if ungrouped.
func FindConvoyForQuest(convoys []dashboard.ConvoyEntry, questName string) string {
	for _, c := range convoys {
		for _, q := range c.Quests {
			if q == questName {
				return c.Name
			}
		}
	}
	return ""
}

// ProgressSummary returns a human-readable summary like "2/3 quests in Implement+".
func ProgressSummary(progress ConvoyProgress) string {
	active := progress.InProgress
	return fmt.Sprintf("%d/%d quests in Implement+", active, progress.Total)
}

// LoadAndMarshalProgress loads state and returns JSON-serializable progress for a convoy.
func LoadAndMarshalProgress(statePath string, name string) ([]byte, error) {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return nil, err
	}

	var convoy *dashboard.ConvoyEntry
	for i := range fs.Convoys {
		if fs.Convoys[i].Name == name {
			convoy = &fs.Convoys[i]
			break
		}
	}
	if convoy == nil {
		return nil, fmt.Errorf("convoy %q not found", name)
	}

	// Build quest statuses
	var quests []dashboard.QuestStatus
	questWorktree := make(map[string]string)
	for _, q := range fs.Quests {
		questWorktree[q.Name] = q.Worktree
	}
	for _, qName := range convoy.Quests {
		wt, ok := questWorktree[qName]
		if !ok || wt == "" {
			continue
		}
		sp := filepath.Join(wt, "tmp", "quest-state.json")
		st, err := state.Load(sp)
		if err != nil {
			continue
		}
		quests = append(quests, dashboard.QuestStatus{
			Name:        qName,
			Worktree:    wt,
			Phase:       st.Phase,
			GatePending: st.GatePending,
		})
	}

	progress := CalculateProgress(*convoy, quests)
	return json.Marshal(progress)
}
