package group

import (
	"fmt"
	"os"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/gate"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// GroupProgress returns aggregate progress for a group.
type GroupProgress struct {
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

// CalculateProgress computes aggregate progress for a group given quest statuses.
func CalculateProgress(grp fellowship.GroupEntry, quests []fellowship.QuestStatus) GroupProgress {
	progress := GroupProgress{
		Name:  grp.Name,
		Total: len(grp.Quests) + len(grp.Scouts),
	}

	questByName := make(map[string]fellowship.QuestStatus)
	for _, q := range quests {
		questByName[q.Name] = q
	}

	for _, qName := range grp.Quests {
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

// BatchApprove approves all pending gates within a group. It returns the names
// of quests that were approved and any errors encountered (non-fatal).
func BatchApprove(conn *sqlite.Conn, grp fellowship.GroupEntry) (approved []string, errs []error) {
	for _, qName := range grp.Quests {
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
			fmt.Sprintf("Batch approved for group %s", grp.Name)); err != nil {
			errs = append(errs, err)
		}

		approved = append(approved, qName)
	}

	return approved, errs
}

// List prints a summary of all groups in the fellowship state.
func List(conn *sqlite.Conn) error {
	groups, err := fellowship.ListGroups(conn)
	if err != nil {
		return err
	}

	if len(groups) == 0 {
		fmt.Println("No groups defined.")
		return nil
	}

	for _, c := range groups {
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

// QuestSummary is one quest's status within a group, for `group show --json`.
type QuestSummary struct {
	Name        string `json:"name"`
	Phase       string `json:"phase,omitempty"`
	GatePending bool   `json:"gate_pending"`
	Unavailable bool   `json:"unavailable,omitempty"` // state could not be loaded
}

// Detail is a group's full detail, for `group show --json`.
type Detail struct {
	Name     string         `json:"name"`
	Quests   []QuestSummary `json:"quests"`
	Scouts   []string       `json:"scouts"`
	Progress GroupProgress  `json:"progress"`
}

// LoadDetail loads a group's full detail — the same data Show prints as a
// table, structured for JSON output.
func LoadDetail(conn *sqlite.Conn, name string) (*Detail, error) {
	grp, err := findGroup(conn, name)
	if err != nil {
		return nil, err
	}

	d := &Detail{
		Name:   grp.Name,
		Quests: []QuestSummary{},
		Scouts: append([]string{}, grp.Scouts...),
	}
	for _, qName := range grp.Quests {
		st, err := state.Load(conn, qName)
		if err != nil {
			d.Quests = append(d.Quests, QuestSummary{Name: qName, Unavailable: true})
			continue
		}
		d.Quests = append(d.Quests, QuestSummary{Name: qName, Phase: st.Phase, GatePending: st.GatePending})
	}

	// DiscoverQuests carries the fellowship-entry status (completed/cancelled)
	// alongside phase and gate state, exactly what CalculateProgress needs and
	// more than state.Load alone provides.
	dash, err := fellowship.DiscoverQuests(conn)
	if err != nil {
		return nil, err
	}
	d.Progress = CalculateProgress(*grp, dash.Quests)

	return d, nil
}

// Show prints detailed status for a single group.
func Show(conn *sqlite.Conn, name string) error {
	grp, err := findGroup(conn, name)
	if err != nil {
		return err
	}

	fmt.Printf("Group: %s\n", grp.Name)
	fmt.Printf("Quests: %d  Scouts: %d\n\n", len(grp.Quests), len(grp.Scouts))

	if len(grp.Quests) > 0 {
		for _, qName := range grp.Quests {
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

	if len(grp.Scouts) > 0 {
		fmt.Println()
		for _, sName := range grp.Scouts {
			fmt.Printf("  %-25s (scout)\n", sName)
		}
	}

	return nil
}

// Approve batch-approves all pending gates in a group.
func Approve(conn *sqlite.Conn, name string) error {
	grp, err := findGroup(conn, name)
	if err != nil {
		return err
	}

	approved, errs := BatchApprove(conn, *grp)

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}

	if len(approved) == 0 {
		fmt.Println("No pending gates in group.")
		return nil
	}

	fmt.Printf("Approved %d gate(s):\n", len(approved))
	for _, name := range approved {
		fmt.Printf("  %s\n", name)
	}
	return nil
}

// findGroup looks up a group by name from the DB.
func findGroup(conn *sqlite.Conn, name string) (*fellowship.GroupEntry, error) {
	var found bool
	entry := &fellowship.GroupEntry{
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
		return nil, fmt.Errorf("group: lookup %s: %w", name, err)
	}
	if !found {
		return nil, fmt.Errorf("group %q not found", name)
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
		return nil, fmt.Errorf("group: load members for %s: %w", name, err)
	}

	return entry, nil
}
