package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type WorkStatus string

const (
	Pending WorkStatus = "pending"
	Active  WorkStatus = "active"
	Done    WorkStatus = "done"
	Blocked WorkStatus = "blocked"
)

type WorkItem struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Status      WorkStatus `json:"status"`
	Phase       string     `json:"phase,omitempty"`
	DependsOn   []string   `json:"depends_on,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

type QuestHook struct {
	Version   int        `json:"version"`
	QuestName string     `json:"quest_name"`
	Task      string     `json:"task"`
	Items     []WorkItem `json:"items"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

func Load(path string) (*QuestHook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading hook file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("hook file is empty")
	}
	var h QuestHook
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parsing hook file: %w", err)
	}
	return &h, nil
}

func Save(path string, h *QuestHook) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling hook: %w", err)
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

func FindHook(fromDir string) (string, error) {
	root, err := gitRoot(fromDir)
	if err != nil {
		root = fromDir
	}
	path := filepath.Join(root, "tmp", "quest-hook.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return path, nil
}

func AddItem(h *QuestHook, desc string, phase string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	id := NextID(h)
	item := WorkItem{
		ID:          id,
		Description: desc,
		Status:      Pending,
		Phase:       phase,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.Items = append(h.Items, item)
	h.UpdatedAt = now
	return id
}

func UpdateStatus(h *QuestHook, id string, status WorkStatus) error {
	for i := range h.Items {
		if h.Items[i].ID == id {
			h.Items[i].Status = status
			h.Items[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			h.UpdatedAt = h.Items[i].UpdatedAt
			return nil
		}
	}
	return fmt.Errorf("work item %q not found", id)
}

func NextID(h *QuestHook) string {
	max := 0
	for _, item := range h.Items {
		var n int
		if _, err := fmt.Sscanf(item.ID, "w-%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("w-%03d", max+1)
}

// ValidStatus checks whether a string is a valid WorkStatus.
func ValidStatus(s string) (WorkStatus, bool) {
	switch WorkStatus(s) {
	case Pending, Active, Done, Blocked:
		return WorkStatus(s), true
	default:
		return "", false
	}
}

func Progress(h *QuestHook) (done int, total int) {
	total = len(h.Items)
	for _, item := range h.Items {
		if item.Status == Done {
			done++
		}
	}
	return done, total
}

func PendingItems(h *QuestHook) []WorkItem {
	doneSet := make(map[string]bool)
	for _, item := range h.Items {
		if item.Status == Done {
			doneSet[item.ID] = true
		}
	}

	var result []WorkItem
	for _, item := range h.Items {
		if item.Status != Pending && item.Status != Blocked {
			continue
		}
		// Check if all dependencies are done
		depsOK := true
		for _, dep := range item.DependsOn {
			if !doneSet[dep] {
				depsOK = false
				break
			}
		}
		if depsOK {
			result = append(result, item)
		}
	}
	return result
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
