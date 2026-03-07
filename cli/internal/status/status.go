package status

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
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

	// Load fellowship state (optional — may not exist).
	statePath := filepath.Join(gitRoot, "tmp", "fellowship-state.json")
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
	mergedOutput, err := runGit(gitRoot, "branch", "--merged", "main")
	if err == nil {
		result.MergedBranches = ParseMergedBranches(mergedOutput)
	}
	mergedSet := map[string]bool{}
	for _, b := range result.MergedBranches {
		mergedSet[b] = true
	}

	// Enumerate worktrees.
	worktrees, err := listWorktrees(gitRoot)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	for _, wt := range worktrees {
		questStatePath := filepath.Join(wt, "tmp", "quest-state.json")
		if !fileExists(questStatePath) {
			continue
		}

		s, err := state.Load(questStatePath)
		if err != nil {
			continue
		}

		branch := branchForWorktree(wt)
		hasCheckpoint := fileExists(filepath.Join(wt, "tmp", "checkpoint.md"))
		hasUncommitted := checkUncommitted(wt)

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

// listWorktrees parses `git worktree list --porcelain` and returns worktree paths.
func listWorktrees(gitRoot string) ([]string, error) {
	out, err := runGit(gitRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths, nil
}

// branchForWorktree returns the current branch name for a worktree directory.
func branchForWorktree(wtPath string) string {
	out, err := runGit(wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// checkUncommitted returns true if `git status --porcelain` produces any output.
func checkUncommitted(wtPath string) bool {
	out, err := runGit(wtPath, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// runGit executes a git command in the given directory and returns stdout.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// fileExists returns true if the path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
