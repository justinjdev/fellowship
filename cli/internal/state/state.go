package state

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type State struct {
	Version          int      `json:"version"`
	QuestName        string   `json:"quest_name"`
	TaskID           string   `json:"task_id"`
	TeamName         string   `json:"team_name"`
	Phase            string   `json:"phase"`
	GatePending      bool     `json:"gate_pending"`
	GateID           *string  `json:"gate_id"`
	LembasCompleted  bool     `json:"lembas_completed"`
	MetadataUpdated  bool     `json:"metadata_updated"`
	AutoApproveGates []string `json:"auto_approve_gates"`
}

var phaseOrder = []string{"Onboard", "Research", "Plan", "Implement", "Review", "Complete"}

func NextPhase(current string) (string, error) {
	for i, p := range phaseOrder {
		if p == current {
			if i+1 >= len(phaseOrder) {
				return "", fmt.Errorf("no phase after %s", current)
			}
			return phaseOrder[i+1], nil
		}
	}
	return "", fmt.Errorf("unknown phase: %s", current)
}

func IsEarlyPhase(phase string) bool {
	return phase == "Onboard" || phase == "Research" || phase == "Plan"
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("state file is empty")
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &s, nil
}

func Save(path string, s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
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

func FindStateFile(fromDir string) (string, error) {
	root, err := gitRoot(fromDir)
	if err != nil {
		root = fromDir
	}
	path := filepath.Join(root, "tmp", "quest-state.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return path, nil
}

func gitRoot(fromDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = fromDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
