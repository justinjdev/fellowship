package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type CompanyEntry struct {
	Name   string   `json:"name"`
	Quests []string `json:"quests"` // quest names
	Scouts []string `json:"scouts"` // scout names
}

type FellowshipState struct {
	Version   int            `json:"version"`
	Name      string         `json:"name"`
	CreatedAt string         `json:"created_at"`
	MainRepo  string         `json:"main_repo"`
	Quests    []QuestEntry   `json:"quests"`
	Scouts    []ScoutEntry   `json:"scouts"`
	Companies []CompanyEntry  `json:"companies"`
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
	ErrandsDone        int     `json:"errands_done"`
	ErrandsTotal       int     `json:"errands_total"`
}

type DashboardStatus struct {
	Name         string        `json:"name"`
	Quests       []QuestStatus `json:"quests"`
	Scouts       []ScoutEntry  `json:"scouts"`
	Companies    []CompanyEntry `json:"companies"`
	PollInterval int           `json:"poll_interval"`
}

// DiscoverQuests tries fellowship-state.json first, falls back to git worktree list.
func DiscoverQuests(gitRoot string) (*DashboardStatus, error) {
	statePath := filepath.Join(gitRoot, datadir.Name(), "fellowship-state.json")
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
		Companies:    fs.Companies,
		PollInterval: 5,
	}
	if status.Scouts == nil {
		status.Scouts = []ScoutEntry{}
	}
	if status.Companies == nil {
		status.Companies = []CompanyEntry{}
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
		questStatePath := filepath.Join(wtPath, datadir.Name(), "quest-state.json")
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
	questStatePath := filepath.Join(worktree, datadir.Name(), "quest-state.json")
	s, err := state.Load(questStatePath)
	if err != nil {
		return nil, err
	}
	done, total := LoadErrandProgress(worktree)
	return &QuestStatus{
		Name:            name,
		Worktree:        worktree,
		Phase:           s.Phase,
		GatePending:     s.GatePending,
		GateID:          s.GateID,
		LembasCompleted: s.LembasCompleted,
		MetadataUpdated: s.MetadataUpdated,
		ErrandsDone:        done,
		ErrandsTotal:       total,
	}, nil
}

// LoadErrandProgress loads the hook file from a worktree and returns progress counts.
func LoadErrandProgress(worktree string) (done, total int) {
	errandPath := filepath.Join(worktree, datadir.Name(), "quest-errands.json")
	h, err := errand.Load(errandPath)
	if err != nil {
		return 0, 0
	}
	return errand.Progress(h)
}

// WithStateLock acquires an exclusive file lock, loads the state, calls fn to
// mutate it, and saves the result. The entire load→mutate→save is atomic with
// respect to other processes using the same lock.
func WithStateLock(path string, fn func(s *FellowshipState) error) error {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	s, err := LoadFellowshipState(path)
	if err != nil {
		return err
	}

	if err := fn(s); err != nil {
		return err
	}

	return SaveFellowshipState(path, s)
}

func SaveFellowshipState(path string, s *FellowshipState) error {
	if s.Quests == nil {
		s.Quests = []QuestEntry{}
	}
	if s.Scouts == nil {
		s.Scouts = []ScoutEntry{}
	}
	if s.Companies == nil {
		s.Companies = []CompanyEntry{}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling fellowship state: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
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
