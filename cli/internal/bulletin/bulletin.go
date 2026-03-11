package bulletin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/filelock"
)

// Entry represents a single bulletin board discovery.
type Entry struct {
	Timestamp string   `json:"ts"`
	Quest     string   `json:"quest"`
	Topic     string   `json:"topic"`
	Files     []string `json:"files"`
	Discovery string   `json:"discovery"`
}

// Post appends an entry to the bulletin JSONL file with exclusive file locking.
func Post(path string, entry Entry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}
	line = append(line, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening bulletin file: %w", err)
	}
	defer f.Close()

	if err := filelock.Lock(f.Fd()); err != nil {
		return fmt.Errorf("locking bulletin file: %w", err)
	}
	defer filelock.Unlock(f.Fd())

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("writing entry: %w", err)
	}
	return nil
}

// Load reads all entries from the bulletin JSONL file under a shared lock
// to avoid observing partially written lines from concurrent Post/Clear calls.
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening bulletin file: %w", err)
	}
	defer f.Close()

	if err := filelock.Lock(f.Fd()); err != nil {
		return nil, fmt.Errorf("locking bulletin file for read: %w", err)
	}
	defer filelock.Unlock(f.Fd())

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("reading bulletin file: %w", err)
	}
	return entries, nil
}

// Scan reads the bulletin and returns entries matching the given files or topics.
// An entry matches if any of its files have a prefix in the files list, or if its
// topic matches any of the given topics. Both filters are case-insensitive.
// If both files and topics are empty, all entries are returned.
//
// File matching is bidirectional: filter "src/auth/" matches entry file
// "src/auth/jwt.go", and filter "src/auth/jwt.go" also matches entry file
// "src/auth/". This allows both directory-level and file-level filters.
func Scan(path string, files []string, topics []string) ([]Entry, error) {
	all, err := Load(path)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 && len(topics) == 0 {
		return all, nil
	}

	// Normalize filters
	lowerTopics := make([]string, len(topics))
	for i, t := range topics {
		lowerTopics[i] = strings.ToLower(t)
	}

	var result []Entry
	for _, e := range all {
		if matchesTopic(e.Topic, lowerTopics) || matchesFiles(e.Files, files) {
			result = append(result, e)
		}
	}
	return result, nil
}

// Clear truncates the bulletin file in place under an exclusive lock,
// ensuring concurrent Post calls are not lost to an unlinked inode.
func Clear(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("opening bulletin file: %w", err)
	}
	defer f.Close()

	if err := filelock.Lock(f.Fd()); err != nil {
		return fmt.Errorf("locking bulletin file: %w", err)
	}
	defer filelock.Unlock(f.Fd())

	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("clearing bulletin: %w", err)
	}
	return nil
}

// MainRepoRoot returns the main repository root, even when called from a worktree.
// Uses git's --git-common-dir to find the shared .git directory.
func MainRepoRoot(fromDir string) (string, error) {
	return mainRepoRootFunc(fromDir)
}

var mainRepoRootFunc = func(fromDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if fromDir != "" {
		cmd.Dir = fromDir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("finding main repo root: %w", err)
	}
	gitDir := strings.TrimSpace(string(out))
	// --git-common-dir returns the .git directory; parent is the repo root
	root := filepath.Dir(gitDir)
	return root, nil
}

// BulletinPath returns the path to the bulletin JSONL file in the main repo.
func BulletinPath(fromDir string) (string, error) {
	root, err := MainRepoRoot(fromDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, datadir.Name(), "bulletin.jsonl"), nil
}

func matchesTopic(topic string, lowerTopics []string) bool {
	if len(lowerTopics) == 0 {
		return false
	}
	lt := strings.ToLower(topic)
	for _, t := range lowerTopics {
		if lt == t {
			return true
		}
	}
	return false
}

func matchesFiles(entryFiles []string, filterFiles []string) bool {
	if len(filterFiles) == 0 {
		return false
	}
	for _, ef := range entryFiles {
		ef = normalizePath(ef)
		for _, ff := range filterFiles {
			ff = normalizePath(ff)
			if pathContains(ef, ff) || pathContains(ff, ef) {
				return true
			}
		}
	}
	return false
}

// normalizePath trims whitespace, normalizes separators, and removes trailing slashes.
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

// pathContains checks if child equals parent or is nested under parent
// using path-boundary matching (separator-aware), preventing false matches
// like "src/auth" matching "src/authz/login.go".
func pathContains(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}
