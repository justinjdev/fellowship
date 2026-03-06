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

func TestParseInput_TaskUpdate(t *testing.T) {
	input := `{"tool_input":{"status":"completed","metadata":{"phase":"Research"}}}`
	hi, err := ParseInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}
	if hi.ToolInput.Status != "completed" {
		t.Errorf("Status = %q", hi.ToolInput.Status)
	}
	if hi.ToolInput.Metadata.Phase != "Research" {
		t.Errorf("Metadata.Phase = %q", hi.ToolInput.Metadata.Phase)
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
