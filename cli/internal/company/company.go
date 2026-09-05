package company

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/gate"
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

// phaseRank maps phases to a numeric rank for progress tracking,
// derived from the canonical phase order in the state package.
var phaseRank = func() map[string]int {
	m := make(map[string]int)
	for i, p := range state.Phases() {
		m[p] = i
	}
	return m
}()

// CalculateProgress computes aggregate progress for a company given quest statuses.
func CalculateProgress(company fellowship.CompanyEntry, quests []fellowship.QuestStatus) CompanyProgress {
	progress := CompanyProgress{
		Name:  company.Name,
		Total: len(company.Quests) + len(company.Scouts),
	}

	questByName := make(map[string]fellowship.QuestStatus)
	for _, q := range quests {
		questByName[q.Name] = q
	}

	for _, qName := range company.Quests {
		qs, ok := questByName[qName]
		if !ok {
			continue
		}
		// Review is terminal but a quest sits in it while the PR is opened,
		// so "done" is the fellowship entry's status, not a phase.
		if qs.Status == "completed" {
			progress.Completed++
		}
		if rank, ok := phaseRank[qs.Phase]; ok && rank >= phaseRank["Implement"] {
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
func BatchApprove(conn *sqlite.Conn, company fellowship.CompanyEntry) (approved []string, errs []error) {
	for _, qName := range company.Quests {
		st, err := state.Load(conn, qName)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading state for %s: %w", qName, err))
			continue
		}

		if !st.GatePending {
			continue
		}

		// Same transition the lead's `gate approve` performs.
		prevPhase, nextPhase, err := state.Approve(st)
		if err != nil {
			errs = append(errs, fmt.Errorf("advancing phase for %s: %w", qName, err))
			continue
		}

		if err := state.Upsert(conn, st); err != nil {
			errs = append(errs, fmt.Errorf("saving state for %s: %w", qName, err))
			continue
		}

		if err := gate.RecordApproval(conn, qName, prevPhase, nextPhase,
			fmt.Sprintf("Batch approved for company %s", company.Name)); err != nil {
			errs = append(errs, err)
		}

		approved = append(approved, qName)
	}

	return approved, errs
}

// List prints a summary of all companies in the fellowship state.
func List(conn *sqlite.Conn) error {
	companies, err := fellowship.ListCompanies(conn)
	if err != nil {
		return err
	}

	if len(companies) == 0 {
		fmt.Println("No companies defined.")
		return nil
	}

	for _, c := range companies {
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
func Show(conn *sqlite.Conn, name string) error {
	company, err := findCompany(conn, name)
	if err != nil {
		return err
	}

	fmt.Printf("Company: %s\n", company.Name)
	fmt.Printf("Quests: %d  Scouts: %d\n\n", len(company.Quests), len(company.Scouts))

	if len(company.Quests) > 0 {
		for _, qName := range company.Quests {
			st, err := state.Load(conn, qName)
			if err != nil {
				fmt.Printf("  %-25s (state unavailable)\n", qName)
				continue
			}

			pending := ""
			if st.GatePending {
				pending = " [GATE PENDING]"
			}
			fmt.Printf("  %-25s %-12s%s\n", qName, st.Phase, pending)
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
func Approve(conn *sqlite.Conn, name string) error {
	company, err := findCompany(conn, name)
	if err != nil {
		return err
	}

	approved, errs := BatchApprove(conn, *company)

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
func FindCompanyForQuest(companies []fellowship.CompanyEntry, questName string) string {
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
func LoadAndMarshalProgress(conn *sqlite.Conn, name string) ([]byte, error) {
	company, err := findCompany(conn, name)
	if err != nil {
		return nil, err
	}

	// Build quest statuses from DB.
	entryStatus, err := questEntryStatuses(conn)
	if err != nil {
		return nil, err
	}
	var quests []fellowship.QuestStatus
	for _, qName := range company.Quests {
		st, err := state.Load(conn, qName)
		if err != nil {
			continue
		}
		quests = append(quests, fellowship.QuestStatus{
			Name:        qName,
			Phase:       st.Phase,
			GatePending: st.GatePending,
			Status:      entryStatus[qName],
		})
	}

	progress := CalculateProgress(*company, quests)
	return json.Marshal(progress)
}

// questEntryStatuses maps quest name to its fellowship entry status, treating
// a missing or empty value as "active". Review is terminal, so this status is
// the only thing that says a quest has finished.
func questEntryStatuses(conn *sqlite.Conn) (map[string]string, error) {
	out := map[string]string{}
	err := sqlitex.Execute(conn,
		`SELECT name, COALESCE(NULLIF(status, ''), 'active') FROM fellowship_quests`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				out[stmt.ColumnText(0)] = stmt.ColumnText(1)
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("loading quest entry statuses: %w", err)
	}
	return out, nil
}

// findCompany looks up a company by name from the DB.
func findCompany(conn *sqlite.Conn, name string) (*fellowship.CompanyEntry, error) {
	var found bool
	entry := &fellowship.CompanyEntry{
		Quests: []string{},
		Scouts: []string{},
	}

	err := sqlitex.Execute(conn,
		`SELECT name FROM companies WHERE name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": name},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				entry.Name = stmt.ColumnText(0)
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("company: lookup %s: %w", name, err)
	}
	if !found {
		return nil, fmt.Errorf("company %q not found", name)
	}

	// Load members
	err = sqlitex.Execute(conn,
		`SELECT member_name, member_type FROM company_members WHERE company_name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": name},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				memberName := stmt.ColumnText(0)
				memberType := stmt.ColumnText(1)
				switch memberType {
				case "quest":
					entry.Quests = append(entry.Quests, memberName)
				case "scout":
					entry.Scouts = append(entry.Scouts, memberName)
				}
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("company: load members for %s: %w", name, err)
	}

	return entry, nil
}
