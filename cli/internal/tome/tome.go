package tome

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type QuestTome struct {
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

func Load(path string) (*QuestTome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading tome file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("tome file is empty")
	}
	var c QuestTome
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing tome file: %w", err)
	}
	return &c, nil
}

func Save(path string, c *QuestTome) error {
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tome: %w", err)
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

func RecordPhase(c *QuestTome, phase string) {
	c.PhasesCompleted = append(c.PhasesCompleted, PhaseRecord{
		Phase:       phase,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func RecordGate(c *QuestTome, phase, action string) {
	c.GateHistory = append(c.GateHistory, GateEvent{
		Phase:     phase,
		Action:    action,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func RecordFiles(c *QuestTome, files []string) {
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

func FindTome(fromDir string) (string, error) {
	root, err := gitRoot(fromDir)
	if err != nil {
		root = fromDir
	}
	path := filepath.Join(root, "tmp", "quest-tome.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return path, nil
}

// LoadOrCreate loads the tome from path, or creates a new one if the file does not exist.
func LoadOrCreate(path string) *QuestTome {
	c, err := Load(path)
	if err == nil {
		return c
	}
	return &QuestTome{
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
