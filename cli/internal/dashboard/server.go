package dashboard

import (
	"encoding/base64"
	"encoding/json"
	iofs "io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type gateRequest struct {
	Dir string `json:"dir"`
}

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
	s.mux.HandleFunc("GET /api/errand/", s.handleErrand)

	staticFS, _ := iofs.Sub(staticFiles, "static")
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

	statePath := filepath.Join(req.Dir, "tmp", "quest-state.json")
	st, err := state.Load(statePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !st.GatePending {
		http.Error(w, "no gate pending", http.StatusBadRequest)
		return
	}

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

	statePath := filepath.Join(req.Dir, "tmp", "quest-state.json")
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

	errandPath := filepath.Join(dir, "tmp", "quest-errands.json")
	h, err := errand.Load(errandPath)
	if err != nil {
		http.Error(w, "no errand file found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h)
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
