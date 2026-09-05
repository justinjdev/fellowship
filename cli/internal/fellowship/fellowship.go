package fellowship

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/todo"
)

type GroupEntry struct {
	Name   string   `json:"name"`
	Quests []string `json:"quests"` // quest names
	Scouts []string `json:"scouts"` // scout names
}

type FellowshipState struct {
	Version    int          `json:"version"`
	Name       string       `json:"name"`
	CreatedAt  string       `json:"created_at"`
	MainRepo   string       `json:"main_repo"`
	BaseBranch string       `json:"base_branch,omitempty"`
	Quests     []QuestEntry `json:"quests"`
	Scouts     []ScoutEntry `json:"scouts"`
	Groups     []GroupEntry `json:"groups"`
}

type QuestEntry struct {
	Name            string `json:"name"`
	TaskDescription string `json:"task_description"`
	Worktree        string `json:"worktree"`
	Branch          string `json:"branch"`
	TaskID          string `json:"task_id"`
	Status          string `json:"status,omitempty"`
}

// QuestEntryStatus returns the effective status of a quest entry.
// Returns q.Status if set, otherwise "active".
func QuestEntryStatus(q QuestEntry) string {
	if q.Status != "" {
		return q.Status
	}
	return "active"
}

type ScoutEntry struct {
	Name     string `json:"name"`
	Question string `json:"question"`
	TaskID   string `json:"task_id"`
}

type QuestStatus struct {
	Name            string  `json:"name"`
	Worktree        string  `json:"worktree"`
	Phase           string  `json:"phase"`
	Status          string  `json:"status"`
	GatePending     bool    `json:"gate_pending"`
	GateID          *string `json:"gate_id"`
	LembasCompleted bool    `json:"lembas_completed"`
	MetadataUpdated bool    `json:"metadata_updated"`
	TodosDone       int     `json:"todos_done"`
	TodosTotal      int     `json:"todos_total"`
}

type DashboardStatus struct {
	Name         string        `json:"name"`
	Quests       []QuestStatus `json:"quests"`
	Scouts       []ScoutEntry  `json:"scouts"`
	Groups       []GroupEntry  `json:"groups"`
	PollInterval int           `json:"poll_interval"`
	Phases       []string      `json:"phases,omitempty"`
}

// InitFellowship inserts the singleton fellowship row (id=1).
func InitFellowship(conn *sqlite.Conn, name, mainRepo, baseBranch string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return sqlitex.Execute(conn,
		`INSERT INTO fellowship (id, version, name, main_repo, base_branch, created_at)
		 VALUES (1, '1', :name, :main_repo, :base_branch, :now)
		 ON CONFLICT(id) DO UPDATE SET
		   name=:name, main_repo=:main_repo, base_branch=:base_branch`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":name":        name,
				":main_repo":   mainRepo,
				":base_branch": baseBranch,
				":now":         now,
			},
		})
}

// LoadFellowship assembles a FellowshipState from the fellowship, fellowship_quests,
// fellowship_scouts, companies, and company_members tables (unchanged table
// names — see the group package for the renamed concept).
func LoadFellowship(conn *sqlite.Conn) (*FellowshipState, error) {
	var fs FellowshipState
	var found bool

	err := sqlitex.Execute(conn,
		`SELECT version, name, main_repo, base_branch, created_at FROM fellowship WHERE id = 1`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				fs.Version = stmt.ColumnInt(0)
				fs.Name = stmt.ColumnText(1)
				fs.MainRepo = stmt.ColumnText(2)
				fs.BaseBranch = stmt.ColumnText(3)
				fs.CreatedAt = stmt.ColumnText(4)
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("dashboard: load fellowship: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("dashboard: fellowship not initialized")
	}

	// Load quests
	fs.Quests = []QuestEntry{}
	err = sqlitex.Execute(conn,
		`SELECT name, task_description, worktree, branch, task_id, status FROM fellowship_quests`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				fs.Quests = append(fs.Quests, QuestEntry{
					Name:            stmt.ColumnText(0),
					TaskDescription: stmt.ColumnText(1),
					Worktree:        stmt.ColumnText(2),
					Branch:          stmt.ColumnText(3),
					TaskID:          stmt.ColumnText(4),
					Status:          stmt.ColumnText(5),
				})
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("dashboard: load quests: %w", err)
	}

	// Load scouts
	fs.Scouts = []ScoutEntry{}
	err = sqlitex.Execute(conn,
		`SELECT name, question, task_id FROM fellowship_scouts`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				fs.Scouts = append(fs.Scouts, ScoutEntry{
					Name:     stmt.ColumnText(0),
					Question: stmt.ColumnText(1),
					TaskID:   stmt.ColumnText(2),
				})
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("dashboard: load scouts: %w", err)
	}

	// Load groups with members
	fs.Groups = []GroupEntry{}
	groupMap := make(map[string]*GroupEntry)

	err = sqlitex.Execute(conn,
		`SELECT name FROM companies`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				name := stmt.ColumnText(0)
				entry := GroupEntry{
					Name:   name,
					Quests: []string{},
					Scouts: []string{},
				}
				fs.Groups = append(fs.Groups, entry)
				groupMap[name] = &fs.Groups[len(fs.Groups)-1]
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("dashboard: load groups: %w", err)
	}

	err = sqlitex.Execute(conn,
		`SELECT company_name, member_name, member_type FROM company_members`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				groupName := stmt.ColumnText(0)
				memberName := stmt.ColumnText(1)
				memberType := stmt.ColumnText(2)
				if c, ok := groupMap[groupName]; ok {
					switch memberType {
					case "quest":
						c.Quests = append(c.Quests, memberName)
					case "scout":
						c.Scouts = append(c.Scouts, memberName)
					}
				}
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("dashboard: load group members: %w", err)
	}

	return &fs, nil
}

// SaveFellowship updates the fellowship singleton and upserts all quests, scouts, and groups.
func SaveFellowship(conn *sqlite.Conn, fs *FellowshipState) error {
	// Update fellowship singleton
	if err := sqlitex.Execute(conn,
		`UPDATE fellowship SET version=:version, name=:name, main_repo=:main_repo,
		 base_branch=:base_branch WHERE id = 1`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":version":     fmt.Sprintf("%d", fs.Version),
				":name":        fs.Name,
				":main_repo":   fs.MainRepo,
				":base_branch": fs.BaseBranch,
			},
		}); err != nil {
		return fmt.Errorf("dashboard: update fellowship: %w", err)
	}

	// Sync quests: delete removed, upsert current
	if err := sqlitex.Execute(conn, `DELETE FROM fellowship_quests`, nil); err != nil {
		return fmt.Errorf("dashboard: clear quests: %w", err)
	}
	for _, q := range fs.Quests {
		if err := upsertQuest(conn, q); err != nil {
			return err
		}
	}

	// Sync scouts
	if err := sqlitex.Execute(conn, `DELETE FROM fellowship_scouts`, nil); err != nil {
		return fmt.Errorf("dashboard: clear scouts: %w", err)
	}
	for _, s := range fs.Scouts {
		if err := upsertScout(conn, s); err != nil {
			return err
		}
	}

	// Sync groups
	if err := sqlitex.Execute(conn, `DELETE FROM company_members`, nil); err != nil {
		return fmt.Errorf("dashboard: clear group members: %w", err)
	}
	if err := sqlitex.Execute(conn, `DELETE FROM companies`, nil); err != nil {
		return fmt.Errorf("dashboard: clear groups: %w", err)
	}
	for _, c := range fs.Groups {
		if err := addGroupInternal(conn, c.Name, c.Quests, c.Scouts); err != nil {
			return err
		}
	}

	return nil
}

// AddQuest inserts a quest into fellowship_quests.
func AddQuest(conn *sqlite.Conn, q QuestEntry) error {
	return upsertQuest(conn, q)
}

func upsertQuest(conn *sqlite.Conn, q QuestEntry) error {
	status := q.Status
	if status == "" {
		status = "active"
	}
	// Store the resolved path: hooks look quests up by the git top-level, which
	// is always absolute and symlink-free.
	q.Worktree = state.CanonicalWorktree(q.Worktree)

	// fellowship_quests.worktree carries a UNIQUE index (schema v2), so
	// re-registering a worktree under a new or renamed quest would otherwise
	// fail the INSERT with a constraint violation the ON CONFLICT(name)
	// clause doesn't cover (that only dedupes on name). If another row
	// already holds this worktree, clear its worktree first so the upsert
	// below can proceed — the previous holder keeps its quest row, just
	// without a worktree, and re-registering under the same name is just an
	// update in place with nothing to clear.
	if q.Worktree != "" {
		var previousHolder string
		if err := sqlitex.Execute(conn,
			`SELECT name FROM fellowship_quests WHERE worktree = :wt AND name != :name`,
			&sqlitex.ExecOptions{
				Named: map[string]any{":wt": q.Worktree, ":name": q.Name},
				ResultFunc: func(stmt *sqlite.Stmt) error {
					previousHolder = stmt.ColumnText(0)
					return nil
				},
			}); err != nil {
			return fmt.Errorf("dashboard: checking worktree conflict for %s: %w", q.Name, err)
		}
		if previousHolder != "" {
			if err := sqlitex.Execute(conn,
				`UPDATE fellowship_quests SET worktree = '' WHERE name = :name`,
				&sqlitex.ExecOptions{Named: map[string]any{":name": previousHolder}}); err != nil {
				return fmt.Errorf("dashboard: clearing worktree from %s: %w", previousHolder, err)
			}
			fmt.Printf("Note: worktree %q was registered to quest %q; reassigning it to %q\n",
				q.Worktree, previousHolder, q.Name)
		}
	}

	return sqlitex.Execute(conn,
		`INSERT INTO fellowship_quests (name, task_description, worktree, branch, task_id, status)
		 VALUES (:name, :desc, :wt, :branch, :task_id, :status)
		 ON CONFLICT(name) DO UPDATE SET
		   task_description=:desc, worktree=:wt, branch=:branch, task_id=:task_id, status=:status`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":name":    q.Name,
				":desc":    q.TaskDescription,
				":wt":      q.Worktree,
				":branch":  q.Branch,
				":task_id": q.TaskID,
				":status":  status,
			},
		})
}

// UpdateQuest updates specific fields on a quest by name.
func UpdateQuest(conn *sqlite.Conn, name string, updates map[string]any) error {
	// Build SET clause from allowed fields
	allowed := map[string]string{
		"task_description": "task_description",
		"worktree":         "worktree",
		"branch":           "branch",
		"task_id":          "task_id",
		"status":           "status",
	}
	setClauses := ""
	named := map[string]any{":name": name}
	for k, v := range updates {
		col, ok := allowed[k]
		if !ok {
			continue
		}
		if setClauses != "" {
			setClauses += ", "
		}
		param := ":" + k
		setClauses += col + "=" + param
		if k == "worktree" {
			if wt, isStr := v.(string); isStr {
				v = state.CanonicalWorktree(wt)
			}
		}
		named[param] = v
	}
	if setClauses == "" {
		return nil
	}
	return sqlitex.Execute(conn,
		`UPDATE fellowship_quests SET `+setClauses+` WHERE name = :name`,
		&sqlitex.ExecOptions{Named: named})
}

// RemoveQuest deletes a quest by name.
func RemoveQuest(conn *sqlite.Conn, name string) error {
	return sqlitex.Execute(conn,
		`DELETE FROM fellowship_quests WHERE name = :name`,
		&sqlitex.ExecOptions{Named: map[string]any{":name": name}})
}

// AddScout inserts a scout into fellowship_scouts.
func AddScout(conn *sqlite.Conn, s ScoutEntry) error {
	return upsertScout(conn, s)
}

func upsertScout(conn *sqlite.Conn, s ScoutEntry) error {
	return sqlitex.Execute(conn,
		`INSERT INTO fellowship_scouts (name, question, task_id)
		 VALUES (:name, :question, :task_id)
		 ON CONFLICT(name) DO UPDATE SET question=:question, task_id=:task_id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":name":     s.Name,
				":question": s.Question,
				":task_id":  s.TaskID,
			},
		})
}

// RemoveScout deletes a scout by name.
func RemoveScout(conn *sqlite.Conn, name string) error {
	return sqlitex.Execute(conn,
		`DELETE FROM fellowship_scouts WHERE name = :name`,
		&sqlitex.ExecOptions{Named: map[string]any{":name": name}})
}

// AddGroup inserts a group with its quest and scout members.
func AddGroup(conn *sqlite.Conn, name string, quests []string, scouts []string) error {
	return addGroupInternal(conn, name, quests, scouts)
}

func addGroupInternal(conn *sqlite.Conn, name string, quests []string, scouts []string) error {
	if err := sqlitex.Execute(conn,
		`INSERT INTO companies (name) VALUES (:name) ON CONFLICT(name) DO NOTHING`,
		&sqlitex.ExecOptions{Named: map[string]any{":name": name}}); err != nil {
		return fmt.Errorf("dashboard: add group %s: %w", name, err)
	}
	for _, q := range quests {
		if err := sqlitex.Execute(conn,
			`INSERT INTO company_members (company_name, member_name, member_type)
			 VALUES (:group, :member, 'quest')
			 ON CONFLICT DO NOTHING`,
			&sqlitex.ExecOptions{
				Named: map[string]any{":group": name, ":member": q},
			}); err != nil {
			return fmt.Errorf("dashboard: add group member %s/%s: %w", name, q, err)
		}
	}
	for _, s := range scouts {
		if err := sqlitex.Execute(conn,
			`INSERT INTO company_members (company_name, member_name, member_type)
			 VALUES (:group, :member, 'scout')
			 ON CONFLICT DO NOTHING`,
			&sqlitex.ExecOptions{
				Named: map[string]any{":group": name, ":member": s},
			}); err != nil {
			return fmt.Errorf("dashboard: add group member %s/%s: %w", name, s, err)
		}
	}
	return nil
}

// ListQuests returns all quests from fellowship_quests.
func ListQuests(conn *sqlite.Conn) ([]QuestEntry, error) {
	var quests []QuestEntry
	err := sqlitex.Execute(conn,
		`SELECT name, task_description, worktree, branch, task_id, status FROM fellowship_quests`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				quests = append(quests, QuestEntry{
					Name:            stmt.ColumnText(0),
					TaskDescription: stmt.ColumnText(1),
					Worktree:        stmt.ColumnText(2),
					Branch:          stmt.ColumnText(3),
					TaskID:          stmt.ColumnText(4),
					Status:          stmt.ColumnText(5),
				})
				return nil
			},
		})
	return quests, err
}

// ListScouts returns all scouts from fellowship_scouts.
func ListScouts(conn *sqlite.Conn) ([]ScoutEntry, error) {
	var scouts []ScoutEntry
	err := sqlitex.Execute(conn,
		`SELECT name, question, task_id FROM fellowship_scouts`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				scouts = append(scouts, ScoutEntry{
					Name:     stmt.ColumnText(0),
					Question: stmt.ColumnText(1),
					TaskID:   stmt.ColumnText(2),
				})
				return nil
			},
		})
	return scouts, err
}

// ListGroups returns all groups with their members.
func ListGroups(conn *sqlite.Conn) ([]GroupEntry, error) {
	var groups []GroupEntry
	groupMap := make(map[string]*GroupEntry)

	err := sqlitex.Execute(conn,
		`SELECT name FROM companies`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				name := stmt.ColumnText(0)
				groups = append(groups, GroupEntry{
					Name:   name,
					Quests: []string{},
					Scouts: []string{},
				})
				groupMap[name] = &groups[len(groups)-1]
				return nil
			},
		})
	if err != nil {
		return nil, err
	}

	err = sqlitex.Execute(conn,
		`SELECT company_name, member_name, member_type FROM company_members`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				groupName := stmt.ColumnText(0)
				memberName := stmt.ColumnText(1)
				memberType := stmt.ColumnText(2)
				if c, ok := groupMap[groupName]; ok {
					switch memberType {
					case "quest":
						c.Quests = append(c.Quests, memberName)
					case "scout":
						c.Scouts = append(c.Scouts, memberName)
					}
				}
				return nil
			},
		})
	if err != nil {
		return nil, err
	}

	return groups, nil
}

// DiscoverQuests queries the DB for fellowship state joined with quest_state for
// phase/gate status. If no fellowship row exists, returns an empty status.
func DiscoverQuests(conn *sqlite.Conn) (*DashboardStatus, error) {
	fs, err := LoadFellowship(conn)
	if err != nil {
		// No fellowship row — return empty status
		return &DashboardStatus{
			Quests: []QuestStatus{},
			Scouts: []ScoutEntry{},
			Groups: []GroupEntry{},
		}, nil
	}

	status := &DashboardStatus{
		Name:   fs.Name,
		Quests: []QuestStatus{},
		Scouts: fs.Scouts,
		Groups: fs.Groups,
	}
	if status.Scouts == nil {
		status.Scouts = []ScoutEntry{}
	}
	if status.Groups == nil {
		status.Groups = []GroupEntry{}
	}

	for _, q := range fs.Quests {
		entryStatus := QuestEntryStatus(q)

		// Try to load quest state from DB
		qs, loadErr := loadQuestStatusFromDB(conn, q.Name, q.Worktree)
		if loadErr != nil {
			// Quest state not in DB — show completed/cancelled as synthetic entries
			if synth, ok := TerminalQuestStatus(q.Name, q.Worktree, entryStatus); ok {
				status.Quests = append(status.Quests, synth)
			}
			continue
		}
		qs.Status = entryStatus
		status.Quests = append(status.Quests, *qs)
	}

	return status, nil
}

// loadQuestStatusFromDB loads a single quest's status from the quest_state table.
func loadQuestStatusFromDB(conn *sqlite.Conn, name, worktree string) (*QuestStatus, error) {
	s, err := state.Load(conn, name)
	if err != nil {
		return nil, err
	}
	done, total, _ := todo.Progress(conn, name)
	return &QuestStatus{
		Name:            name,
		Worktree:        worktree,
		Phase:           s.Phase,
		GatePending:     s.GatePending,
		GateID:          s.GateID,
		LembasCompleted: s.LembasCompleted,
		MetadataUpdated: s.MetadataUpdated,
		TodosDone:       done,
		TodosTotal:      total,
	}, nil
}

// TerminalQuestStatus builds the synthetic QuestStatus callers fall back to
// when a quest has no quest_state row (e.g. history-only or pre-phase-
// machinery entries) but its fellowship entry status is terminal. ok is
// false — and the QuestStatus zero — when entryStatus isn't "completed" or
// "cancelled", meaning no synthetic entry should be recorded.
//
// Both DiscoverQuests and group.LoadDetail hit this case and must agree on
// it, since CalculateProgress relies on the same terminal-phase convention
// to count these quests toward group progress.
func TerminalQuestStatus(name, worktree, entryStatus string) (QuestStatus, bool) {
	if entryStatus != "completed" && entryStatus != "cancelled" {
		return QuestStatus{}, false
	}
	return QuestStatus{
		Name:     name,
		Worktree: worktree,
		Phase:    state.TerminalPhase,
		Status:   entryStatus,
	}, true
}
