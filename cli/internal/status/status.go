package status

import (
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/state"
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

// Scan discovers fellowship quest state across git worktrees for crash recovery.
func Scan(gitRoot string) (*StatusResult, error) {
	result := &StatusResult{
		Quests:         []QuestInfo{},
		MergedBranches: []string{},
	}

	dataDir := datadir.Name()

	// Load fellowship state (optional — may not exist).
	statePath := filepath.Join(gitRoot, dataDir, "fellowship-state.json")
	fs, err := dashboard.LoadFellowshipState(statePath)
	if err == nil {
		result.Fellowship = &FellowshipInfo{
			Name:      fs.Name,
			CreatedAt: fs.CreatedAt,
		}
	}

	// Build a task description lookup from fellowship state.
	taskDescriptions := map[string]string{}
	if fs != nil {
		for _, q := range fs.Quests {
			taskDescriptions[q.Name] = q.TaskDescription
		}
	}

	// Discover merged branches.
	mergedOutput, err := gitutil.RunGit(gitRoot, "branch", "--merged", "main")
	if err == nil {
		result.MergedBranches = ParseMergedBranches(mergedOutput)
	}
	mergedSet := map[string]bool{}
	for _, b := range result.MergedBranches {
		mergedSet[b] = true
	}

	// Enumerate worktrees.
	worktrees, err := gitutil.ListWorktrees(gitRoot)
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		questStatePath := filepath.Join(wt, dataDir, "quest-state.json")
		if !gitutil.FileExists(questStatePath) {
			continue
		}

		s, err := state.Load(questStatePath)
		if err != nil {
			continue
		}

		branch := gitutil.BranchForWorktree(wt)
		hasCheckpoint := gitutil.FileExists(filepath.Join(wt, dataDir, "checkpoint.md"))
		hasUncommitted := gitutil.CheckUncommitted(wt)

		qi := QuestInfo{
			Name:            s.QuestName,
			TaskDescription: taskDescriptions[s.QuestName],
			Worktree:        wt,
			Branch:          branch,
			Phase:           s.Phase,
			GatePending:     s.GatePending,
			HasCheckpoint:   hasCheckpoint,
			HasUncommitted:  hasUncommitted,
			Merged:          mergedSet[branch],
		}
		qi.Classification = ClassifyQuest(qi)
		result.Quests = append(result.Quests, qi)
	}

	return result, nil
}

