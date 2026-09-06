package hooks

import (
	"strings"
	"testing"
)

func TestParseInput_BashCommand(t *testing.T) {
	input := `{"tool_input":{"command":"ls"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.ToolInput.Command != "ls" {
		t.Errorf("Command = %q, want ls", hi.ToolInput.Command)
	}
}

func TestParseInput_EditFile(t *testing.T) {
	input := `{"tool_input":{"file_path":"/repo/src/main.ts","old_string":"foo","new_string":"bar"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.ToolInput.FilePath != "/repo/src/main.ts" {
		t.Errorf("FilePath = %q", hi.ToolInput.FilePath)
	}
}

func TestParseInput_SendMessage(t *testing.T) {
	input := `{"tool_input":{"content":"[GATE] Research complete\n- [x] done"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.ToolInput.Content != "[GATE] Research complete\n- [x] done" {
		t.Errorf("Content = %q", hi.ToolInput.Content)
	}
}

// The SendMessage tool carries its body in `message`; `content` is the field
// the pre-implicit-team tool used. Both must read as the gate text.
func TestParseInput_SendMessageMessageField(t *testing.T) {
	input := `{"tool_name":"SendMessage","tool_input":{"to":"main","summary":"gate","message":"[GATE] Research complete\n- [x] done"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if got := hi.ToolInput.MessageBody(); got != "[GATE] Research complete\n- [x] done" {
		t.Errorf("MessageBody = %q", got)
	}
}

// A hook that fires inside a subagent carries agent_id and agent_type next to
// the shared session_id; the main conversation's payload carries neither.
func TestParseInput_SubagentIdentity(t *testing.T) {
	input := `{"session_id":"s-1","agent_id":"a-9","agent_type":"general-purpose","tool_name":"Bash","tool_input":{"command":"ls"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.SessionID != "s-1" || hi.AgentID != "a-9" || hi.AgentType != "general-purpose" {
		t.Errorf("identity = %q/%q/%q", hi.SessionID, hi.AgentID, hi.AgentType)
	}
}

func TestParseInput_SkillInvocation(t *testing.T) {
	input := `{"tool_input":{"skill":"fellowship:lembas"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.ToolInput.Skill != "fellowship:lembas" {
		t.Errorf("Skill = %q", hi.ToolInput.Skill)
	}
}

func TestParseInput_NotebookEdit(t *testing.T) {
	input := `{"tool_input":{"notebook_path":"/repo/analysis.ipynb"}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.ToolInput.NotebookPath != "/repo/analysis.ipynb" {
		t.Errorf("NotebookPath = %q", hi.ToolInput.NotebookPath)
	}
}

func TestParseInput_MalformedJSON(t *testing.T) {
	_, err := ParseInput(strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseInput_EmptyInput(t *testing.T) {
	_, err := ParseInput(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}
