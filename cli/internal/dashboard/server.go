package dashboard

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	iofs "io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/bulletin"
	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/eagles"
	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type gateRequest struct {
	Dir string `json:"dir"`
}

type Server struct {
	mux          *http.ServeMux
	gitRoot      string
	pollInterval int
	hub          *Hub
}

func NewServer(gitRoot string, pollInterval int) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		gitRoot:      gitRoot,
		pollInterval: pollInterval,
		hub:          NewHub(),
	}
	s.mux.HandleFunc("GET /ws", s.hub.HandleWS)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/eagles", s.handleEagles)
	s.mux.HandleFunc("GET /api/herald", s.handleHerald)
	s.mux.HandleFunc("GET /api/herald/problems", s.handleProblems)
	s.mux.HandleFunc("POST /api/gate/approve", s.handleGateApprove)
	s.mux.HandleFunc("POST /api/gate/reject", s.handleGateReject)
	s.mux.HandleFunc("POST /api/company/", s.handleCompanyApprove)
	s.mux.HandleFunc("GET /api/errand/", s.handleErrand)
	s.mux.HandleFunc("GET /api/bulletin", s.handleBulletin)
	s.mux.HandleFunc("POST /api/quest/spawn", s.handleSpawnQuest)
	s.mux.HandleFunc("POST /api/quest/kill", s.handleKillQuest)
	s.mux.HandleFunc("POST /api/quest/restart", s.handleRestartQuest)
	s.mux.HandleFunc("POST /api/scout/spawn", s.handleSpawnScout)
	s.mux.HandleFunc("GET /api/commands", s.handleCommands)
	s.mux.HandleFunc("GET /api/autopsies/", s.handleAutopsies)
	s.mux.HandleFunc("GET /api/autopsies", s.handleAutopsies)
	s.mux.HandleFunc("GET /api/tome/", s.handleTome)
	s.mux.HandleFunc("GET /api/config", s.handleConfigRead)
	s.mux.HandleFunc("POST /api/config", s.handleConfigWrite)

	staticFS, err := iofs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("dashboard: failed to load static assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
			http.NotFound(w, r)
			return
		}

		// Try to serve static file first
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := staticFS.Open(path); err == nil {
			stat, statErr := f.Stat()
			f.Close()
			if statErr == nil && !stat.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for client-side routes
		data, _ := staticFiles.ReadFile("static/index.html")
		if data == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) validWorktreeDir(dir string) bool {
	status, err := DiscoverQuests(s.gitRoot)
	if err != nil {
		return false
	}
	for _, q := range status.Quests {
		if q.Worktree == dir {
			return true
		}
	}
	return false
}

func (s *Server) handleGateApprove(w http.ResponseWriter, r *http.Request) {
	var req gateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !s.validWorktreeDir(req.Dir) {
		http.Error(w, "invalid worktree directory", http.StatusBadRequest)
		return
	}

	statePath := filepath.Join(req.Dir, datadir.Name(), "quest-state.json")
	st, err := state.Load(statePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !st.GatePending {
		http.Error(w, "no gate pending", http.StatusBadRequest)
		return
	}

	prevPhase := st.Phase

	nextPhase, err := state.NextPhase(st.Phase)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	st.GatePending = false
	st.Phase = nextPhase
	st.GateID = nil
	st.LembasCompleted = false
	st.MetadataUpdated = false

	if err := state.Save(statePath, st); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	ts := now.Unix()
	s.hub.Broadcast(WSEvent{Type: "gate-resolved", QuestID: st.QuestName, Action: "approved", Timestamp: ts})
	s.hub.Broadcast(WSEvent{Type: "quest-changed", QuestID: st.QuestName, Timestamp: ts})

	nowStr := now.Format(time.RFC3339)
	herald.Announce(req.Dir, herald.Tiding{
		Timestamp: nowStr, Quest: st.QuestName, Type: herald.GateApproved,
		Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
	})
	herald.Announce(req.Dir, herald.Tiding{
		Timestamp: nowStr, Quest: st.QuestName, Type: herald.PhaseTransition,
		Phase: st.Phase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, st.Phase),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(QuestStatus{
		Name:            st.QuestName,
		Worktree:        req.Dir,
		Phase:           st.Phase,
		GatePending:     st.GatePending,
		GateID:          st.GateID,
		LembasCompleted: st.LembasCompleted,
		MetadataUpdated: st.MetadataUpdated,
	})
}

func (s *Server) handleGateReject(w http.ResponseWriter, r *http.Request) {
	var req gateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !s.validWorktreeDir(req.Dir) {
		http.Error(w, "invalid worktree directory", http.StatusBadRequest)
		return
	}

	statePath := filepath.Join(req.Dir, datadir.Name(), "quest-state.json")
	st, err := state.Load(statePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !st.GatePending {
		http.Error(w, "no gate pending", http.StatusBadRequest)
		return
	}

	st.GatePending = false
	st.GateID = nil

	if err := state.Save(statePath, st); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rejectTS := time.Now().UTC()
	s.hub.Broadcast(WSEvent{Type: "gate-resolved", QuestID: st.QuestName, Action: "rejected", Timestamp: rejectTS.Unix()})
	s.hub.Broadcast(WSEvent{Type: "quest-changed", QuestID: st.QuestName, Timestamp: rejectTS.Unix()})

	herald.Announce(req.Dir, herald.Tiding{
		Timestamp: rejectTS.Format(time.RFC3339),
		Quest: st.QuestName, Type: herald.GateRejected,
		Phase: st.Phase, Detail: fmt.Sprintf("Gate rejected for %s", st.Phase),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(QuestStatus{
		Name:            st.QuestName,
		Worktree:        req.Dir,
		Phase:           st.Phase,
		GatePending:     st.GatePending,
		GateID:          st.GateID,
		LembasCompleted: st.LembasCompleted,
		MetadataUpdated: st.MetadataUpdated,
	})
}

func (s *Server) handleCompanyApprove(w http.ResponseWriter, r *http.Request) {
	// Extract company name from path: /api/company/<name>/approve
	path := strings.TrimPrefix(r.URL.Path, "/api/company/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "approve" || parts[0] == "" {
		http.Error(w, "usage: POST /api/company/<name>/approve", http.StatusBadRequest)
		return
	}
	name := parts[0]

	statePath := filepath.Join(s.gitRoot, datadir.Name(), "fellowship-state.json")
	fs, err := LoadFellowshipState(statePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var target *CompanyEntry
	for i := range fs.Companies {
		if fs.Companies[i].Name == name {
			target = &fs.Companies[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "company not found: "+name, http.StatusNotFound)
		return
	}

	approved, errs := batchApproveCompany(*target, fs, s.hub)

	type companyApproveResponse struct {
		Approved []string `json:"approved"`
		Errors   []string `json:"errors,omitempty"`
	}

	resp := companyApproveResponse{Approved: approved}
	if resp.Approved == nil {
		resp.Approved = []string{}
	}
	for _, e := range errs {
		resp.Errors = append(resp.Errors, e.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// batchApproveCompany approves all pending gates within a company.
func batchApproveCompany(c CompanyEntry, fs *FellowshipState, hub *Hub) (approved []string, errs []error) {
	questWorktree := make(map[string]string)
	for _, q := range fs.Quests {
		questWorktree[q.Name] = q.Worktree
	}

	for _, qName := range c.Quests {
		wt, ok := questWorktree[qName]
		if !ok || wt == "" {
			continue
		}

		statePath := filepath.Join(wt, datadir.Name(), "quest-state.json")
		st, err := state.Load(statePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading state for %s: %w", qName, err))
			continue
		}

		if !st.GatePending {
			continue
		}

		prevPhase := st.Phase

		nextPhase, err := state.NextPhase(st.Phase)
		if err != nil {
			errs = append(errs, fmt.Errorf("advancing phase for %s: %w", qName, err))
			continue
		}

		st.GatePending = false
		st.Phase = nextPhase
		st.GateID = nil
		st.LembasCompleted = false
		st.MetadataUpdated = false

		if err := state.Save(statePath, st); err != nil {
			errs = append(errs, fmt.Errorf("saving state for %s: %w", qName, err))
			continue
		}

		now := time.Now().UTC().Format(time.RFC3339)
		herald.Announce(wt, herald.Tiding{
			Timestamp: now, Quest: qName, Type: herald.GateApproved,
			Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
		})
		herald.Announce(wt, herald.Tiding{
			Timestamp: now, Quest: qName, Type: herald.PhaseTransition,
			Phase: nextPhase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, nextPhase),
		})

		if hub != nil {
			batchTS := time.Now().Unix()
			hub.Broadcast(WSEvent{Type: "gate-resolved", QuestID: qName, Action: "approved", Timestamp: batchTS})
			hub.Broadcast(WSEvent{Type: "quest-changed", QuestID: qName, Timestamp: batchTS})
		}

		approved = append(approved, qName)
	}

	return approved, errs
}

func (s *Server) handleEagles(w http.ResponseWriter, r *http.Request) {
	opts := eagles.DefaultOptions()
	report, err := eagles.Sweep(s.gitRoot, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (s *Server) handleErrand(w http.ResponseWriter, r *http.Request) {
	// Extract base64-encoded worktree path from URL: /api/errand/<base64>
	pathPart := strings.TrimPrefix(r.URL.Path, "/api/errand/")
	if pathPart == "" {
		http.Error(w, "missing worktree path", http.StatusBadRequest)
		return
	}

	dirBytes, err := base64.URLEncoding.DecodeString(pathPart)
	if err != nil {
		http.Error(w, "invalid base64 path", http.StatusBadRequest)
		return
	}
	dir := string(dirBytes)

	if !s.validWorktreeDir(dir) {
		http.Error(w, "invalid worktree directory", http.StatusBadRequest)
		return
	}

	errandPath := filepath.Join(dir, datadir.Name(), "quest-errands.json")
	h, err := errand.Load(errandPath)
	if err != nil {
		http.Error(w, "no errand file found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h)
}

func (s *Server) worktreeDirs() []string {
	status, err := DiscoverQuests(s.gitRoot)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, q := range status.Quests {
		dirs = append(dirs, q.Worktree)
	}
	return dirs
}

func (s *Server) handleHerald(w http.ResponseWriter, r *http.Request) {
	worktrees := s.worktreeDirs()
	tidings, err := herald.ReadAll(worktrees, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tidings == nil {
		tidings = []herald.Tiding{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tidings)
}

func (s *Server) handleProblems(w http.ResponseWriter, r *http.Request) {
	worktrees := s.worktreeDirs()
	problems := herald.DetectProblems(worktrees)
	if problems == nil {
		problems = []herald.Problem{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problems)
}

func (s *Server) handleBulletin(w http.ResponseWriter, r *http.Request) {
	bulletinPath := filepath.Join(s.gitRoot, datadir.Name(), "bulletin.jsonl")
	entries, err := bulletin.Load(bulletinPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []bulletin.Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
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
