package autopsy

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// DefaultExpiryDays is the default autopsy TTL when not configured.
const DefaultExpiryDays = 90

// Autopsy represents a structured failure record.
type Autopsy struct {
	ID         int64    `json:"id"`
	Timestamp  string   `json:"ts"`
	Quest      string   `json:"quest"`
	Task       string   `json:"task"`
	Phase      string   `json:"phase"`
	Trigger    string   `json:"trigger"` // "recovery", "rejection", "abandonment"
	Files      []string `json:"files"`
	Modules    []string `json:"modules"`
	WhatFailed string   `json:"what_failed"`
	Resolution string   `json:"resolution,omitempty"`
	Tags       []string `json:"tags"`
	ExpiresAt  string   `json:"expires_at"`
}

// CreateInput is the subset of fields the caller provides; timestamp and expiry are filled in.
type CreateInput struct {
	Quest      string   `json:"quest"`
	Task       string   `json:"task"`
	Phase      string   `json:"phase"`
	Trigger    string   `json:"trigger"`
	Files      []string `json:"files"`
	Modules    []string `json:"modules"`
	WhatFailed string   `json:"what_failed"`
	Resolution string   `json:"resolution,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// ScanOptions configures which autopsies to match.
type ScanOptions struct {
	Files   []string
	Modules []string
	Tags    []string
}

var validTriggers = map[string]bool{
	"recovery":    true,
	"rejection":   true,
	"abandonment": true,
}

// Create validates input, inserts the autopsy and its related files/modules/tags into the DB,
// and returns the row ID.
func Create(conn *sqlite.Conn, input *CreateInput) (int64, error) {
	if input == nil {
		return 0, fmt.Errorf("input is required")
	}
	if input.Quest == "" {
		return 0, fmt.Errorf("quest is required")
	}
	if input.WhatFailed == "" {
		return 0, fmt.Errorf("what_failed is required")
	}
	if !validTriggers[input.Trigger] {
		return 0, fmt.Errorf("invalid trigger %q (must be recovery, rejection, or abandonment)", input.Trigger)
	}

	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	expiresAt := now.AddDate(0, 0, DefaultExpiryDays).Format(time.RFC3339)

	err := sqlitex.Execute(conn,
		`INSERT INTO autopsies (timestamp, quest, task, phase, trigger_type, what_failed, resolution, expires_at)
		 VALUES (:ts, :quest, :task, :phase, :trigger, :what_failed, :resolution, :expires_at)`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":ts":          timestamp,
				":quest":       input.Quest,
				":task":        input.Task,
				":phase":       input.Phase,
				":trigger":     input.Trigger,
				":what_failed": input.WhatFailed,
				":resolution":  input.Resolution,
				":expires_at":  expiresAt,
			},
		})
	if err != nil {
		return 0, fmt.Errorf("autopsy: insert: %w", err)
	}

	id := conn.LastInsertRowID()

	for _, f := range input.Files {
		if err := sqlitex.Execute(conn,
			`INSERT INTO autopsy_files (autopsy_id, file_path) VALUES (?, ?)`,
			&sqlitex.ExecOptions{
				Args: []any{id, f},
			}); err != nil {
			return 0, fmt.Errorf("autopsy: insert file: %w", err)
		}
	}

	for _, m := range input.Modules {
		if err := sqlitex.Execute(conn,
			`INSERT INTO autopsy_modules (autopsy_id, module) VALUES (?, ?)`,
			&sqlitex.ExecOptions{
				Args: []any{id, m},
			}); err != nil {
			return 0, fmt.Errorf("autopsy: insert module: %w", err)
		}
	}

	for _, tag := range input.Tags {
		if err := sqlitex.Execute(conn,
			`INSERT INTO autopsy_tags (autopsy_id, tag) VALUES (?, ?)`,
			&sqlitex.ExecOptions{
				Args: []any{id, tag},
			}); err != nil {
			return 0, fmt.Errorf("autopsy: insert tag: %w", err)
		}
	}

	return id, nil
}

// Scan queries autopsies from the DB, filtering by files/modules/tags and excluding expired entries.
func Scan(conn *sqlite.Conn, opts ScanOptions, expiryDays int) ([]Autopsy, error) {
	if len(opts.Files) == 0 && len(opts.Modules) == 0 && len(opts.Tags) == 0 {
		return nil, fmt.Errorf("at least one of --files, --modules, or --tags is required")
	}

	// Build a query that joins across the junction tables.
	// We select all non-expired autopsies that match any of the filter criteria.
	var conditions []string
	var args []any

	if len(opts.Files) > 0 {
		var fileCondParts []string
		for _, f := range opts.Files {
			// Exact match
			args = append(args, f)
			fileCondParts = append(fileCondParts, "af.file_path = ?")

			// Same directory match (for files with directories)
			dir := filepath.Dir(filepath.ToSlash(f))
			if dir != "." {
				escaped := strings.ReplaceAll(strings.ReplaceAll(dir, "%", "\\%"), "_", "\\_")
				args = append(args, escaped+"/%")
				fileCondParts = append(fileCondParts, "af.file_path LIKE ? ESCAPE '\\'")
			}

			// Query file is under a directory prefix in the autopsy
			if strings.HasSuffix(f, "/") {
				escaped := strings.ReplaceAll(strings.ReplaceAll(f, "%", "\\%"), "_", "\\_")
				args = append(args, escaped+"%")
				fileCondParts = append(fileCondParts, "af.file_path LIKE ? ESCAPE '\\'")
			}
		}
		conditions = append(conditions,
			fmt.Sprintf("a.id IN (SELECT af.autopsy_id FROM autopsy_files af WHERE %s)",
				strings.Join(fileCondParts, " OR ")))
	}

	if len(opts.Modules) > 0 {
		placeholders := make([]string, len(opts.Modules))
		for i, m := range opts.Modules {
			placeholders[i] = "?"
			args = append(args, m)
		}
		conditions = append(conditions,
			fmt.Sprintf("a.id IN (SELECT am.autopsy_id FROM autopsy_modules am WHERE am.module IN (%s))",
				strings.Join(placeholders, ",")))
	}

	if len(opts.Tags) > 0 {
		placeholders := make([]string, len(opts.Tags))
		for i, tag := range opts.Tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions,
			fmt.Sprintf("a.id IN (SELECT at2.autopsy_id FROM autopsy_tags at2 WHERE at2.tag IN (%s))",
				strings.Join(placeholders, ",")))
	}

	query := fmt.Sprintf(
		`SELECT a.id, a.timestamp, a.quest, a.task, a.phase, a.trigger_type,
		        a.what_failed, a.resolution, a.expires_at
		 FROM autopsies a
		 WHERE datetime(a.expires_at) > datetime('now')
		   AND (%s)
		 ORDER BY a.timestamp DESC`,
		strings.Join(conditions, " OR "))

	var autopsies []Autopsy
	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			a := Autopsy{
				ID:         stmt.ColumnInt64(0),
				Timestamp:  stmt.ColumnText(1),
				Quest:      stmt.ColumnText(2),
				Task:       stmt.ColumnText(3),
				Phase:      stmt.ColumnText(4),
				Trigger:    stmt.ColumnText(5),
				WhatFailed: stmt.ColumnText(6),
				Resolution: stmt.ColumnText(7),
				ExpiresAt:  stmt.ColumnText(8),
			}
			autopsies = append(autopsies, a)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("autopsy: scan: %w", err)
	}

	// Load files, modules, and tags for each autopsy
	for i := range autopsies {
		if err := loadAutopsyRelations(conn, &autopsies[i]); err != nil {
			return nil, err
		}
	}

	if autopsies == nil {
		autopsies = []Autopsy{}
	}
	return autopsies, nil
}

// loadAutopsyRelations populates Files, Modules, and Tags for an autopsy.
func loadAutopsyRelations(conn *sqlite.Conn, a *Autopsy) error {
	// Files
	a.Files = []string{}
	if err := sqlitex.Execute(conn,
		`SELECT file_path FROM autopsy_files WHERE autopsy_id = ? ORDER BY file_path`,
		&sqlitex.ExecOptions{
			Args: []any{a.ID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				a.Files = append(a.Files, stmt.ColumnText(0))
				return nil
			},
		}); err != nil {
		return fmt.Errorf("autopsy: load files: %w", err)
	}

	// Modules
	a.Modules = []string{}
	if err := sqlitex.Execute(conn,
		`SELECT module FROM autopsy_modules WHERE autopsy_id = ? ORDER BY module`,
		&sqlitex.ExecOptions{
			Args: []any{a.ID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				a.Modules = append(a.Modules, stmt.ColumnText(0))
				return nil
			},
		}); err != nil {
		return fmt.Errorf("autopsy: load modules: %w", err)
	}

	// Tags
	a.Tags = []string{}
	if err := sqlitex.Execute(conn,
		`SELECT tag FROM autopsy_tags WHERE autopsy_id = ? ORDER BY tag`,
		&sqlitex.ExecOptions{
			Args: []any{a.ID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				a.Tags = append(a.Tags, stmt.ColumnText(0))
				return nil
			},
		}); err != nil {
		return fmt.Errorf("autopsy: load tags: %w", err)
	}

	return nil
}

// Infer reconstructs a best-effort autopsy from quest DB state.
// It queries fellowship_quests for respawns/status, quest_gates for rejections,
// quest_phases for phase history, and quest_files for files touched.
func Infer(conn *sqlite.Conn, questName string) (int64, error) {
	// Load quest info from fellowship_quests
	var status string
	var respawns int
	var taskDesc string
	found := false
	err := sqlitex.Execute(conn,
		`SELECT status, respawns, COALESCE(task_description, '') FROM fellowship_quests WHERE name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				status = stmt.ColumnText(0)
				respawns = stmt.ColumnInt(1)
				taskDesc = stmt.ColumnText(2)
				found = true
				return nil
			},
		})
	if err != nil {
		return 0, fmt.Errorf("autopsy: query quest: %w", err)
	}
	if !found {
		return 0, fmt.Errorf("quest %q not found", questName)
	}

	// Determine trigger
	trigger, whatFailed, err := inferTriggerFromDB(conn, questName, status, respawns)
	if err != nil {
		return 0, err
	}
	if trigger == "" {
		return 0, fmt.Errorf("no failure signals found for quest %q", questName)
	}

	// Get phase from quest_phases (last completed phase)
	phase := "unknown"
	err = sqlitex.Execute(conn,
		`SELECT phase FROM quest_phases WHERE quest_name = :name ORDER BY completed_at DESC LIMIT 1`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				phase = stmt.ColumnText(0)
				return nil
			},
		})
	if err != nil {
		return 0, fmt.Errorf("autopsy: query phases: %w", err)
	}

	// Get files touched from quest_files
	var files []string
	err = sqlitex.Execute(conn,
		`SELECT file_path FROM quest_files WHERE quest_name = :name ORDER BY file_path`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				files = append(files, stmt.ColumnText(0))
				return nil
			},
		})
	if err != nil {
		return 0, fmt.Errorf("autopsy: query files: %w", err)
	}
	if files == nil {
		files = []string{}
	}

	modules := inferModules(files)

	input := &CreateInput{
		Quest:      questName,
		Task:       taskDesc,
		Phase:      phase,
		Trigger:    trigger,
		Files:      files,
		Modules:    modules,
		WhatFailed: whatFailed,
	}

	return Create(conn, input)
}

// inferTriggerFromDB determines the failure trigger by querying DB tables.
func inferTriggerFromDB(conn *sqlite.Conn, questName, status string, respawns int) (string, string, error) {
	// Check for respawns
	if respawns > 0 {
		return "recovery", fmt.Sprintf("Quest required %d respawn(s)", respawns), nil
	}

	// Check for gate rejections in quest_gates
	var rejectionReason string
	var rejectionPhase string
	err := sqlitex.Execute(conn,
		`SELECT phase, COALESCE(reason, '') FROM quest_gates
		 WHERE quest_name = :name AND action = 'rejected'
		 ORDER BY timestamp DESC LIMIT 1`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				rejectionPhase = stmt.ColumnText(0)
				rejectionReason = stmt.ColumnText(1)
				return nil
			},
		})
	if err != nil {
		return "", "", fmt.Errorf("autopsy: query gates: %w", err)
	}
	if rejectionPhase != "" {
		detail := rejectionReason
		if detail == "" {
			detail = fmt.Sprintf("Gate rejected at %s phase", rejectionPhase)
		}
		return "rejection", detail, nil
	}

	// Check for failed/cancelled status
	if status == "failed" || status == "cancelled" {
		return "abandonment", fmt.Sprintf("Quest %s with status: %s", questName, status), nil
	}

	return "", "", nil
}

// inferModules derives module names from file paths using the first directory component.
func inferModules(files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		if len(parts) >= 2 {
			mod := parts[0]
			if !seen[mod] {
				seen[mod] = true
			}
		}
	}
	modules := make([]string, 0, len(seen))
	for m := range seen {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	return modules
}
