package bulletin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
)

func TestPostAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	e1 := Entry{Quest: "quest-1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "JWT moved"}
	e2 := Entry{Quest: "quest-2", Topic: "db", Files: []string{"src/db/conn.go"}, Discovery: "Connection pooling changed"}

	if err := Post(path, e1); err != nil {
		t.Fatalf("Post e1: %v", err)
	}
	if err := Post(path, e2); err != nil {
		t.Fatalf("Post e2: %v", err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Quest != "quest-1" || entries[1].Quest != "quest-2" {
		t.Errorf("unexpected entries: %+v", entries)
	}
	if entries[0].Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
}

func TestPostCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "bulletin.jsonl")

	e := Entry{Quest: "q", Topic: "t", Discovery: "d"}
	if err := Post(path, e); err != nil {
		t.Fatalf("Post: %v", err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestPostPreservesTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	e := Entry{Timestamp: "2026-01-01T00:00:00Z", Quest: "q", Topic: "t", Discovery: "d"}
	if err := Post(path, e); err != nil {
		t.Fatalf("Post: %v", err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if entries[0].Timestamp != "2026-01-01T00:00:00Z" {
		t.Errorf("expected preserved timestamp, got %s", entries[0].Timestamp)
	}
}

func TestLoadNonexistent(t *testing.T) {
	entries, err := Load("/nonexistent/path/bulletin.jsonl")
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

func TestLoadSkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	content := `{"ts":"2026-01-01T00:00:00Z","quest":"q1","topic":"t","files":[],"discovery":"good"}
not json at all
{"ts":"2026-01-02T00:00:00Z","quest":"q2","topic":"t","files":[],"discovery":"also good"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping malformed), got %d", len(entries))
	}
}

func TestScanByTopic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	Post(path, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1"})
	Post(path, Entry{Quest: "q2", Topic: "db", Files: []string{"src/db/conn.go"}, Discovery: "d2"})
	Post(path, Entry{Quest: "q3", Topic: "Auth", Files: []string{"src/auth/session.go"}, Discovery: "d3"})

	entries, err := Scan(path, nil, []string{"auth"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries matching topic 'auth', got %d", len(entries))
	}
}

func TestScanByFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	Post(path, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1"})
	Post(path, Entry{Quest: "q2", Topic: "db", Files: []string{"src/db/conn.go"}, Discovery: "d2"})

	entries, err := Scan(path, []string{"src/auth/"}, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry matching files, got %d", len(entries))
	}
	if entries[0].Quest != "q1" {
		t.Errorf("expected quest q1, got %s", entries[0].Quest)
	}
}

func TestScanBothFilters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	Post(path, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1"})
	Post(path, Entry{Quest: "q2", Topic: "db", Files: []string{"src/db/conn.go"}, Discovery: "d2"})
	Post(path, Entry{Quest: "q3", Topic: "cache", Files: []string{"src/cache/redis.go"}, Discovery: "d3"})

	entries, err := Scan(path, []string{"src/db/"}, []string{"auth"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestScanNoFilters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	Post(path, Entry{Quest: "q1", Topic: "auth", Files: []string{}, Discovery: "d1"})
	Post(path, Entry{Quest: "q2", Topic: "db", Files: []string{}, Discovery: "d2"})

	entries, err := Scan(path, nil, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected all 2 entries with no filters, got %d", len(entries))
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	Post(path, Entry{Quest: "q1", Topic: "t", Discovery: "d"})

	if err := Clear(path); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("Load after clear: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries after clear, got %v", entries)
	}
}

func TestClearNonexistent(t *testing.T) {
	if err := Clear("/nonexistent/bulletin.jsonl"); err != nil {
		t.Fatalf("Clear nonexistent should not error: %v", err)
	}
}

func TestPostJSONLFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	Post(path, Entry{Quest: "q1", Topic: "t", Discovery: "d1"})
	Post(path, Entry{Quest: "q2", Topic: "t", Discovery: "d2"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestMainRepoRootFuncOverride(t *testing.T) {
	orig := mainRepoRootFunc
	defer func() { mainRepoRootFunc = orig }()

	mainRepoRootFunc = func(fromDir string) (string, error) {
		return "/fake/repo", nil
	}

	root, err := MainRepoRoot("")
	if err != nil {
		t.Fatal(err)
	}
	if root != "/fake/repo" {
		t.Errorf("expected /fake/repo, got %s", root)
	}
}

func TestPostConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulletin.jsonl")

	const writers = 16
	var wg sync.WaitGroup
	wg.Add(writers)

	for i := 0; i < writers; i++ {
		i := i
		go func() {
			defer wg.Done()
			if err := Post(path, Entry{
				Quest:     fmt.Sprintf("q-%d", i),
				Topic:     "auth",
				Files:     []string{fmt.Sprintf("src/auth/%d.go", i)},
				Discovery: "concurrent write",
			}); err != nil {
				t.Errorf("Post(%d): %v", i, err)
			}
		}()
	}

	wg.Wait()

	entries, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != writers {
		t.Fatalf("expected %d entries, got %d", writers, len(entries))
	}
}

func TestBulletinPath(t *testing.T) {
	orig := mainRepoRootFunc
	defer func() { mainRepoRootFunc = orig }()

	mainRepoRootFunc = func(fromDir string) (string, error) {
		return "/repo", nil
	}

	path, err := BulletinPath("")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join("/repo", datadir.Name(), "bulletin.jsonl")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}
