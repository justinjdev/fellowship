package company

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// CompanyProgress returns aggregate progress for a company.
type CompanyProgress struct {
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

// CalculateProgress computes aggregate progress for a company given quest statuses.
func CalculateProgress(company dashboard.CompanyEntry, quests []dashboard.QuestStatus) CompanyProgress {
	progress := CompanyProgress{
		Name:  company.Name,
		Total: len(company.Quests) + len(company.Scouts),
	}

	questByName := make(map[string]dashboard.QuestStatus)
	for _, q := range quests {
		questByName[q.Name] = q
	}

	for _, qName := range company.Quests {
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

// BatchApprove approves all pending gates within a company. It returns the names
// of quests that were approved and any errors encountered (non-fatal).
func BatchApprove(company dashboard.CompanyEntry, fellowshipState *dashboard.FellowshipState) (approved []string, errs []error) {
	questWorktree := make(map[string]string)
	for _, q := range fellowshipState.Quests {
		questWorktree[q.Name] = q.Worktree
	}

	for _, qName := range company.Quests {
		wt, ok := questWorktree[qName]
		if !ok || wt == "" {
			continue
		}

		statePath := filepath.Join(wt, datadir.Name(), "quest-state.json")
		st, err := state.Load(statePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading state for %s: %w", qName, err))
			continue
		}

		if !st.GatePending {
			continue
		}

		prevPhase := st.Phase

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

		now := time.Now().UTC().Format(time.RFC3339)
		herald.Announce(wt, herald.Tiding{
			Timestamp: now, Quest: qName, Type: herald.GateApproved,
			Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
		})
		herald.Announce(wt, herald.Tiding{
			Timestamp: now, Quest: qName, Type: herald.PhaseTransition,
			Phase: nextPhase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, nextPhase),
		})

		approved = append(approved, qName)
	}

	return approved, errs
}

// List prints a summary of all companies in the fellowship state.
func List(statePath string) error {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return err
	}

	if len(fs.Companies) == 0 {
		fmt.Println("No companies defined.")
		return nil
	}

	for _, c := range fs.Companies {
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

// Show prints detailed status for a single company.
func Show(statePath string, name string) error {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return err
	}

	var company *dashboard.CompanyEntry
	for i := range fs.Companies {
		if fs.Companies[i].Name == name {
			company = &fs.Companies[i]
			break
		}
	}
	if company == nil {
		return fmt.Errorf("company %q not found", name)
	}

	// Load quest statuses
	questWorktree := make(map[string]string)
	for _, q := range fs.Quests {
		questWorktree[q.Name] = q.Worktree
	}

	fmt.Printf("Company: %s\n", company.Name)
	fmt.Printf("Quests: %d  Scouts: %d\n\n", len(company.Quests), len(company.Scouts))

	if len(company.Quests) > 0 {
		for _, qName := range company.Quests {
			wt, ok := questWorktree[qName]
			if !ok || wt == "" {
				fmt.Printf("  %-25s (no worktree)\n", qName)
				continue
			}

			statePath := filepath.Join(wt, datadir.Name(), "quest-state.json")
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

	if len(company.Scouts) > 0 {
		fmt.Println()
		for _, sName := range company.Scouts {
			fmt.Printf("  %-25s (scout)\n", sName)
		}
	}

	return nil
}

// Approve batch-approves all pending gates in a company.
func Approve(statePath string, name string) error {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return err
	}

	var company *dashboard.CompanyEntry
	for i := range fs.Companies {
		if fs.Companies[i].Name == name {
			company = &fs.Companies[i]
			break
		}
	}
	if company == nil {
		return fmt.Errorf("company %q not found", name)
	}

	approved, errs := BatchApprove(*company, fs)

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}

	if len(approved) == 0 {
		fmt.Println("No pending gates in company.")
		return nil
	}

	fmt.Printf("Approved %d gate(s):\n", len(approved))
	for _, name := range approved {
		fmt.Printf("  %s\n", name)
	}
	return nil
}

// FindCompanyForQuest returns the company name a quest belongs to, or "" if ungrouped.
func FindCompanyForQuest(companies []dashboard.CompanyEntry, questName string) string {
	for _, c := range companies {
		for _, q := range c.Quests {
			if q == questName {
				return c.Name
			}
		}
	}
	return ""
}

// ProgressSummary returns a human-readable summary like "2/3 quests in Implement+".
func ProgressSummary(progress CompanyProgress) string {
	active := progress.InProgress
	return fmt.Sprintf("%d/%d quests in Implement+", active, progress.Total)
}

// LoadAndMarshalProgress loads state and returns JSON-serializable progress for a company.
func LoadAndMarshalProgress(statePath string, name string) ([]byte, error) {
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err != nil {
		return nil, err
	}

	var company *dashboard.CompanyEntry
	for i := range fs.Companies {
		if fs.Companies[i].Name == name {
			company = &fs.Companies[i]
			break
		}
	}
	if company == nil {
		return nil, fmt.Errorf("company %q not found", name)
	}

	// Build quest statuses
	var quests []dashboard.QuestStatus
	questWorktree := make(map[string]string)
	for _, q := range fs.Quests {
		questWorktree[q.Name] = q.Worktree
	}
	for _, qName := range company.Quests {
		wt, ok := questWorktree[qName]
		if !ok || wt == "" {
			continue
		}
		sp := filepath.Join(wt, datadir.Name(), "quest-state.json")
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

	progress := CalculateProgress(*company, quests)
	return json.Marshal(progress)
}
