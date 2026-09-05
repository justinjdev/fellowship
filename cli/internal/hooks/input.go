package hooks

import (
	"encoding/json"
	"fmt"
	"io"
)

type HookInput struct {
	ToolName  string    `json:"tool_name,omitempty"`
	ToolInput ToolInput `json:"tool_input"`
	// SessionID identifies the Claude Code session the hook is firing for.
	// Claude Code sends it with every hook payload; it is the only thing in
	// the payload that distinguishes the lead's session from a teammate's when
	// both resolve to the same git top-level (see the worktree-guard).
	SessionID string `json:"session_id,omitempty"`
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
