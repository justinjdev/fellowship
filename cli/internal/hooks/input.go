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
	// AgentID is set only when the hook fires inside a subagent — a teammate
	// spawned with the Agent tool. Such an agent runs in-process and shares
	// the lead's session id, so the session id alone can no longer tell the
	// lead from a teammate: a payload that carries an agent id is never the
	// lead's own conversation.
	AgentID string `json:"agent_id,omitempty"`
	// AgentType is the subagent's type (e.g. "general-purpose",
	// "fellowship:scout"), present alongside AgentID.
	AgentType string `json:"agent_type,omitempty"`
}

type ToolInput struct {
	Command      string `json:"command,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
	NotebookPath string `json:"notebook_path,omitempty"`
	Content      string `json:"content,omitempty"`
	// Message is the SendMessage tool's body. Content is the field the
	// pre-implicit-team tool used; both are read so a gate is detected
	// whichever name the running Claude Code sends.
	Message string `json:"message,omitempty"`
	Skill   string `json:"skill,omitempty"`
}

// MessageBody returns the text of a SendMessage call, whichever field carried
// it.
func (t ToolInput) MessageBody() string {
	if t.Message != "" {
		return t.Message
	}
	return t.Content
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
