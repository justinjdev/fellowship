package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/autopsy"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

// GET /api/autopsies — list all autopsy records
// GET /api/autopsies/<filename> — single autopsy detail
func (s *Server) handleAutopsies(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/autopsies")
	suffix = strings.TrimPrefix(suffix, "/")

	// Individual autopsy lookup not supported via SQLite API; scan all and filter.
	var records []autopsy.Autopsy
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var loadErr error
		records, loadErr = autopsy.Scan(conn, autopsy.ScanOptions{}, autopsy.DefaultExpiryDays)
		return loadErr
	})

	if suffix != "" {
		// Filter to matching record by quest name
		for _, r := range records {
			if r.Quest == suffix {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(r)
				return
			}
		}
		http.Error(w, "autopsy not found", http.StatusNotFound)
		return
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	if records == nil {
		records = []autopsy.Autopsy{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// GET /api/tome/<questName> — quest tome (CV)
func (s *Server) handleTome(w http.ResponseWriter, r *http.Request) {
	questName := strings.TrimPrefix(r.URL.Path, "/api/tome/")
	if questName == "" {
		http.Error(w, "quest name required", http.StatusBadRequest)
		return
	}

	var t *tome.QuestTome
	err := s.db.WithConn(context.Background(), func(conn *db.Conn) error {
		var loadErr error
		t, loadErr = tome.Load(conn, questName)
		return loadErr
	})
	if err != nil || t == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"quest_name":       questName,
			"phases_completed": []interface{}{},
			"gate_history":     []interface{}{},
			"files_touched":    []string{},
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// GET /api/config — read fellowship config
func (s *Server) handleConfigRead(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"global":  nil,
		"project": nil,
	}

	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".claude", "fellowship.json")
		if data, err := os.ReadFile(globalPath); err == nil {
			var global interface{}
			if json.Unmarshal(data, &global) == nil {
				result["global"] = global
			}
		}
	} // silently skip global config if home directory unavailable

	projectPath := filepath.Join(s.gitRoot, ".fellowship", "config.json")
	if data, err := os.ReadFile(projectPath); err == nil {
		var project interface{}
		if json.Unmarshal(data, &project) == nil {
			result["project"] = project
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type ConfigWriteRequest struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Scope string      `json:"scope"`
}

func (s *Server) handleConfigWrite(w http.ResponseWriter, r *http.Request) {
	var req ConfigWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var configPath string
	switch req.Scope {
	case "global":
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".claude", "fellowship.json")
	case "project":
		configPath = filepath.Join(s.gitRoot, ".fellowship", "config.json")
	default:
		http.Error(w, "scope must be 'global' or 'project'", http.StatusBadRequest)
		return
	}

	existing := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			http.Error(w, "existing config file contains invalid JSON", http.StatusInternalServerError)
			return
		}
	}

	existing[req.Key] = req.Value

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		http.Error(w, "failed to create config directory", http.StatusInternalServerError)
		return
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		http.Error(w, "failed to marshal config", http.StatusInternalServerError)
		return
	}
	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		http.Error(w, "failed to write config", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmp, configPath); err != nil {
		os.Remove(tmp) // best-effort cleanup
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
