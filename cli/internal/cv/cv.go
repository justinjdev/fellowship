package cv

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type QuestCV struct {
	Version         int           `json:"version"`
	QuestName       string        `json:"quest_name"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
	Task            string        `json:"task"`
	PhasesCompleted []PhaseRecord `json:"phases_completed"`
	GateHistory     []GateEvent   `json:"gate_history"`
	FilesTouched    []string      `json:"files_touched"`
	Respawns        int           `json:"respawns"`
	Status          string        `json:"status"` // "active", "completed", "failed"
}

type PhaseRecord struct {
	Phase       string `json:"phase"`
	CompletedAt string `json:"completed_at"`
	Duration    string `json:"duration,omitempty"`
}

type GateEvent struct {
	Phase     string `json:"phase"`
	Action    string `json:"action"` // "submitted", "approved", "rejected"
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

func Load(path string) (*QuestCV, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cv file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("cv file is empty")
	}
	var c QuestCV
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing cv file: %w", err)
	}
	return &c, nil
}

func Save(path string, c *QuestCV) error {
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cv: %w", err)
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

func RecordPhase(c *QuestCV, phase string) {
	c.PhasesCompleted = append(c.PhasesCompleted, PhaseRecord{
		Phase:       phase,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func RecordGate(c *QuestCV, phase, action string) {
	c.GateHistory = append(c.GateHistory, GateEvent{
		Phase:     phase,
		Action:    action,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func RecordFiles(c *QuestCV, files []string) {
	seen := make(map[string]bool, len(c.FilesTouched))
	for _, f := range c.FilesTouched {
		seen[f] = true
	}
	for _, f := range files {
		if !seen[f] {
			c.FilesTouched = append(c.FilesTouched, f)
			seen[f] = true
		}
	}
}

func FindCV(fromDir string) (string, error) {
	root, err := gitRoot(fromDir)
	if err != nil {
		root = fromDir
	}
	path := filepath.Join(root, "tmp", "quest-cv.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return path, nil
}

// LoadOrCreate loads the CV from path, or creates a new one if the file does not exist.
func LoadOrCreate(path string) *QuestCV {
	c, err := Load(path)
	if err == nil {
		return c
	}
	return &QuestCV{
		Version:         1,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Status:          "active",
		PhasesCompleted: []PhaseRecord{},
		GateHistory:     []GateEvent{},
		FilesTouched:    []string{},
	}
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
