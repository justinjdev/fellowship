package bulletin

import (
	"fmt"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

// Entry represents a single bulletin board discovery.
type Entry struct {
	Timestamp string   `json:"ts"`
	Quest     string   `json:"quest"`
	Topic     string   `json:"topic"`
	Files     []string `json:"files"`
	Discovery string   `json:"discovery"`
}

// Post inserts an entry into the bulletin table and its files into bulletin_files.
func Post(conn *db.Conn, entry Entry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	if err := sqlitex.Execute(conn,
		`INSERT INTO bulletin (timestamp, quest, topic, discovery) VALUES (?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []any{entry.Timestamp, entry.Quest, entry.Topic, entry.Discovery},
		},
	); err != nil {
		return fmt.Errorf("bulletin: post: %w", err)
	}

	id := conn.LastInsertRowID()

	for _, f := range entry.Files {
		if err := sqlitex.Execute(conn,
			`INSERT INTO bulletin_files (bulletin_id, file_path) VALUES (?, ?)`,
			&sqlitex.ExecOptions{
				Args: []any{id, f},
			},
		); err != nil {
			return fmt.Errorf("bulletin: post file %s: %w", f, err)
		}
	}
	return nil
}

// Load reads all bulletin entries from the database, assembling the Files slice
// from the bulletin_files join table.
func Load(conn *db.Conn) ([]Entry, error) {
	// First load all entries.
	type row struct {
		id        int64
		entry     Entry
	}
	var rows []row

	err := sqlitex.Execute(conn,
		`SELECT id, timestamp, quest, topic, discovery FROM bulletin ORDER BY id ASC`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				rows = append(rows, row{
					id: stmt.ColumnInt64(0),
					entry: Entry{
						Timestamp: stmt.ColumnText(1),
						Quest:     stmt.ColumnText(2),
						Topic:     stmt.ColumnText(3),
						Discovery: stmt.ColumnText(4),
					},
				})
				return nil
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("bulletin: load: %w", err)
	}

	if len(rows) == 0 {
		return []Entry{}, nil
	}

	// Build a map for file association.
	idToIdx := make(map[int64]int, len(rows))
	for i, r := range rows {
		idToIdx[r.id] = i
	}

	// Load all files for these bulletin entries.
	err = sqlitex.Execute(conn,
		`SELECT bulletin_id, file_path FROM bulletin_files ORDER BY bulletin_id, file_path`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				bid := stmt.ColumnInt64(0)
				if idx, ok := idToIdx[bid]; ok {
					rows[idx].entry.Files = append(rows[idx].entry.Files, stmt.ColumnText(1))
				}
				return nil
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("bulletin: load files: %w", err)
	}

	entries := make([]Entry, len(rows))
	for i, r := range rows {
		entries[i] = r.entry
	}
	return entries, nil
}

// Scan reads all bulletin entries and returns those matching the given files or topics.
// An entry matches if any of its files have a bidirectional path containment with the
// files list, or if its topic matches any of the given topics. Both filters are
// case-insensitive. If both files and topics are empty, all entries are returned.
func Scan(conn *db.Conn, files []string, topics []string) ([]Entry, error) {
	all, err := Load(conn)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 && len(topics) == 0 {
		return all, nil
	}

	// Normalize filters
	lowerTopics := make([]string, len(topics))
	for i, t := range topics {
		lowerTopics[i] = strings.ToLower(t)
	}

	var result []Entry
	for _, e := range all {
		if matchesTopic(e.Topic, lowerTopics) || matchesFiles(e.Files, files) {
			result = append(result, e)
		}
	}
	return result, nil
}

// Clear deletes all bulletin entries and their associated files.
func Clear(conn *db.Conn) error {
	if err := sqlitex.Execute(conn, `DELETE FROM bulletin_files`, nil); err != nil {
		return fmt.Errorf("bulletin: clear files: %w", err)
	}
	if err := sqlitex.Execute(conn, `DELETE FROM bulletin`, nil); err != nil {
		return fmt.Errorf("bulletin: clear: %w", err)
	}
	return nil
}

func matchesTopic(topic string, lowerTopics []string) bool {
	if len(lowerTopics) == 0 {
		return false
	}
	lt := strings.ToLower(topic)
	for _, t := range lowerTopics {
		if lt == t {
			return true
		}
	}
	return false
}

func matchesFiles(entryFiles []string, filterFiles []string) bool {
	if len(filterFiles) == 0 {
		return false
	}
	for _, ef := range entryFiles {
		ef = normalizePath(ef)
		for _, ff := range filterFiles {
			ff = normalizePath(ff)
			if pathContains(ef, ff) || pathContains(ff, ef) {
				return true
			}
		}
	}
	return false
}

// normalizePath trims whitespace, normalizes separators, and removes trailing slashes.
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

// pathContains checks if child equals parent or is nested under parent
// using path-boundary matching (separator-aware), preventing false matches
// like "src/auth" matching "src/authz/login.go".
func pathContains(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}
