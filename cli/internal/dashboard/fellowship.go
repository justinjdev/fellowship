package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/hook"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type FellowshipState struct {
	Version   int          `json:"version"`
	Name      string       `json:"name"`
	CreatedAt string       `json:"created_at"`
	MainRepo  string       `json:"main_repo"`
	Quests    []QuestEntry `json:"quests"`
	Scouts    []ScoutEntry `json:"scouts"`
}

type QuestEntry struct {
	Name            string `json:"name"`
	TaskDescription string `json:"task_description"`
	Worktree        string `json:"worktree"`
	Branch          string `json:"branch"`
	TaskID          string `json:"task_id"`
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
	GatePending     bool    `json:"gate_pending"`
	GateID          *string `json:"gate_id"`
	LembasCompleted bool    `json:"lembas_completed"`
	MetadataUpdated bool    `json:"metadata_updated"`
	WorkDone        int     `json:"work_done"`
	WorkTotal       int     `json:"work_total"`
}

type DashboardStatus struct {
	Name         string        `json:"name"`
	Quests       []QuestStatus `json:"quests"`
	Scouts       []ScoutEntry  `json:"scouts"`
	PollInterval int           `json:"poll_interval"`
}

// DiscoverQuests tries fellowship-state.json first, falls back to git worktree list.
func DiscoverQuests(gitRoot string) (*DashboardStatus, error) {
	statePath := filepath.Join(gitRoot, "tmp", "fellowship-state.json")
	fs, err := LoadFellowshipState(statePath)
	if err == nil {
		return discoverFromFellowshipState(fs)
	}
	return discoverFromWorktrees(gitRoot)
}

// discoverFromFellowshipState reads fellowship state and loads each quest's state.
func discoverFromFellowshipState(fs *FellowshipState) (*DashboardStatus, error) {
	status := &DashboardStatus{
		Name:         fs.Name,
		Quests:       []QuestStatus{},
		Scouts:       fs.Scouts,
		PollInterval: 5,
	}
	if status.Scouts == nil {
		status.Scouts = []ScoutEntry{}
	}
	for _, q := range fs.Quests {
		qs, err := loadQuestStatus(q.Name, q.Worktree)
		if err != nil {
			// Skip quests where state can't be loaded
			continue
		}
		status.Quests = append(status.Quests, *qs)
	}
	return status, nil
}

// discoverFromWorktrees scans git worktree list for quest-state.json files.
func discoverFromWorktrees(gitRoot string) (*DashboardStatus, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	status := &DashboardStatus{
		Name:         filepath.Base(gitRoot),
		Quests:       []QuestStatus{},
		Scouts:       []ScoutEntry{},
		PollInterval: 5,
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		wtPath := strings.TrimPrefix(line, "worktree ")
		questStatePath := filepath.Join(wtPath, "tmp", "quest-state.json")
		if _, err := os.Stat(questStatePath); err != nil {
			continue
		}
		name := filepath.Base(wtPath)
		qs, err := loadQuestStatus(name, wtPath)
		if err != nil {
			continue
		}
		status.Quests = append(status.Quests, *qs)
	}

	return status, nil
}

// loadQuestStatus loads a single quest's state from its worktree.
func loadQuestStatus(name, worktree string) (*QuestStatus, error) {
	questStatePath := filepath.Join(worktree, "tmp", "quest-state.json")
	s, err := state.Load(questStatePath)
	if err != nil {
		return nil, err
	}
	done, total := LoadWorkProgress(worktree)
	return &QuestStatus{
		Name:            name,
		Worktree:        worktree,
		Phase:           s.Phase,
		GatePending:     s.GatePending,
		GateID:          s.GateID,
		LembasCompleted: s.LembasCompleted,
		MetadataUpdated: s.MetadataUpdated,
		WorkDone:        done,
		WorkTotal:       total,
	}, nil
}

// LoadWorkProgress loads the hook file from a worktree and returns progress counts.
func LoadWorkProgress(worktree string) (done, total int) {
	hookPath := filepath.Join(worktree, "tmp", "quest-hook.json")
	h, err := hook.Load(hookPath)
	if err != nil {
		return 0, 0
	}
	return hook.Progress(h)
}

func LoadFellowshipState(path string) (*FellowshipState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fellowship state file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("fellowship state file is empty")
	}
	var s FellowshipState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing fellowship state file: %w", err)
	}
	return &s, nil
}
