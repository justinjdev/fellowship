package hooks

import (
	"encoding/json"
	"fmt"
	"io"
)

type HookInput struct {
	ToolInput ToolInput `json:"tool_input"`
}

type ToolInput struct {
	Command      string        `json:"command,omitempty"`
	FilePath     string        `json:"file_path,omitempty"`
	NotebookPath string        `json:"notebook_path,omitempty"`
	Content      string        `json:"content,omitempty"`
	Skill        string        `json:"skill,omitempty"`
	Status       string        `json:"status,omitempty"`
	Metadata     *TaskMetadata `json:"metadata,omitempty"`
}

type TaskMetadata struct {
	Phase string `json:"phase,omitempty"`
}

func ParseInput(r io.Reader) (*HookInput, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	var hi HookInput
	if err := json.Unmarshal(data, &hi); err != nil {
		return nil, fmt.Errorf("parsing input: %w", err)
	}
	return &hi, nil
}
