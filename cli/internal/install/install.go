package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

func buildHooks(binPath string) map[string][]hookMatcher {
	return map[string][]hookMatcher{
		"PreToolUse": {
			{Matcher: "Edit|Write|Bash|Agent|Skill|NotebookEdit", Hooks: []hookEntry{{Type: "command", Command: binPath + " hook gate-guard"}}},
			{Matcher: "SendMessage", Hooks: []hookEntry{{Type: "command", Command: binPath + " hook gate-submit"}}},
			{Matcher: "TaskUpdate", Hooks: []hookEntry{{Type: "command", Command: binPath + " hook completion-guard"}}},
		},
		"PostToolUse": {
			{Matcher: "Skill", Hooks: []hookEntry{{Type: "command", Command: binPath + " hook gate-prereq"}}},
			{Matcher: "TaskUpdate", Hooks: []hookEntry{{Type: "command", Command: binPath + " hook metadata-track"}}},
		},
	}
}

func Install(projectDir, binPath string) error {
	claudeDir := filepath.Join(projectDir, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}

	settings := make(map[string]any)
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing existing settings.json: %w", err)
		}
	}

	settings["hooks"] = buildHooks(binPath)

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')

	tmp := settingsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	return os.Rename(tmp, settingsPath)
}

func Uninstall(projectDir string) error {
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parsing settings.json: %w", err)
	}

	delete(settings, "hooks")

	if len(settings) == 0 {
		return os.Remove(settingsPath)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	tmp := settingsPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath)
}
