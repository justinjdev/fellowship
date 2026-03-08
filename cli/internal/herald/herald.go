package herald

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
)

const heraldFile = "quest-herald.jsonl"

// TidingType represents the type of a quest tiding.
type TidingType string

const (
	GateSubmitted   TidingType = "gate_submitted"
	GateApproved    TidingType = "gate_approved"
	GateRejected    TidingType = "gate_rejected"
	PhaseTransition TidingType = "phase_transition"
	LembasCompleted TidingType = "lembas_completed"
	MetadataUpdated TidingType = "metadata_updated"
)

// Tiding represents a single quest event.
type Tiding struct {
	Timestamp string     `json:"timestamp"`
	Quest     string     `json:"quest"`
	Type      TidingType `json:"type"`
	Phase     string     `json:"phase,omitempty"`
	Detail    string     `json:"detail,omitempty"`
}

// Announce appends a tiding to the herald log file.
func Announce(dir string, t Tiding) error {
	dataDirPath := filepath.Join(dir, datadir.Name())
	if err := os.MkdirAll(dataDirPath, 0755); err != nil {
		return err
	}
	path := filepath.Join(dataDirPath, heraldFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(t)
}

// Read returns tidings from a single worktree's herald log.
// If n > 0, returns at most the last n tidings.
func Read(dir string, n int) ([]Tiding, error) {
	path := filepath.Join(dir, datadir.Name(), heraldFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Tiding{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var tidings []Tiding
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var t Tiding
		if err := json.Unmarshal(scanner.Bytes(), &t); err != nil {
			continue
		}
		tidings = append(tidings, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if n > 0 && len(tidings) > n {
		tidings = tidings[len(tidings)-n:]
	}
	return tidings, nil
}

// ReadAll aggregates tidings from multiple worktrees, sorted descending by timestamp.
// If n > 0, returns at most n tidings.
func ReadAll(dirs []string, n int) ([]Tiding, error) {
	var all []Tiding
	for _, dir := range dirs {
		tidings, err := Read(dir, 0)
		if err != nil {
			continue
		}
		all = append(all, tidings...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp > all[j].Timestamp
	})
	if n > 0 && len(all) > n {
		all = all[:n]
	}
	return all, nil
}
