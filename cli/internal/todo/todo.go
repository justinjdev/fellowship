package todo

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type TodoStatus string

const (
	Pending    TodoStatus = "pending"
	InProgress TodoStatus = "in_progress"
	Done       TodoStatus = "done"
	Blocked    TodoStatus = "blocked"
	Skipped    TodoStatus = "skipped"
)

type Todo struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Status      TodoStatus `json:"status"`
	Phase       string     `json:"phase,omitempty"`
	DependsOn   []string   `json:"depends_on,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

type QuestTodoList struct {
	QuestName string `json:"quest_name"`
	Items     []Todo `json:"items"`
}

// allStatuses lists every accepted todo status, in the order shown to users.
var allStatuses = []TodoStatus{Pending, InProgress, Done, Blocked, Skipped}

// Statuses returns the accepted todo status names.
func Statuses() []string {
	out := make([]string, len(allStatuses))
	for i, st := range allStatuses {
		out[i] = string(st)
	}
	return out
}

// ValidStatus checks whether a string is a valid TodoStatus.
func ValidStatus(s string) (TodoStatus, bool) {
	for _, st := range allStatuses {
		if string(st) == s {
			return st, true
		}
	}
	return "", false
}

// Init creates the initial todo list metadata for a quest.
// This is a no-op for DB-backed storage since todos reference quest_state via FK.
//
// It used to also accept a task description to store alongside the list, but
// neither errands nor errand_deps carries a column for it, and adding one is
// a schema change out of scope here — `fellowship todo init` dropped --task
// rather than silently discard it.
func Init(conn *sqlite.Conn, quest string) error {
	// todos are stored per-row with quest_name FK; nothing to initialize.
	_ = conn
	_ = quest
	return nil
}

// Add inserts a new todo and returns its generated ID (w-NNN).
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
		return "", fmt.Errorf("todo: next id: %w", err)
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
		return "", fmt.Errorf("todo: add: %w", err)
	}

	return id, nil
}

// UpdateStatus changes the status of a todo.
func UpdateStatus(conn *sqlite.Conn, quest, id string, status TodoStatus) error {
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
		return fmt.Errorf("todo: update status: %w", err)
	}

	if conn.Changes() == 0 {
		return fmt.Errorf("todo %q not found in quest %q", id, quest)
	}
	return nil
}

// List returns all todos for a quest, ordered by ID. It always returns a
// non-nil slice — `todo show` marshals this straight to JSON, and a quest
// with no todos yet should read as [], not null.
func List(conn *sqlite.Conn, quest string) ([]Todo, error) {
	items := []Todo{}
	err := sqlitex.Execute(conn,
		`SELECT id, description, status, phase, created_at, updated_at
		 FROM errands WHERE quest_name = :quest ORDER BY id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": quest},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				e := Todo{
					ID:          stmt.ColumnText(0),
					Description: stmt.ColumnText(1),
					Status:      TodoStatus(stmt.ColumnText(2)),
					Phase:       stmt.ColumnText(3),
					CreatedAt:   stmt.ColumnText(4),
					UpdatedAt:   stmt.ColumnText(5),
				}
				items = append(items, e)
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("todo: list: %w", err)
	}

	// Load dependencies for each todo.
	for i := range items {
		deps, err := loadDeps(conn, quest, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].DependsOn = deps
	}

	return items, nil
}

// Progress returns the count of done todos and total todos for a quest.
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
		err = fmt.Errorf("todo: progress: %w", err)
	}
	return
}

// PendingTodos returns todos that are pending or blocked but whose
// dependencies are all done.
func PendingTodos(conn *sqlite.Conn, quest string) ([]Todo, error) {
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

	var result []Todo
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

// loadDeps returns the dependency IDs for a todo.
func loadDeps(conn *sqlite.Conn, quest, todoID string) ([]string, error) {
	var deps []string
	err := sqlitex.Execute(conn,
		`SELECT depends_on FROM errand_deps WHERE quest_name = :quest AND errand_id = :id`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":quest": quest, ":id": todoID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				deps = append(deps, stmt.ColumnText(0))
				return nil
			},
		})
	if err != nil {
		return nil, fmt.Errorf("todo: load deps: %w", err)
	}
	return deps, nil
}
