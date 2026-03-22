package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var queueMu sync.Mutex

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

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating command ID: %w", err)
	}
	return hex.EncodeToString(b), nil
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
		return nil, fmt.Errorf("corrupt command queue %s: %w", path, err)
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
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func lockQueueFile(gitRoot string) (*os.File, error) {
	lockPath := commandQueuePath(gitRoot) + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening queue lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring queue lock: %w", err)
	}
	return f, nil
}

func unlockQueueFile(f *os.File) {
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}

func EnqueueCommand(gitRoot string, action CommandAction, params json.RawMessage) (*Command, error) {
	queueMu.Lock()
	defer queueMu.Unlock()

	lockFile, err := lockQueueFile(gitRoot)
	if err != nil {
		return nil, err
	}
	defer unlockQueueFile(lockFile)

	q, err := LoadCommandQueue(gitRoot)
	if err != nil {
		return nil, err
	}
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	cmd := Command{
		ID:        id,
		Action:    action,
		Status:    StatusPending,
		Params:    params,
		Timestamp: time.Now().Unix(),
	}
	q.Commands = append(q.Commands, cmd)
	// Prune old commands to prevent unbounded growth.
	const maxCommands = 200
	if len(q.Commands) > maxCommands {
		q.Commands = q.Commands[len(q.Commands)-maxCommands:]
	}
	if err := SaveCommandQueue(gitRoot, q); err != nil {
		return nil, err
	}
	return &cmd, nil
}
