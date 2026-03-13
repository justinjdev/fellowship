package errand

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type ErrandStatus string

const (
	Pending    ErrandStatus = "pending"
	InProgress ErrandStatus = "in_progress"
	Done       ErrandStatus = "done"
	Blocked    ErrandStatus = "blocked"
	Skipped    ErrandStatus = "skipped"
)

type Errand struct {
	ID          string       `json:"id"`
	Description string       `json:"description"`
	Status      ErrandStatus `json:"status"`
	Phase       string       `json:"phase,omitempty"`
	DependsOn   []string     `json:"depends_on,omitempty"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
}

type QuestErrandList struct {
	QuestName string   `json:"quest_name"`
	Task      string   `json:"task"`
	Items     []Errand `json:"items"`
}

// ValidStatus checks whether a string is a valid ErrandStatus.
func ValidStatus(s string) (ErrandStatus, bool) {
	switch ErrandStatus(s) {
	case Pending, InProgress, Done, Blocked, Skipped:
		return ErrandStatus(s), true
	default:
		return "", false
	}
}

// Init creates the initial errand list metadata for a quest.
// This is a no-op for DB-backed storage since errands reference quest_state via FK.
func Init(conn *sqlite.Conn, quest, task string) error {
	// errands are stored per-row with quest_name FK; nothing to initialize.
	_ = conn
	_ = quest
	_ = task
	return nil
}

// Add inserts a new errand and returns its generated ID (w-NNN).
func Add(conn *sqlite.Conn, quest, desc, phase string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Generate next ID using MAX to handle gaps from deletions.
	var nextNum int
	err := sqlitex.Execute(conn,
		`SELECT COALESCE(MAX(CAST(SUBSTR(id, 3) AS INTEGER)), 0) + 1 FROM errands WHERE quest_name = :quest`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": quest},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				nextNum = stmt.ColumnInt(0)
				return nil
			},
		})
	if err != nil {
		return "", fmt.Errorf("errand: next id: %w", err)
	}

	id := fmt.Sprintf("w-%03d", nextNum)

	err = sqlitex.Execute(conn,
		`INSERT INTO errands (id, quest_name, description, status, phase, created_at, updated_at)
		 VALUES (:id, :quest, :desc, :status, :phase, :now, :now)`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":id":     id,
				":quest":  quest,
				":desc":   desc,
				":status": string(Pending),
				":phase":  phase,
				":now":    now,
			},
		})
	if err != nil {
		return "", fmt.Errorf("errand: add: %w", err)
	}

	return id, nil
}

// UpdateStatus changes the status of an errand.
func UpdateStatus(conn *sqlite.Conn, quest, id string, status ErrandStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)

	err := sqlitex.Execute(conn,
		`UPDATE errands SET status = :status, updated_at = :now
		 WHERE quest_name = :quest AND id = :id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":status": string(status),
				":now":    now,
				":quest":  quest,
				":id":     id,
			},
		})
	if err != nil {
		return fmt.Errorf("errand: update status: %w", err)
	}

	if conn.Changes() == 0 {
		return fmt.Errorf("errand %q not found in quest %q", id, quest)
	}
	return nil
}

// List returns all errands for a quest, ordered by ID.
func List(conn *sqlite.Conn, quest string) ([]Errand, error) {
	var items []Errand
	err := sqlitex.Execute(conn,
		`SELECT id, description, status, phase, created_at, updated_at
		 FROM errands WHERE quest_name = :quest ORDER BY id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": quest},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				e := Errand{
					ID:          stmt.ColumnText(0),
					Description: stmt.ColumnText(1),
					Status:      ErrandStatus(stmt.ColumnText(2)),
					Phase:       stmt.ColumnText(3),
					CreatedAt:   stmt.ColumnText(4),
					UpdatedAt:   stmt.ColumnText(5),
				}
				items = append(items, e)
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("errand: list: %w", err)
	}

	// Load dependencies for each errand.
	for i := range items {
		deps, err := loadDeps(conn, quest, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].DependsOn = deps
	}

	return items, nil
}

// Progress returns the count of done errands and total errands for a quest.
func Progress(conn *sqlite.Conn, quest string) (done, total int, err error) {
	err = sqlitex.Execute(conn,
		`SELECT COUNT(*) AS total, SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END) AS done
		 FROM errands WHERE quest_name = :quest`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": quest},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				total = stmt.ColumnInt(0)
				done = stmt.ColumnInt(1)
				return nil
			},
		})
	if err != nil {
		err = fmt.Errorf("errand: progress: %w", err)
	}
	return
}

// PendingErrands returns errands that are pending or blocked but whose
// dependencies are all done.
func PendingErrands(conn *sqlite.Conn, quest string) ([]Errand, error) {
	items, err := List(conn, quest)
	if err != nil {
		return nil, err
	}

	doneSet := make(map[string]bool)
	for _, item := range items {
		if item.Status == Done {
			doneSet[item.ID] = true
		}
	}

	var result []Errand
	for _, item := range items {
		if item.Status != Pending && item.Status != Blocked {
			continue
		}
		depsOK := true
		for _, dep := range item.DependsOn {
			if !doneSet[dep] {
				depsOK = false
				break
			}
		}
		if depsOK {
			result = append(result, item)
		}
	}
	return result, nil
}

// loadDeps returns the dependency IDs for an errand.
func loadDeps(conn *sqlite.Conn, quest, errandID string) ([]string, error) {
	var deps []string
	err := sqlitex.Execute(conn,
		`SELECT depends_on FROM errand_deps WHERE quest_name = :quest AND errand_id = :id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": quest, ":id": errandID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				deps = append(deps, stmt.ColumnText(0))
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("errand: load deps: %w", err)
	}
	return deps, nil
}
