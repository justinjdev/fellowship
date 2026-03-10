package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/filelock"
)

// ErrNoSave can be returned from a WithLock callback to skip saving
// the state file while still releasing the lock without error.
var ErrNoSave = fmt.Errorf("no save needed")

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
	Held             bool     `json:"held"`
	HeldReason       *string  `json:"held_reason"`
}

var phaseOrder = []string{"Onboard", "Research", "Plan", "Implement", "Adversarial", "Review", "Complete"}

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

// WithLock acquires an exclusive file lock, loads the state, calls fn to
// mutate it, and saves the result. The entire load→mutate→save is atomic with
// respect to other processes using the same lock.
func WithLock(path string, fn func(s *State) error) error {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lockFile.Close()

	if err := filelock.Lock(lockFile.Fd()); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer filelock.Unlock(lockFile.Fd())

	s, err := Load(path)
	if err != nil {
		return err
	}

	if err := fn(s); err != nil {
		if err == ErrNoSave {
			return nil
		}
		return err
	}

	return Save(path, s)
}

func FindStateFile(fromDir string) (string, error) {
	root, err := gitRoot(fromDir)
	if err != nil {
		root = fromDir
	}
	dd := filepath.Join(root, datadir.Name())
	path := filepath.Join(dd, "quest-state.json")
	if _, err := os.Stat(path); err != nil {
		return "", nil
	}
	// If fellowship-state.json also exists in this data directory, the CWD is
	// at the main repo root where the lead (Gandalf) runs — not inside a quest
	// worktree. Skip quest-state enforcement so the lead isn't blocked by a
	// quest runner's state file that leaked into the repo root.
	if _, err := os.Stat(filepath.Join(dd, "fellowship-state.json")); err == nil {
		return "", nil
	}
	return path, nil
}

func gitRoot(fromDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = fromDir
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
