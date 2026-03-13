package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	iofs "io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/bulletin"
	"github.com/justinjdev/fellowship/cli/internal/db"
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
	db           *db.DB
	pollInterval int
}

func NewServer(d *db.DB, pollInterval int) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		db:           d,
		pollInterval: pollInterval,
	}
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/eagles", s.handleEagles)
	s.mux.HandleFunc("GET /api/herald", s.handleHerald)
	s.mux.HandleFunc("GET /api/herald/problems", s.handleProblems)
	s.mux.HandleFunc("POST /api/gate/approve", s.handleGateApprove)
	s.mux.HandleFunc("POST /api/gate/reject", s.handleGateReject)
	s.mux.HandleFunc("POST /api/company/", s.handleCompanyApprove)
	s.mux.HandleFunc("GET /api/errand/", s.handleErrand)
	s.mux.HandleFunc("GET /api/bulletin", s.handleBulletin)

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
	var valid bool
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		status, err := DiscoverQuests(conn)
		if err != nil {
			return nil
		}
		for _, q := range status.Quests {
			if q.Worktree == dir {
				valid = true
				break
			}
		}
		return nil
	})
	return valid
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

	err := s.db.WithTx(context.Background(), func(conn *db.Conn) error {
		// Find the quest name for this worktree
		questName, err := state.FindQuest(conn, req.Dir)
		if err != nil || questName == "" {
			return fmt.Errorf("quest not found for worktree %s", req.Dir)
		}

		st, err := state.Load(conn, questName)
		if err != nil {
			return err
		}

		if !st.GatePending {
			return fmt.Errorf("no gate pending")
		}

		prevPhase := st.Phase

		nextPhase, err := state.NextPhase(st.Phase)
		if err != nil {
			return err
		}

		st.GatePending = false
		st.Phase = nextPhase
		st.GateID = nil
		st.LembasCompleted = false
		st.MetadataUpdated = false

		if err := state.Upsert(conn, st); err != nil {
			return err
		}

		now := time.Now().UTC().Format(time.RFC3339)
		if err := herald.Announce(conn, herald.Tiding{
			Timestamp: now, Quest: st.QuestName, Type: herald.GateApproved,
			Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
		}); err != nil {
			return err
		}
		if err := herald.Announce(conn, herald.Tiding{
			Timestamp: now, Quest: st.QuestName, Type: herald.PhaseTransition,
			Phase: st.Phase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, st.Phase),
		}); err != nil {
			return err
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
		return nil
	})
	if err != nil {
		if err.Error() == "no gate pending" {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
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

	err := s.db.WithTx(context.Background(), func(conn *db.Conn) error {
		questName, err := state.FindQuest(conn, req.Dir)
		if err != nil || questName == "" {
			return fmt.Errorf("quest not found for worktree %s", req.Dir)
		}

		st, err := state.Load(conn, questName)
		if err != nil {
			return err
		}

		if !st.GatePending {
			return fmt.Errorf("no gate pending")
		}

		st.GatePending = false
		st.GateID = nil

		if err := state.Upsert(conn, st); err != nil {
			return err
		}

		if err := herald.Announce(conn, herald.Tiding{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     st.QuestName, Type: herald.GateRejected,
			Phase: st.Phase, Detail: fmt.Sprintf("Gate rejected for %s", st.Phase),
		}); err != nil {
			return err
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
		return nil
	})
	if err != nil {
		if err.Error() == "no gate pending" {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
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

	type companyApproveResponse struct {
		Approved []string `json:"approved"`
		Errors   []string `json:"errors,omitempty"`
	}

	var resp companyApproveResponse
	resp.Approved = []string{}

	err := s.db.WithTx(context.Background(), func(conn *db.Conn) error {
		fs, err := LoadFellowship(conn)
		if err != nil {
			return err
		}

		var target *CompanyEntry
		for i := range fs.Companies {
			if fs.Companies[i].Name == name {
				target = &fs.Companies[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("company not found: %s", name)
		}

		approved, errs := batchApproveCompany(conn, *target, fs)
		resp.Approved = approved
		if resp.Approved == nil {
			resp.Approved = []string{}
		}
		for _, e := range errs {
			resp.Errors = append(resp.Errors, e.Error())
		}
		return nil
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "company not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// batchApproveCompany approves all pending gates within a company.
func batchApproveCompany(conn *db.Conn, c CompanyEntry, fs *FellowshipState) (approved []string, errs []error) {
	for _, qName := range c.Quests {
		// Find worktree from fellowship quests
		var wt string
		for _, q := range fs.Quests {
			if q.Name == qName {
				wt = q.Worktree
				break
			}
		}

		st, err := state.Load(conn, qName)
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

		if err := state.Upsert(conn, st); err != nil {
			errs = append(errs, fmt.Errorf("saving state for %s: %w", qName, err))
			continue
		}

		now := time.Now().UTC().Format(time.RFC3339)
		herald.Announce(conn, herald.Tiding{
			Timestamp: now, Quest: qName, Type: herald.GateApproved,
			Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
		})
		herald.Announce(conn, herald.Tiding{
			Timestamp: now, Quest: qName, Type: herald.PhaseTransition,
			Phase: nextPhase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, nextPhase),
		})

		_ = wt // worktree used for context but not needed for DB operations
		approved = append(approved, qName)
	}

	return approved, errs
}

func (s *Server) handleEagles(w http.ResponseWriter, r *http.Request) {
	opts := eagles.DefaultOptions()
	var report *eagles.EaglesReport
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var sweepErr error
		report, sweepErr = eagles.Sweep(conn, opts)
		return sweepErr
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (s *Server) handleErrand(w http.ResponseWriter, r *http.Request) {
	// Extract base64-encoded quest name from URL: /api/errand/<base64>
	pathPart := strings.TrimPrefix(r.URL.Path, "/api/errand/")
	if pathPart == "" {
		http.Error(w, "missing quest identifier", http.StatusBadRequest)
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

	var errands []errand.Errand
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		questName, findErr := state.FindQuest(conn, dir)
		if findErr != nil || questName == "" {
			return nil
		}
		errands, _ = errand.List(conn, questName)
		return nil
	})

	if errands == nil {
		http.Error(w, "no errand file found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(errands)
}

func (s *Server) worktreeDirs() []string {
	var dirs []string
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		status, err := DiscoverQuests(conn)
		if err != nil {
			return nil
		}
		for _, q := range status.Quests {
			dirs = append(dirs, q.Worktree)
		}
		return nil
	})
	return dirs
}

func (s *Server) handleHerald(w http.ResponseWriter, r *http.Request) {
	var tidings []herald.Tiding
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		tidings, err = herald.ReadAll(conn, 50)
		return err
	})
	if tidings == nil {
		tidings = []herald.Tiding{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tidings)
}

func (s *Server) handleProblems(w http.ResponseWriter, r *http.Request) {
	var problems []herald.Problem
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		problems = herald.DetectProblems(conn)
		return nil
	})
	if problems == nil {
		problems = []herald.Problem{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problems)
}

func (s *Server) handleBulletin(w http.ResponseWriter, r *http.Request) {
	var entries []bulletin.Entry
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		entries, err = bulletin.Load(conn)
		return err
	})
	if entries == nil {
		entries = []bulletin.Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var status *DashboardStatus
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var e error
		status, e = DiscoverQuests(conn)
		return e
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status.PollInterval = s.pollInterval
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
