package status

import (
	"path/filepath"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
)

type QuestInfo struct {
	Name            string `json:"name"`
	TaskDescription string `json:"task_description"`
	Worktree        string `json:"worktree"`
	Branch          string `json:"branch"`
	Phase           string `json:"phase"`
	GatePending     bool   `json:"gate_pending"`
	HasCheckpoint   bool   `json:"has_checkpoint"`
	HasUncommitted  bool   `json:"has_uncommitted"`
	Merged          bool   `json:"merged"`
	Classification  string `json:"classification"`
}

type FellowshipInfo struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type StatusResult struct {
	Fellowship     *FellowshipInfo `json:"fellowship,omitempty"`
	Quests         []QuestInfo     `json:"quests"`
	MergedBranches []string        `json:"merged_branches"`
}

// ClassifyQuest returns "complete" if Merged, "resumable" if HasCheckpoint, "stale" otherwise.
func ClassifyQuest(q QuestInfo) string {
	if q.Merged {
		return "complete"
	}
	if q.HasCheckpoint {
		return "resumable"
	}
	return "stale"
}

// ParseMergedBranches parses `git branch --merged` output and returns only
// branches with the "fellowship/" prefix. Lines may have a `*` prefix and
// leading whitespace.
func ParseMergedBranches(gitOutput string) []string {
	result := []string{}
	for _, line := range strings.Split(gitOutput, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "fellowship/") {
			result = append(result, line)
		}
	}
	return result
}

// questRow holds joined data from fellowship_quests + quest_state.
type questRow struct {
	name            string
	taskDescription string
	worktree        string
	branch          string
	phase           string
	gatePending     bool
}

// Scan discovers fellowship quest state from the DB and git worktrees for crash recovery.
func Scan(conn *sqlite.Conn, gitRoot string) (*StatusResult, error) {
	result := &StatusResult{
		Quests:         []QuestInfo{},
		MergedBranches: []string{},
	}

	dataDir := datadir.Name()

	// Load fellowship metadata from DB (optional — may not exist).
	var fellowshipName, fellowshipCreatedAt string
	var hasFellowship bool
	err := sqlitex.Execute(conn,
		`SELECT name, created_at FROM fellowship WHERE id = 1`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				hasFellowship = true
				fellowshipName = stmt.ColumnText(0)
				fellowshipCreatedAt = stmt.ColumnText(1)
				return nil
			},
		})
	if err == nil && hasFellowship {
		result.Fellowship = &FellowshipInfo{
			Name:      fellowshipName,
			CreatedAt: fellowshipCreatedAt,
		}
	}

	// Query quests from DB: join fellowship_quests with quest_state.
	var rows []questRow
	err = sqlitex.Execute(conn,
		`SELECT fq.name, fq.task_description, fq.worktree, fq.branch,
			COALESCE(qs.phase, ''), COALESCE(qs.gate_pending, 0)
		 FROM fellowship_quests fq
		 LEFT JOIN quest_state qs ON fq.name = qs.quest_name`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				rows = append(rows, questRow{
					name:            stmt.ColumnText(0),
					taskDescription: stmt.ColumnText(1),
					worktree:        stmt.ColumnText(2),
					branch:          stmt.ColumnText(3),
					phase:           stmt.ColumnText(4),
					gatePending:     stmt.ColumnInt(5) != 0,
				})
				return nil
			},
		})
	if err != nil {
		return nil, err
	}

	// Discover merged branches (git operation).
	mergedOutput, err := gitutil.RunGit(gitRoot, "branch", "--merged", "main")
	if err == nil {
		result.MergedBranches = ParseMergedBranches(mergedOutput)
	}
	mergedSet := map[string]bool{}
	for _, b := range result.MergedBranches {
		mergedSet[b] = true
	}

	// Build quest info from DB rows + git filesystem checks.
	for _, row := range rows {
		hasCheckpoint := false
		hasUncommitted := false
		if row.worktree != "" {
			hasCheckpoint = gitutil.FileExists(filepath.Join(row.worktree, dataDir, "checkpoint.md"))
			hasUncommitted = gitutil.CheckUncommitted(row.worktree)
		}

		qi := QuestInfo{
			Name:            row.name,
			TaskDescription: row.taskDescription,
			Worktree:        row.worktree,
			Branch:          row.branch,
			Phase:           row.phase,
			GatePending:     row.gatePending,
			HasCheckpoint:   hasCheckpoint,
			HasUncommitted:  hasUncommitted,
			Merged:          mergedSet[row.branch],
		}
		qi.Classification = ClassifyQuest(qi)
		result.Quests = append(result.Quests, qi)
	}

	return result, nil
}
