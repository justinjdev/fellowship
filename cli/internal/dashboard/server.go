package dashboard

import (
	"encoding/json"
	"fmt"
	iofs "io/fs"
	"net/http"
	"path/filepath"
	"strings"

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
	s.mux.HandleFunc("GET /api/convoy/", s.handleConvoyApprove)

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

func (s *Server) handleConvoyApprove(w http.ResponseWriter, r *http.Request) {
	// Extract convoy name from path: /api/convoy/<name>/approve
	path := strings.TrimPrefix(r.URL.Path, "/api/convoy/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "approve" || parts[0] == "" {
		http.Error(w, "usage: GET /api/convoy/<name>/approve", http.StatusBadRequest)
		return
	}
	name := parts[0]

	statePath := filepath.Join(s.gitRoot, "tmp", "fellowship-state.json")
	fs, err := LoadFellowshipState(statePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var target *ConvoyEntry
	for i := range fs.Convoys {
		if fs.Convoys[i].Name == name {
			target = &fs.Convoys[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "convoy not found: "+name, http.StatusNotFound)
		return
	}

	approved, errs := batchApproveConvoy(*target, fs)

	type convoyApproveResponse struct {
		Approved []string `json:"approved"`
		Errors   []string `json:"errors,omitempty"`
	}

	resp := convoyApproveResponse{Approved: approved}
	if resp.Approved == nil {
		resp.Approved = []string{}
	}
	for _, e := range errs {
		resp.Errors = append(resp.Errors, e.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// batchApproveConvoy approves all pending gates within a convoy.
func batchApproveConvoy(c ConvoyEntry, fs *FellowshipState) (approved []string, errs []error) {
	questWorktree := make(map[string]string)
	for _, q := range fs.Quests {
		questWorktree[q.Name] = q.Worktree
	}

	for _, qName := range c.Quests {
		wt, ok := questWorktree[qName]
		if !ok || wt == "" {
			continue
		}

		statePath := filepath.Join(wt, "tmp", "quest-state.json")
		st, err := state.Load(statePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading state for %s: %w", qName, err))
			continue
		}

		if !st.GatePending {
			continue
		}

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

		approved = append(approved, qName)
	}

	return approved, errs
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
