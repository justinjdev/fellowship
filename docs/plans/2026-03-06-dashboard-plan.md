# Fellowship Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `fellowship dashboard` command that serves a live web UI for monitoring quests and approving/rejecting gates.

**Architecture:** New `cli/internal/dashboard/` package with HTTP server, embedded static assets, and JSON API endpoints. Reuses existing `state` package. Dual-source quest discovery (fellowship state file + worktree scanning fallback).

**Tech Stack:** Go stdlib (`net/http`, `embed`, `html/template`), vanilla JS, CSS

---

### Task 1: Fellowship State Types

**Files:**
- Create: `cli/internal/dashboard/fellowship.go`
- Test: `cli/internal/dashboard/fellowship_test.go`

**Step 1: Write the failing test**

```go
package dashboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFellowshipState(t *testing.T) {
	dir := t.TempDir()
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(tmpDir, 0755)
	path := filepath.Join(tmpDir, "fellowship-state.json")
	os.WriteFile(path, []byte(`{
		"name": "test-fellowship",
		"created_at": "2026-03-06T10:00:00Z",
		"quests": [
			{"name": "quest-auth", "worktree": "/tmp/wt1", "task_id": "t1"}
		],
		"scouts": [
			{"name": "scout-oauth", "task_id": "t2"}
		]
	}`), 0644)

	fs, err := LoadFellowshipState(path)
	if err != nil {
		t.Fatalf("LoadFellowshipState: %v", err)
	}
	if fs.Name != "test-fellowship" {
		t.Errorf("Name = %q, want test-fellowship", fs.Name)
	}
	if len(fs.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(fs.Quests))
	}
	if fs.Quests[0].Name != "quest-auth" {
		t.Errorf("Quests[0].Name = %q, want quest-auth", fs.Quests[0].Name)
	}
	if len(fs.Scouts) != 1 {
		t.Fatalf("len(Scouts) = %d, want 1", len(fs.Scouts))
	}
}

func TestLoadFellowshipState_Missing(t *testing.T) {
	_, err := LoadFellowshipState("/nonexistent")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestLoadFellowshipState -v`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
)

type FellowshipState struct {
	Name      string       `json:"name"`
	CreatedAt string       `json:"created_at"`
	Quests    []QuestEntry `json:"quests"`
	Scouts    []ScoutEntry `json:"scouts"`
}

type QuestEntry struct {
	Name     string `json:"name"`
	Worktree string `json:"worktree"`
	TaskID   string `json:"task_id"`
}

type ScoutEntry struct {
	Name   string `json:"name"`
	TaskID string `json:"task_id"`
}

func LoadFellowshipState(path string) (*FellowshipState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fellowship state: %w", err)
	}
	var fs FellowshipState
	if err := json.Unmarshal(data, &fs); err != nil {
		return nil, fmt.Errorf("parsing fellowship state: %w", err)
	}
	return &fs, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestLoadFellowshipState -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli/internal/dashboard/fellowship.go cli/internal/dashboard/fellowship_test.go
git commit -m "feat(dashboard): add fellowship state types and loader"
```

---

### Task 2: Quest Discovery (Worktree Scanning Fallback)

**Files:**
- Modify: `cli/internal/dashboard/fellowship.go`
- Test: `cli/internal/dashboard/fellowship_test.go`

**Step 1: Write the failing test**

```go
func TestDiscoverQuests_FromFellowshipState(t *testing.T) {
	// Set up a fellowship state file and a quest state file in a fake worktree
	root := t.TempDir()
	tmpDir := filepath.Join(root, "tmp")
	os.MkdirAll(tmpDir, 0755)

	// Create a fake worktree with quest state
	wt := t.TempDir()
	wtTmp := filepath.Join(wt, "tmp")
	os.MkdirAll(wtTmp, 0755)
	os.WriteFile(filepath.Join(wtTmp, "quest-state.json"), []byte(`{
		"version": 1, "quest_name": "quest-auth", "task_id": "t1",
		"team_name": "team", "phase": "Implement", "gate_pending": false,
		"gate_id": null, "lembas_completed": false, "metadata_updated": false,
		"auto_approve_gates": []
	}`), 0644)

	// Write fellowship state pointing to the fake worktree
	fsPath := filepath.Join(tmpDir, "fellowship-state.json")
	os.WriteFile(fsPath, []byte(fmt.Sprintf(`{
		"name": "test",
		"quests": [{"name": "quest-auth", "worktree": %q, "task_id": "t1"}],
		"scouts": []
	}`, wt)), 0644)

	status, err := DiscoverQuests(root)
	if err != nil {
		t.Fatalf("DiscoverQuests: %v", err)
	}
	if status.Name != "test" {
		t.Errorf("Name = %q, want test", status.Name)
	}
	if len(status.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(status.Quests))
	}
	if status.Quests[0].Phase != "Implement" {
		t.Errorf("Phase = %q, want Implement", status.Quests[0].Phase)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestDiscoverQuests -v`
Expected: FAIL — DiscoverQuests undefined

**Step 3: Write minimal implementation**

Add to `fellowship.go`:

```go
type QuestStatus struct {
	Name            string  `json:"name"`
	Worktree        string  `json:"worktree"`
	Phase           string  `json:"phase"`
	GatePending     bool    `json:"gate_pending"`
	GateID          *string `json:"gate_id"`
	LembasCompleted bool    `json:"lembas_completed"`
	MetadataUpdated bool    `json:"metadata_updated"`
}

type DashboardStatus struct {
	Name         string        `json:"name"`
	Quests       []QuestStatus `json:"quests"`
	Scouts       []ScoutEntry  `json:"scouts"`
	PollInterval int           `json:"poll_interval"`
}

func DiscoverQuests(gitRoot string) (*DashboardStatus, error) {
	fsPath := filepath.Join(gitRoot, "tmp", "fellowship-state.json")
	fs, err := LoadFellowshipState(fsPath)
	if err == nil {
		return discoverFromFellowshipState(fs)
	}
	return discoverFromWorktrees(gitRoot)
}

func discoverFromFellowshipState(fs *FellowshipState) (*DashboardStatus, error) {
	status := &DashboardStatus{
		Name:   fs.Name,
		Scouts: fs.Scouts,
	}
	for _, q := range fs.Quests {
		qs, err := loadQuestStatus(q.Name, q.Worktree)
		if err != nil {
			continue // worktree may not exist yet
		}
		status.Quests = append(status.Quests, *qs)
	}
	return status, nil
}

func discoverFromWorktrees(gitRoot string) (*DashboardStatus, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return &DashboardStatus{Name: "fellowship"}, nil
	}
	status := &DashboardStatus{Name: "fellowship"}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			wtPath := strings.TrimPrefix(line, "worktree ")
			qs, err := loadQuestStatus("", wtPath)
			if err != nil {
				continue
			}
			status.Quests = append(status.Quests, *qs)
		}
	}
	return status, nil
}

func loadQuestStatus(name, worktree string) (*QuestStatus, error) {
	statePath := filepath.Join(worktree, "tmp", "quest-state.json")
	s, err := state.Load(statePath)
	if err != nil {
		return nil, err
	}
	qName := name
	if qName == "" {
		qName = s.QuestName
	}
	return &QuestStatus{
		Name:            qName,
		Worktree:        worktree,
		Phase:           s.Phase,
		GatePending:     s.GatePending,
		GateID:          s.GateID,
		LembasCompleted: s.LembasCompleted,
		MetadataUpdated: s.MetadataUpdated,
	}, nil
}
```

Add imports: `"os/exec"`, `"strings"`, `"path/filepath"`, and `"github.com/justinjdev/fellowship/cli/internal/state"`.

**Step 4: Run test to verify it passes**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestDiscoverQuests -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli/internal/dashboard/fellowship.go cli/internal/dashboard/fellowship_test.go
git commit -m "feat(dashboard): add quest discovery with worktree fallback"
```

---

### Task 3: HTTP Server and API Endpoints

**Files:**
- Create: `cli/internal/dashboard/server.go`
- Test: `cli/internal/dashboard/server_test.go`

**Step 1: Write the failing test**

```go
package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	tmpDir := filepath.Join(root, "tmp")
	os.MkdirAll(tmpDir, 0755)

	wt := t.TempDir()
	wtTmp := filepath.Join(wt, "tmp")
	os.MkdirAll(wtTmp, 0755)
	os.WriteFile(filepath.Join(wtTmp, "quest-state.json"), []byte(`{
		"version":1,"quest_name":"quest-auth","task_id":"t1",
		"team_name":"team","phase":"Plan","gate_pending":true,
		"gate_id":"g1","lembas_completed":true,"metadata_updated":true,
		"auto_approve_gates":[]
	}`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "fellowship-state.json"), []byte(
		`{"name":"test","quests":[{"name":"quest-auth","worktree":"`+wt+`","task_id":"t1"}],"scouts":[]}`,
	), 0644)

	return root
}

func TestAPIStatus(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	var status DashboardStatus
	json.NewDecoder(w.Body).Decode(&status)
	if len(status.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(status.Quests))
	}
	if status.Quests[0].Phase != "Plan" {
		t.Errorf("Phase = %q, want Plan", status.Quests[0].Phase)
	}
	if !status.Quests[0].GatePending {
		t.Error("GatePending should be true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestAPIStatus -v`
Expected: FAIL — NewServer undefined

**Step 3: Write minimal implementation**

```go
package dashboard

import (
	"encoding/json"
	"net/http"
)

type Server struct {
	mux          *http.ServeMux
	gitRoot      string
	pollInterval int
}

func NewServer(gitRoot string, pollInterval int) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		gitRoot:      gitRoot,
		pollInterval: pollInterval,
	}
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/gate/approve", s.handleGateApprove)
	s.mux.HandleFunc("POST /api/gate/reject", s.handleGateReject)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := DiscoverQuests(s.gitRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status.PollInterval = s.pollInterval
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestAPIStatus -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli/internal/dashboard/server.go cli/internal/dashboard/server_test.go
git commit -m "feat(dashboard): add HTTP server with status endpoint"
```

---

### Task 4: Gate Approve/Reject Endpoints

**Files:**
- Modify: `cli/internal/dashboard/server.go`
- Modify: `cli/internal/dashboard/server_test.go`

**Step 1: Write the failing tests**

Add to `server_test.go`:

```go
func TestAPIGateApprove(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	// Get the worktree path from status first
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var status DashboardStatus
	json.NewDecoder(w.Body).Decode(&status)
	wtDir := status.Quests[0].Worktree

	// Approve the gate
	body := strings.NewReader(`{"dir":"` + wtDir + `"}`)
	req = httptest.NewRequest("POST", "/api/gate/approve", body)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// Verify state changed
	var qs QuestStatus
	json.NewDecoder(w.Body).Decode(&qs)
	if qs.Phase != "Implement" {
		t.Errorf("Phase = %q, want Implement", qs.Phase)
	}
	if qs.GatePending {
		t.Error("GatePending should be false after approve")
	}
}

func TestAPIGateReject(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var status DashboardStatus
	json.NewDecoder(w.Body).Decode(&status)
	wtDir := status.Quests[0].Worktree

	body := strings.NewReader(`{"dir":"` + wtDir + `"}`)
	req = httptest.NewRequest("POST", "/api/gate/reject", body)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	var qs QuestStatus
	json.NewDecoder(w.Body).Decode(&qs)
	if qs.Phase != "Plan" {
		t.Errorf("Phase = %q, want Plan (unchanged)", qs.Phase)
	}
	if qs.GatePending {
		t.Error("GatePending should be false after reject")
	}
}

func TestAPIGateApprove_NoPending(t *testing.T) {
	root := t.TempDir()
	tmpDir := filepath.Join(root, "tmp")
	os.MkdirAll(tmpDir, 0755)

	wt := t.TempDir()
	wtTmp := filepath.Join(wt, "tmp")
	os.MkdirAll(wtTmp, 0755)
	os.WriteFile(filepath.Join(wtTmp, "quest-state.json"), []byte(`{
		"version":1,"quest_name":"q","task_id":"t","team_name":"tm",
		"phase":"Plan","gate_pending":false,"gate_id":null,
		"lembas_completed":false,"metadata_updated":false,"auto_approve_gates":[]
	}`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "fellowship-state.json"), []byte(
		`{"name":"test","quests":[{"name":"q","worktree":"`+wt+`","task_id":"t"}],"scouts":[]}`,
	), 0644)

	srv := NewServer(root, 5)
	body := strings.NewReader(`{"dir":"` + wt + `"}`)
	req := httptest.NewRequest("POST", "/api/gate/approve", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status code = %d, want 400", w.Code)
	}
}
```

Add `"strings"` to imports.

**Step 2: Run tests to verify they fail**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestAPIGate -v`
Expected: FAIL — handleGateApprove/handleGateReject not implemented

**Step 3: Write implementation**

Add to `server.go`:

```go
import (
	"path/filepath"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

type gateRequest struct {
	Dir string `json:"dir"`
}

func (s *Server) handleGateApprove(w http.ResponseWriter, r *http.Request) {
	var req gateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	statePath := filepath.Join(req.Dir, "tmp", "quest-state.json")
	st, err := state.Load(statePath)
	if err != nil {
		http.Error(w, "cannot load quest state: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !st.GatePending {
		http.Error(w, "no gate pending", http.StatusBadRequest)
		return
	}

	nextPhase, err := state.NextPhase(st.Phase)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	st.GatePending = false
	st.Phase = nextPhase
	st.GateID = nil
	st.LembasCompleted = false
	st.MetadataUpdated = false
	if err := state.Save(statePath, st); err != nil {
		http.Error(w, "failed to save state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	qs := QuestStatus{
		Name:     st.QuestName,
		Worktree: req.Dir,
		Phase:    st.Phase,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(qs)
}

func (s *Server) handleGateReject(w http.ResponseWriter, r *http.Request) {
	var req gateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	statePath := filepath.Join(req.Dir, "tmp", "quest-state.json")
	st, err := state.Load(statePath)
	if err != nil {
		http.Error(w, "cannot load quest state: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !st.GatePending {
		http.Error(w, "no gate pending", http.StatusBadRequest)
		return
	}

	st.GatePending = false
	st.GateID = nil
	if err := state.Save(statePath, st); err != nil {
		http.Error(w, "failed to save state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	qs := QuestStatus{
		Name:     st.QuestName,
		Worktree: req.Dir,
		Phase:    st.Phase,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(qs)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/justin/git/fellowship/cli && go test ./internal/dashboard/ -run TestAPIGate -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli/internal/dashboard/server.go cli/internal/dashboard/server_test.go
git commit -m "feat(dashboard): add gate approve/reject endpoints"
```

---

### Task 5: Embedded Static Assets (HTML, CSS, JS)

**Files:**
- Create: `cli/internal/dashboard/static/index.html`
- Create: `cli/internal/dashboard/static/style.css`
- Create: `cli/internal/dashboard/static/app.js`
- Create: `cli/internal/dashboard/embed.go`

**Step 1: Create the HTML page**

`cli/internal/dashboard/static/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Fellowship Dashboard</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link href="https://fonts.googleapis.com/css2?family=Cinzel:wght@400;700&family=Crimson+Text:ital,wght@0,400;0,600;1,400&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <header id="header">
        <h1 id="fellowship-name">The Fellowship</h1>
        <p id="fellowship-meta" class="meta"></p>
        <span id="poll-indicator" class="poll-indicator"></span>
    </header>
    <main id="quests"></main>
    <section id="activity">
        <h2>Activity</h2>
        <ul id="activity-feed"></ul>
    </section>
    <script src="/static/app.js"></script>
</body>
</html>
```

**Step 2: Create the CSS**

`cli/internal/dashboard/static/style.css` — Tolkien-minimal dark aesthetic with Cinzel headings, Crimson Text body, earth tones, amber accents, knotwork header border. Full CSS to be written during implementation — key design tokens:

```css
:root {
    --bg-deep: #1a1510;
    --bg-card: #2a2118;
    --bg-card-pending: #2e2214;
    --text-primary: #d4c4a8;
    --text-secondary: #8a7d6b;
    --gold: #c8a84e;
    --gold-dim: #8a7a3e;
    --amber-glow: rgba(200, 168, 78, 0.15);
    --green: #5a8a5a;
    --red: #8a4a4a;
    --font-heading: 'Cinzel', serif;
    --font-body: 'Crimson Text', serif;
}
```

Styles for: body, header (knotwork bottom border), quest cards, progress bars, gate buttons, activity feed, poll pulse animation, pending glow animation.

**Step 3: Create the JS**

`cli/internal/dashboard/static/app.js` — Vanilla JS that:
- Reads `poll_interval` from first `/api/status` response
- Polls `/api/status` every N seconds via `setInterval` + `fetch`
- Renders quest cards with phase progress bars (6 phases)
- Shows approve/reject buttons on pending gates
- Reject button shows inline confirmation
- Sends POST to `/api/gate/approve` or `/api/gate/reject`
- On gate action response, re-renders affected card immediately
- Maintains client-side activity feed (detects changes between polls)
- Pulse animation on poll indicator during fetch

Key functions: `poll()`, `render(status)`, `renderCard(quest)`, `approve(dir)`, `reject(dir)`, `addActivity(msg)`

**Step 4: Create the embed file**

`cli/internal/dashboard/embed.go`:
```go
package dashboard

import "embed"

//go:embed static
var staticFiles embed.FS
```

**Step 5: Wire static file serving into the server**

Add to `NewServer()` in `server.go`:
```go
import "io/fs"

// In NewServer:
staticFS, _ := fs.Sub(staticFiles, "static")
s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    data, _ := staticFiles.ReadFile("static/index.html")
    w.Header().Set("Content-Type", "text/html")
    w.Write(data)
})
```

**Step 6: Commit**

```bash
git add cli/internal/dashboard/static/ cli/internal/dashboard/embed.go cli/internal/dashboard/server.go
git commit -m "feat(dashboard): add embedded static assets (HTML, CSS, JS)"
```

---

### Task 6: Dashboard CLI Subcommand

**Files:**
- Modify: `cli/cmd/fellowship/main.go`

**Step 1: Add dashboard case to main switch**

Add to `main.go` switch statement after `case "init"`:

```go
case "dashboard":
    os.Exit(runDashboard(os.Args[2:]))
```

**Step 2: Write runDashboard function**

```go
import (
	"flag"
	"net/http"
	"runtime"

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
)

func runDashboard(args []string) int {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	port := fs.Int("port", 3000, "HTTP port")
	poll := fs.Int("poll", 5, "Poll interval in seconds")
	fs.Parse(args)

	root := gitRootOrCwd()
	srv := dashboard.NewServer(root, *poll)

	addr := fmt.Sprintf("localhost:%d", *port)
	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Fellowship dashboard: %s\n", url)

	// Open browser
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	}

	if err := http.ListenAndServe(addr, srv); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	return 0
}
```

**Step 3: Update usage string**

Add to `usage()`:
```
  dashboard              Start live web dashboard
    --port N             HTTP port (default: 3000)
    --poll N             Poll interval in seconds (default: 5)
```

**Step 4: Smoke test manually**

Run: `cd /Users/justin/git/fellowship/cli && go run ./cmd/fellowship dashboard --port 3001`
Expected: prints URL, opens browser, serves dashboard page (may show empty state)

**Step 5: Commit**

```bash
git add cli/cmd/fellowship/main.go
git commit -m "feat(dashboard): add dashboard subcommand to CLI"
```

---

### Task 7: Run Full Test Suite

**Files:** None (verification only)

**Step 1: Run all existing tests**

Run: `cd /Users/justin/git/fellowship/cli && go test ./... -v`
Expected: all tests pass, including new dashboard tests and existing state/hooks/install tests

**Step 2: Run vet and build**

Run: `cd /Users/justin/git/fellowship/cli && go vet ./... && go build ./cmd/fellowship`
Expected: no warnings, successful build

**Step 3: Manual integration test**

1. In a test project with worktrees and quest state files, run `fellowship dashboard`
2. Verify dashboard loads in browser
3. Verify quest cards render with correct phases
4. If a gate is pending, verify approve/reject buttons work
5. Verify polling updates the UI

**Step 4: Commit any fixes**

```bash
git commit -am "fix(dashboard): address issues found in integration testing"
```

---

### Task 8: Update Issue #13

**Step 1: Update the GitHub issue**

Reference the commits and mark the implementation as complete. Link to the design doc.

```bash
gh issue comment 13 --body "Implementation complete. See docs/plans/2026-03-06-dashboard-design.md for design."
```
