package dashboard

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type CompanyEntry struct {
	Name   string   `json:"name"`
	Quests []string `json:"quests"` // quest names
	Scouts []string `json:"scouts"` // scout names
}

type FellowshipState struct {
	Version    int            `json:"version"`
	Name       string         `json:"name"`
	CreatedAt  string         `json:"created_at"`
	MainRepo   string         `json:"main_repo"`
	BaseBranch string         `json:"base_branch,omitempty"`
	Quests     []QuestEntry   `json:"quests"`
	Scouts     []ScoutEntry   `json:"scouts"`
	Companies  []CompanyEntry `json:"companies"`
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
	ErrandsDone     int     `json:"errands_done"`
	ErrandsTotal    int     `json:"errands_total"`
}

type DashboardStatus struct {
	Name         string         `json:"name"`
	Quests       []QuestStatus  `json:"quests"`
	Scouts       []ScoutEntry   `json:"scouts"`
	Companies    []CompanyEntry `json:"companies"`
	PollInterval int            `json:"poll_interval"`
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
// fellowship_scouts, companies, and company_members tables.
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

	// Load companies with members
	fs.Companies = []CompanyEntry{}
	companyMap := make(map[string]*CompanyEntry)

	err = sqlitex.Execute(conn,
		`SELECT name FROM companies`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				name := stmt.ColumnText(0)
				entry := CompanyEntry{
					Name:   name,
					Quests: []string{},
					Scouts: []string{},
				}
				fs.Companies = append(fs.Companies, entry)
				companyMap[name] = &fs.Companies[len(fs.Companies)-1]
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("dashboard: load companies: %w", err)
	}

	err = sqlitex.Execute(conn,
		`SELECT company_name, member_name, member_type FROM company_members`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				companyName := stmt.ColumnText(0)
				memberName := stmt.ColumnText(1)
				memberType := stmt.ColumnText(2)
				if c, ok := companyMap[companyName]; ok {
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
		return nil, fmt.Errorf("dashboard: load company members: %w", err)
	}

	return &fs, nil
}

// SaveFellowship updates the fellowship singleton and upserts all quests, scouts, and companies.
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

	// Sync companies
	if err := sqlitex.Execute(conn, `DELETE FROM company_members`, nil); err != nil {
		return fmt.Errorf("dashboard: clear company members: %w", err)
	}
	if err := sqlitex.Execute(conn, `DELETE FROM companies`, nil); err != nil {
		return fmt.Errorf("dashboard: clear companies: %w", err)
	}
	for _, c := range fs.Companies {
		if err := addCompanyInternal(conn, c.Name, c.Quests, c.Scouts); err != nil {
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

// AddCompany inserts a company with its quest and scout members.
func AddCompany(conn *sqlite.Conn, name string, quests []string, scouts []string) error {
	return addCompanyInternal(conn, name, quests, scouts)
}

func addCompanyInternal(conn *sqlite.Conn, name string, quests []string, scouts []string) error {
	if err := sqlitex.Execute(conn,
		`INSERT INTO companies (name) VALUES (:name) ON CONFLICT(name) DO NOTHING`,
		&sqlitex.ExecOptions{Named: map[string]any{":name": name}}); err != nil {
		return fmt.Errorf("dashboard: add company %s: %w", name, err)
	}
	for _, q := range quests {
		if err := sqlitex.Execute(conn,
			`INSERT INTO company_members (company_name, member_name, member_type)
			 VALUES (:company, :member, 'quest')
			 ON CONFLICT DO NOTHING`,
			&sqlitex.ExecOptions{
				Named: map[string]any{":company": name, ":member": q},
			}); err != nil {
			return fmt.Errorf("dashboard: add company member %s/%s: %w", name, q, err)
		}
	}
	for _, s := range scouts {
		if err := sqlitex.Execute(conn,
			`INSERT INTO company_members (company_name, member_name, member_type)
			 VALUES (:company, :member, 'scout')
			 ON CONFLICT DO NOTHING`,
			&sqlitex.ExecOptions{
				Named: map[string]any{":company": name, ":member": s},
			}); err != nil {
			return fmt.Errorf("dashboard: add company member %s/%s: %w", name, s, err)
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

// ListCompanies returns all companies with their members.
func ListCompanies(conn *sqlite.Conn) ([]CompanyEntry, error) {
	var companies []CompanyEntry
	companyMap := make(map[string]*CompanyEntry)

	err := sqlitex.Execute(conn,
		`SELECT name FROM companies`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				name := stmt.ColumnText(0)
				companies = append(companies, CompanyEntry{
					Name:   name,
					Quests: []string{},
					Scouts: []string{},
				})
				companyMap[name] = &companies[len(companies)-1]
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
				companyName := stmt.ColumnText(0)
				memberName := stmt.ColumnText(1)
				memberType := stmt.ColumnText(2)
				if c, ok := companyMap[companyName]; ok {
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

	return companies, nil
}

// DiscoverQuests queries the DB for fellowship state joined with quest_state for
// phase/gate status. If no fellowship row exists, returns an empty status.
func DiscoverQuests(conn *sqlite.Conn) (*DashboardStatus, error) {
	fs, err := LoadFellowship(conn)
	if err != nil {
		// No fellowship row — return empty status
		return &DashboardStatus{
			Quests:    []QuestStatus{},
			Scouts:    []ScoutEntry{},
			Companies: []CompanyEntry{},
		}, nil
	}

	status := &DashboardStatus{
		Name:      fs.Name,
		Quests:    []QuestStatus{},
		Scouts:    fs.Scouts,
		Companies: fs.Companies,
	}
	if status.Scouts == nil {
		status.Scouts = []ScoutEntry{}
	}
	if status.Companies == nil {
		status.Companies = []CompanyEntry{}
	}

	for _, q := range fs.Quests {
		entryStatus := QuestEntryStatus(q)

		// Try to load quest state from DB
		qs, loadErr := loadQuestStatusFromDB(conn, q.Name, q.Worktree)
		if loadErr != nil {
			// Quest state not in DB — show completed/cancelled as synthetic entries
			if entryStatus == "completed" || entryStatus == "cancelled" {
				status.Quests = append(status.Quests, QuestStatus{
					Name:     q.Name,
					Worktree: q.Worktree,
					Phase:    "Complete",
					Status:   entryStatus,
				})
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
	done, total, _ := errand.Progress(conn, name)
	return &QuestStatus{
		Name:            name,
		Worktree:        worktree,
		Phase:           s.Phase,
		GatePending:     s.GatePending,
		GateID:          s.GateID,
		LembasCompleted: s.LembasCompleted,
		MetadataUpdated: s.MetadataUpdated,
		ErrandsDone:     done,
		ErrandsTotal:    total,
	}, nil
}
