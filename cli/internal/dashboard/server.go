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

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/group"
	"github.com/justinjdev/fellowship/cli/internal/health"
	"github.com/justinjdev/fellowship/cli/internal/notes"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/todo"
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
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
	s.mux.HandleFunc("GET /api/events/problems", s.handleProblems)
	s.mux.HandleFunc("POST /api/gate/approve", s.handleGateApprove)
	s.mux.HandleFunc("POST /api/gate/reject", s.handleGateReject)
	s.mux.HandleFunc("POST /api/group/", s.handleGroupApprove)
	s.mux.HandleFunc("GET /api/todo/", s.handleTodo)
	s.mux.HandleFunc("GET /api/notes", s.handleNotes)

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

func (s *Server) validWorktreeDir(dir string) (bool, error) {
	var valid bool
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		status, err := fellowship.DiscoverQuests(conn)
		if err != nil {
			return err
		}
		for _, q := range status.Quests {
			if q.Worktree == dir {
				valid = true
				break
			}
		}
		return nil
	})
	return valid, err
}

func (s *Server) handleGateApprove(w http.ResponseWriter, r *http.Request) {
	var req gateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if valid, err := s.validWorktreeDir(req.Dir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if !valid {
		http.Error(w, "invalid worktree directory", http.StatusBadRequest)
		return
	}

	var result fellowship.QuestStatus
	var prevPhase string
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

		prevPhase = st.Phase

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

		result = fellowship.QuestStatus{
			Name:            st.QuestName,
			Worktree:        req.Dir,
			Phase:           st.Phase,
			GatePending:     st.GatePending,
			GateID:          st.GateID,
			LembasCompleted: st.LembasCompleted,
			MetadataUpdated: st.MetadataUpdated,
		}
		return nil
	})
	if err != nil {
		if err.Error() == "no gate pending" {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Best-effort event announcements after tx commits.
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		now := time.Now().UTC().Format(time.RFC3339)
		events.Record(conn, events.Event{
			Timestamp: now, Quest: result.Name, Type: events.GateApproved,
			Phase: prevPhase, Detail: fmt.Sprintf("Gate approved for %s", prevPhase),
		})
		events.Record(conn, events.Event{
			Timestamp: now, Quest: result.Name, Type: events.PhaseTransition,
			Phase: result.Phase, Detail: fmt.Sprintf("Phase advanced from %s to %s", prevPhase, result.Phase),
		})
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleGateReject(w http.ResponseWriter, r *http.Request) {
	var req gateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if valid, err := s.validWorktreeDir(req.Dir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if !valid {
		http.Error(w, "invalid worktree directory", http.StatusBadRequest)
		return
	}

	var result fellowship.QuestStatus
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

		result = fellowship.QuestStatus{
			Name:            st.QuestName,
			Worktree:        req.Dir,
			Phase:           st.Phase,
			GatePending:     st.GatePending,
			GateID:          st.GateID,
			LembasCompleted: st.LembasCompleted,
			MetadataUpdated: st.MetadataUpdated,
		}
		return nil
	})
	if err != nil {
		if err.Error() == "no gate pending" {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Best-effort event announcement after tx commits.
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		events.Record(conn, events.Event{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     result.Name, Type: events.GateRejected,
			Phase: result.Phase, Detail: fmt.Sprintf("Gate rejected for %s", result.Phase),
		})
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleGroupApprove(w http.ResponseWriter, r *http.Request) {
	// Extract group name from path: /api/group/<name>/approve
	path := strings.TrimPrefix(r.URL.Path, "/api/group/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "approve" || parts[0] == "" {
		http.Error(w, "usage: POST /api/group/<name>/approve", http.StatusBadRequest)
		return
	}
	name := parts[0]

	type groupApproveResponse struct {
		Approved []string `json:"approved"`
		Errors   []string `json:"errors,omitempty"`
	}

	var resp groupApproveResponse
	resp.Approved = []string{}

	err := s.db.WithTx(context.Background(), func(conn *db.Conn) error {
		fs, err := fellowship.LoadFellowship(conn)
		if err != nil {
			return err
		}

		var target *fellowship.GroupEntry
		for i := range fs.Groups {
			if fs.Groups[i].Name == name {
				target = &fs.Groups[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("group not found: %s", name)
		}

		approved, errs := group.BatchApprove(conn, *target)
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
		if strings.HasPrefix(err.Error(), "group not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	opts := health.DefaultOptions()
	var report *health.HealthReport
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var sweepErr error
		report, sweepErr = health.Sweep(conn, opts)
		return sweepErr
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (s *Server) handleTodo(w http.ResponseWriter, r *http.Request) {
	// Extract base64-encoded quest name from URL: /api/todo/<base64>
	pathPart := strings.TrimPrefix(r.URL.Path, "/api/todo/")
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

	if valid, err := s.validWorktreeDir(dir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if !valid {
		http.Error(w, "invalid worktree directory", http.StatusBadRequest)
		return
	}

	var todos []todo.Todo
	err = s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		questName, findErr := state.FindQuest(conn, dir)
		if findErr != nil {
			return findErr
		}
		if questName == "" {
			return nil
		}
		var listErr error
		todos, listErr = todo.List(conn, questName)
		return listErr
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if todos == nil {
		http.Error(w, "no todo file found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todos)
}

func (s *Server) worktreeDirs() []string {
	var dirs []string
	s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		status, err := fellowship.DiscoverQuests(conn)
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

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	var evts []events.Event
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		evts, err = events.ReadAll(conn, 50)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if evts == nil {
		evts = []events.Event{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(evts)
}

func (s *Server) handleProblems(w http.ResponseWriter, r *http.Request) {
	var problems []events.Problem
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		problems, err = events.DetectProblems(conn)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if problems == nil {
		problems = []events.Problem{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problems)
}

func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request) {
	var entries []notes.Entry
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		entries, err = notes.Load(conn)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []notes.Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var status *fellowship.DashboardStatus
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var e error
		status, e = fellowship.DiscoverQuests(conn)
		return e
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status.PollInterval = s.pollInterval
	status.Phases = state.Phases()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
