package db

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"
)

// execCommand is the function used to create exec.Cmd. Tests can override it.
var execCommand = exec.Command

// JSON structs for parsing legacy files.

type fellowshipStateJSON struct {
	Version    int                  `json:"version"`
	Name       string               `json:"name"`
	MainRepo   string               `json:"main_repo"`
	BaseBranch string               `json:"base_branch"`
	Quests     []fellowshipQuestJSON `json:"quests"`
	Scouts     []fellowshipScoutJSON `json:"scouts"`
	Companies  []companyJSON         `json:"companies"`
	CreatedAt  string               `json:"created_at"`
}

type fellowshipQuestJSON struct {
	Name            string `json:"name"`
	TaskDescription string `json:"task_description"`
	Worktree        string `json:"worktree"`
	Branch          string `json:"branch"`
	TaskID          string `json:"task_id"`
}

type fellowshipScoutJSON struct {
	Name     string `json:"name"`
	Question string `json:"question"`
	TaskID   string `json:"task_id"`
}

type companyJSON struct {
	Name   string   `json:"name"`
	Quests []string `json:"quests"`
	Scouts []string `json:"scouts"`
}

type questStateJSON struct {
	Version         int      `json:"version"`
	QuestName       string   `json:"quest_name"`
	TaskID          string   `json:"task_id"`
	TeamName        string   `json:"team_name"`
	Phase           string   `json:"phase"`
	GatePending     bool     `json:"gate_pending"`
	GateID          *string  `json:"gate_id"`
	LembasCompleted bool     `json:"lembas_completed"`
	MetadataUpdated bool     `json:"metadata_updated"`
	Held            bool     `json:"held"`
	HeldReason      *string  `json:"held_reason"`
	AutoApproveGates []string `json:"auto_approve_gates"`
}

type questTomeJSON struct {
	Version         int              `json:"version"`
	QuestName       string           `json:"quest_name"`
	Task            string           `json:"task"`
	PhasesCompleted []phaseRecJSON   `json:"phases_completed"`
	GateHistory     []gateEventJSON  `json:"gate_history"`
	FilesTouched    []string         `json:"files_touched"`
	Respawns        int              `json:"respawns"`
	Status          string           `json:"status"`
}

type phaseRecJSON struct {
	Phase       string `json:"phase"`
	CompletedAt string `json:"completed_at"`
	DurationS   int    `json:"duration_s"`
}

type gateEventJSON struct {
	Phase     string `json:"phase"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

type questErrandsJSON struct {
	Version   int          `json:"version"`
	QuestName string       `json:"quest_name"`
	Task      string       `json:"task"`
	Items     []errandJSON `json:"items"`
}

type errandJSON struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Phase       string   `json:"phase"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	DependsOn   []string `json:"depends_on"`
}

type heraldLineJSON struct {
	Timestamp string `json:"timestamp"`
	Quest     string `json:"quest"`
	Type      string `json:"type"`
	Phase     string `json:"phase"`
	Detail    string `json:"detail"`
}

type bulletinLineJSON struct {
	Timestamp string   `json:"ts"`
	Quest     string   `json:"quest"`
	Topic     string   `json:"topic"`
	Files     []string `json:"files"`
	Discovery string   `json:"discovery"`
}

type autopsyJSON struct {
	Version    int      `json:"version"`
	Timestamp  string   `json:"ts"`
	Quest      string   `json:"quest"`
	Task       string   `json:"task"`
	Phase      string   `json:"phase"`
	Trigger    string   `json:"trigger"`
	Files      []string `json:"files"`
	Modules    []string `json:"modules"`
	WhatFailed string   `json:"what_failed"`
	Tags       []string `json:"tags"`
	ExpiresAt  string   `json:"expires_at"`
}

// migrationFile tracks a discovered JSON file to migrate.
type migrationFile struct {
	path     string // absolute path to the file
	relPath  string // relative path for backup structure
	fileType string // e.g., "fellowship-state", "quest-state", etc.
}

// MigrateJSON reads all JSON/JSONL files from mainRepo and its worktrees,
// inserts them into the DB, backs up originals, and deletes them.
func MigrateJSON(d *DB, mainRepo string) error {
	dataDir := filepath.Join(mainRepo, ".fellowship")
	backupDir := filepath.Join(dataDir, "backup")

	// 1. Discover all JSON files
	files, err := discoverJSONFiles(mainRepo)
	if err != nil {
		return fmt.Errorf("migrate: discover files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("migrate: no JSON files found to migrate")
	}

	// 2. Back up originals
	if err := backupFiles(files, dataDir, backupDir); err != nil {
		return fmt.Errorf("migrate: backup: %w", err)
	}

	// 3. Parse and insert in a single transaction
	var summary migrationSummary
	if err := d.WithTx(context.Background(), func(conn *Conn) error {
		for _, f := range files {
			if err := migrateFile(conn, f, &summary); err != nil {
				return fmt.Errorf("migrate %s (%s): %w", f.relPath, f.fileType, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 4. Delete originals + .lock sidecars
	for _, f := range files {
		os.Remove(f.path)
		os.Remove(f.path + ".lock")
	}

	// Also clean up empty autopsies dirs
	autopsyDir := filepath.Join(dataDir, "autopsies")
	removeEmptyDir(autopsyDir)

	// 5. Print summary
	fmt.Printf("Migration complete:\n")
	fmt.Printf("  Fellowship state: %d\n", summary.fellowship)
	fmt.Printf("  Quest states:     %d\n", summary.questStates)
	fmt.Printf("  Quest tomes:      %d\n", summary.questTomes)
	fmt.Printf("  Quest errands:    %d\n", summary.questErrands)
	fmt.Printf("  Herald events:    %d\n", summary.heraldEvents)
	fmt.Printf("  Bulletin entries: %d\n", summary.bulletinEntries)
	fmt.Printf("  Autopsies:        %d\n", summary.autopsies)
	fmt.Printf("  Backup directory: %s\n", backupDir)
	return nil
}

type migrationSummary struct {
	fellowship      int
	questStates     int
	questTomes      int
	questErrands    int
	heraldEvents    int
	bulletinEntries int
	autopsies       int
}

// discoverJSONFiles finds all JSON/JSONL files in the main .fellowship/ dir
// and all worktree .fellowship/ dirs.
func discoverJSONFiles(mainRepo string) ([]migrationFile, error) {
	var result []migrationFile

	// Main repo .fellowship/ files
	mainDataDir := filepath.Join(mainRepo, ".fellowship")
	result = append(result, scanDataDir(mainDataDir, "main")...)

	// Discover worktrees
	worktrees, err := listWorktreePaths(mainRepo)
	if err != nil {
		return nil, err
	}
	for _, wt := range worktrees {
		// Skip the main repo itself
		if filepath.Clean(wt) == filepath.Clean(mainRepo) {
			continue
		}
		wtDataDir := filepath.Join(wt, ".fellowship")
		result = append(result, scanDataDir(wtDataDir, filepath.Base(wt))...)
	}

	return result, nil
}

// scanDataDir looks for known JSON files in a .fellowship directory.
func scanDataDir(dataDir string, label string) []migrationFile {
	var files []migrationFile

	// Ordered to satisfy FK constraints: fellowship-state and quest-state first,
	// then tables that reference them.
	knownFiles := []struct {
		name     string
		fileType string
	}{
		{"fellowship-state.json", "fellowship-state"},
		{"quest-state.json", "quest-state"},
		{"quest-tome.json", "quest-tome"},
		{"quest-errands.json", "quest-errands"},
		{"quest-herald.jsonl", "quest-herald"},
		{"bulletin.jsonl", "bulletin"},
	}

	for _, kf := range knownFiles {
		p := filepath.Join(dataDir, kf.name)
		if _, err := os.Stat(p); err == nil {
			files = append(files, migrationFile{
				path:     p,
				relPath:  filepath.Join(label, kf.name),
				fileType: kf.fileType,
			})
		}
	}

	// Autopsies directory
	autopsyDir := filepath.Join(dataDir, "autopsies")
	entries, err := os.ReadDir(autopsyDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			p := filepath.Join(autopsyDir, e.Name())
			files = append(files, migrationFile{
				path:     p,
				relPath:  filepath.Join(label, "autopsies", e.Name()),
				fileType: "autopsy",
			})
		}
	}

	return files
}

// backupFiles copies each file to backupDir preserving the relative path.
func backupFiles(files []migrationFile, dataDir, backupDir string) error {
	for _, f := range files {
		dst := filepath.Join(backupDir, f.relPath)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(f.path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// migrateFile parses and inserts a single file based on its type.
func migrateFile(conn *Conn, f migrationFile, s *migrationSummary) error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}

	switch f.fileType {
	case "fellowship-state":
		return migrateFellowshipState(conn, data, s)
	case "quest-state":
		return migrateQuestState(conn, data, s)
	case "quest-tome":
		return migrateQuestTome(conn, data, s)
	case "quest-errands":
		return migrateQuestErrands(conn, data, s)
	case "quest-herald":
		return migrateHerald(conn, data, s)
	case "bulletin":
		return migrateBulletin(conn, data, s)
	case "autopsy":
		return migrateAutopsy(conn, data, s)
	default:
		return fmt.Errorf("unknown file type: %s", f.fileType)
	}
}

func migrateFellowshipState(conn *Conn, data []byte, s *migrationSummary) error {
	var fs fellowshipStateJSON
	if err := json.Unmarshal(data, &fs); err != nil {
		return fmt.Errorf("parse fellowship-state.json: %w", err)
	}

	versionStr := fmt.Sprintf("%d", fs.Version)
	if fs.CreatedAt == "" {
		fs.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Insert fellowship singleton
	if err := sqlitex.Execute(conn,
		`INSERT OR REPLACE INTO fellowship (id, version, name, main_repo, base_branch, created_at)
		 VALUES (1, :version, :name, :main_repo, :base_branch, :created_at)`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":version":     versionStr,
				":name":        fs.Name,
				":main_repo":   fs.MainRepo,
				":base_branch": fs.BaseBranch,
				":created_at":  fs.CreatedAt,
			},
		}); err != nil {
		return err
	}

	// Insert quests
	for _, q := range fs.Quests {
		if err := sqlitex.Execute(conn,
			`INSERT OR REPLACE INTO fellowship_quests (name, task_description, worktree, branch, task_id)
			 VALUES (:name, :task_desc, :worktree, :branch, :task_id)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":name":      q.Name,
					":task_desc": q.TaskDescription,
					":worktree":  q.Worktree,
					":branch":    q.Branch,
					":task_id":   q.TaskID,
				},
			}); err != nil {
			return err
		}
	}

	// Insert scouts
	for _, sc := range fs.Scouts {
		if err := sqlitex.Execute(conn,
			`INSERT OR REPLACE INTO fellowship_scouts (name, question, task_id)
			 VALUES (:name, :question, :task_id)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":name":     sc.Name,
					":question": sc.Question,
					":task_id":  sc.TaskID,
				},
			}); err != nil {
			return err
		}
	}

	// Insert companies and members
	for _, c := range fs.Companies {
		if err := sqlitex.Execute(conn,
			`INSERT OR REPLACE INTO companies (name) VALUES (:name)`,
			&sqlitex.ExecOptions{Named: map[string]any{":name": c.Name}}); err != nil {
			return err
		}
		for _, q := range c.Quests {
			if err := sqlitex.Execute(conn,
				`INSERT OR REPLACE INTO company_members (company_name, member_name, member_type)
				 VALUES (:company, :member, 'quest')`,
				&sqlitex.ExecOptions{
					Named: map[string]any{":company": c.Name, ":member": q},
				}); err != nil {
				return err
			}
		}
		for _, sc := range c.Scouts {
			if err := sqlitex.Execute(conn,
				`INSERT OR REPLACE INTO company_members (company_name, member_name, member_type)
				 VALUES (:company, :member, 'scout')`,
				&sqlitex.ExecOptions{
					Named: map[string]any{":company": c.Name, ":member": sc},
				}); err != nil {
				return err
			}
		}
	}

	s.fellowship++
	return nil
}

func migrateQuestState(conn *Conn, data []byte, s *migrationSummary) error {
	var qs questStateJSON
	if err := json.Unmarshal(data, &qs); err != nil {
		return fmt.Errorf("parse quest-state.json: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	boolInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	var gateID any
	if qs.GateID != nil {
		gateID = *qs.GateID
	}
	var heldReason any
	if qs.HeldReason != nil {
		heldReason = *qs.HeldReason
	}
	var autoApprove any
	if len(qs.AutoApproveGates) > 0 {
		b, _ := json.Marshal(qs.AutoApproveGates)
		autoApprove = string(b)
	}

	if err := sqlitex.Execute(conn,
		`INSERT OR REPLACE INTO quest_state
		 (quest_name, task_id, team_name, phase, gate_pending, gate_id,
		  lembas_completed, metadata_updated, held, held_reason, auto_approve,
		  created_at, updated_at)
		 VALUES (:name, :task_id, :team, :phase, :gate_pending, :gate_id,
		  :lembas, :metadata, :held, :held_reason, :auto_approve, :now, :now)`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":name":         qs.QuestName,
				":task_id":      qs.TaskID,
				":team":         qs.TeamName,
				":phase":        qs.Phase,
				":gate_pending": boolInt(qs.GatePending),
				":gate_id":      gateID,
				":lembas":       boolInt(qs.LembasCompleted),
				":metadata":     boolInt(qs.MetadataUpdated),
				":held":         boolInt(qs.Held),
				":held_reason":  heldReason,
				":auto_approve": autoApprove,
				":now":          now,
			},
		}); err != nil {
		return err
	}

	s.questStates++
	return nil
}

func migrateQuestTome(conn *Conn, data []byte, s *migrationSummary) error {
	var qt questTomeJSON
	if err := json.Unmarshal(data, &qt); err != nil {
		return fmt.Errorf("parse quest-tome.json: %w", err)
	}

	// Insert phase records
	for _, p := range qt.PhasesCompleted {
		if err := sqlitex.Execute(conn,
			`INSERT INTO quest_phases (quest_name, phase, completed_at, duration_s)
			 VALUES (:quest, :phase, :completed_at, :dur)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":quest":        qt.QuestName,
					":phase":        p.Phase,
					":completed_at": p.CompletedAt,
					":dur":          p.DurationS,
				},
			}); err != nil {
			return err
		}
	}

	// Insert gate history
	for _, g := range qt.GateHistory {
		if err := sqlitex.Execute(conn,
			`INSERT INTO quest_gates (quest_name, phase, action, timestamp, reason)
			 VALUES (:quest, :phase, :action, :ts, :reason)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":quest":  qt.QuestName,
					":phase":  g.Phase,
					":action": g.Action,
					":ts":     g.Timestamp,
					":reason": g.Reason,
				},
			}); err != nil {
			return err
		}
	}

	// Insert files touched
	for _, f := range qt.FilesTouched {
		if err := sqlitex.Execute(conn,
			`INSERT OR IGNORE INTO quest_files (quest_name, file_path) VALUES (:quest, :file)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{":quest": qt.QuestName, ":file": f},
			}); err != nil {
			return err
		}
	}

	// Update fellowship_quests with status and respawns
	if qt.Status != "" || qt.Respawns > 0 {
		if err := sqlitex.Execute(conn,
			`UPDATE fellowship_quests SET status = :status, respawns = :respawns WHERE name = :name`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":name":     qt.QuestName,
					":status":   qt.Status,
					":respawns": qt.Respawns,
				},
			}); err != nil {
			return err
		}
	}

	s.questTomes++
	return nil
}

func migrateQuestErrands(conn *Conn, data []byte, s *migrationSummary) error {
	var qe questErrandsJSON
	if err := json.Unmarshal(data, &qe); err != nil {
		return fmt.Errorf("parse quest-errands.json: %w", err)
	}

	// Insert all errands first, then deps, to satisfy FK constraints.
	for _, item := range qe.Items {
		if err := sqlitex.Execute(conn,
			`INSERT OR REPLACE INTO errands (id, quest_name, description, status, phase, created_at, updated_at)
			 VALUES (:id, :quest, :desc, :status, :phase, :created_at, :updated_at)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":id":         item.ID,
					":quest":      qe.QuestName,
					":desc":       item.Description,
					":status":     item.Status,
					":phase":      item.Phase,
					":created_at": item.CreatedAt,
					":updated_at": item.UpdatedAt,
				},
			}); err != nil {
			return err
		}
	}
	for _, item := range qe.Items {
		for _, dep := range item.DependsOn {
			if err := sqlitex.Execute(conn,
				`INSERT OR REPLACE INTO errand_deps (quest_name, errand_id, depends_on)
				 VALUES (:quest, :id, :dep)`,
				&sqlitex.ExecOptions{
					Named: map[string]any{":quest": qe.QuestName, ":id": item.ID, ":dep": dep},
				}); err != nil {
				return err
			}
		}
	}

	s.questErrands += len(qe.Items)
	return nil
}

func migrateHerald(conn *Conn, data []byte, s *migrationSummary) error {
	lines, err := parseJSONL[heraldLineJSON](data)
	if err != nil {
		return fmt.Errorf("parse quest-herald.jsonl: %w", err)
	}
	for _, h := range lines {
		if err := sqlitex.Execute(conn,
			`INSERT INTO herald (timestamp, quest, type, phase, detail)
			 VALUES (:ts, :quest, :type, :phase, :detail)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":ts":     h.Timestamp,
					":quest":  h.Quest,
					":type":   h.Type,
					":phase":  h.Phase,
					":detail": h.Detail,
				},
			}); err != nil {
			return err
		}
		s.heraldEvents++
	}
	return nil
}

func migrateBulletin(conn *Conn, data []byte, s *migrationSummary) error {
	lines, err := parseJSONL[bulletinLineJSON](data)
	if err != nil {
		return fmt.Errorf("parse bulletin.jsonl: %w", err)
	}
	for _, b := range lines {
		if err := sqlitex.Execute(conn,
			`INSERT INTO bulletin (timestamp, quest, topic, discovery)
			 VALUES (:ts, :quest, :topic, :discovery)`,
			&sqlitex.ExecOptions{
				Named: map[string]any{
					":ts":        b.Timestamp,
					":quest":     b.Quest,
					":topic":     b.Topic,
					":discovery": b.Discovery,
				},
			}); err != nil {
			return err
		}
		id := conn.LastInsertRowID()
		for _, f := range b.Files {
			if err := sqlitex.Execute(conn,
				`INSERT INTO bulletin_files (bulletin_id, file_path) VALUES (:id, :file)`,
				&sqlitex.ExecOptions{
					Named: map[string]any{":id": id, ":file": f},
				}); err != nil {
				return err
			}
		}
		s.bulletinEntries++
	}
	return nil
}

func migrateAutopsy(conn *Conn, data []byte, s *migrationSummary) error {
	var a autopsyJSON
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("parse autopsy json: %w", err)
	}

	if err := sqlitex.Execute(conn,
		`INSERT INTO autopsies (timestamp, quest, task, phase, trigger_type, what_failed, expires_at)
		 VALUES (:ts, :quest, :task, :phase, :trigger, :what_failed, :expires_at)`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":ts":          a.Timestamp,
				":quest":       a.Quest,
				":task":        a.Task,
				":phase":       a.Phase,
				":trigger":     a.Trigger,
				":what_failed": a.WhatFailed,
				":expires_at":  a.ExpiresAt,
			},
		}); err != nil {
		return err
	}

	id := conn.LastInsertRowID()
	for _, f := range a.Files {
		if err := sqlitex.Execute(conn,
			`INSERT INTO autopsy_files (autopsy_id, file_path) VALUES (:id, :file)`,
			&sqlitex.ExecOptions{Named: map[string]any{":id": id, ":file": f}}); err != nil {
			return err
		}
	}
	for _, m := range a.Modules {
		if err := sqlitex.Execute(conn,
			`INSERT INTO autopsy_modules (autopsy_id, module) VALUES (:id, :mod)`,
			&sqlitex.ExecOptions{Named: map[string]any{":id": id, ":mod": m}}); err != nil {
			return err
		}
	}
	for _, tag := range a.Tags {
		if err := sqlitex.Execute(conn,
			`INSERT INTO autopsy_tags (autopsy_id, tag) VALUES (:id, :tag)`,
			&sqlitex.ExecOptions{Named: map[string]any{":id": id, ":tag": tag}}); err != nil {
			return err
		}
	}

	s.autopsies++
	return nil
}

// parseJSONL parses newline-delimited JSON, skipping empty lines.
func parseJSONL[T any](data []byte) ([]T, error) {
	var result []T
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return result, nil
}

// listWorktreePaths parses `git worktree list --porcelain` output.
func listWorktreePaths(mainRepo string) ([]string, error) {
	cmd := execCommand("git", "worktree", "list", "--porcelain")
	cmd.Dir = mainRepo
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	if len(paths) == 0 {
		return []string{mainRepo}, nil
	}
	return paths, nil
}

// removeEmptyDir removes a directory only if it's empty.
func removeEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
}
