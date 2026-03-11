package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type CommandAction string

const (
	ActionSpawnQuest   CommandAction = "spawn-quest"
	ActionSpawnScout   CommandAction = "spawn-scout"
	ActionKillQuest    CommandAction = "kill-quest"
	ActionRestartQuest CommandAction = "restart-quest"
)

type CommandStatus string

const (
	StatusPending   CommandStatus = "pending"
	StatusCompleted CommandStatus = "completed"
	StatusFailed    CommandStatus = "failed"
)

type Command struct {
	ID        string          `json:"id"`
	Action    CommandAction   `json:"action"`
	Status    CommandStatus   `json:"status"`
	Params    json.RawMessage `json:"params"`
	Timestamp int64           `json:"timestamp"`
	Result    string          `json:"result,omitempty"`
}

type CommandQueue struct {
	Commands []Command `json:"commands"`
}

func commandQueuePath(gitRoot string) string {
	return filepath.Join(gitRoot, ".fellowship", "command-queue.json")
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func LoadCommandQueue(gitRoot string) (*CommandQueue, error) {
	path := commandQueuePath(gitRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CommandQueue{}, nil
		}
		return nil, err
	}
	var q CommandQueue
	if err := json.Unmarshal(data, &q); err != nil {
		return &CommandQueue{}, nil
	}
	return &q, nil
}

func SaveCommandQueue(gitRoot string, q *CommandQueue) error {
	path := commandQueuePath(gitRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func EnqueueCommand(gitRoot string, action CommandAction, params json.RawMessage) (*Command, error) {
	q, err := LoadCommandQueue(gitRoot)
	if err != nil {
		return nil, err
	}
	cmd := Command{
		ID:        generateID(),
		Action:    action,
		Status:    StatusPending,
		Params:    params,
		Timestamp: time.Now().Unix(),
	}
	q.Commands = append(q.Commands, cmd)
	if err := SaveCommandQueue(gitRoot, q); err != nil {
		return nil, err
	}
	return &cmd, nil
}
