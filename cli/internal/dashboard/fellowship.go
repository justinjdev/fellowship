package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
)

type FellowshipState struct {
	Name      string       `json:"name"`
	CreatedAt string       `json:"created_at"`
	Quests    []QuestEntry `json:"quests"`
	Scouts    []ScoutEntry `json:"scouts"`
}

type QuestEntry struct {
	Name     string `json:"name"`
	Worktree string `json:"worktree"`
	TaskID   string `json:"task_id"`
}

type ScoutEntry struct {
	Name   string `json:"name"`
	TaskID string `json:"task_id"`
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
