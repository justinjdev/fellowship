package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/autopsy"
	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

// GET /api/autopsies — list all autopsy records
// GET /api/autopsies/<filename> — single autopsy detail
func (s *Server) handleAutopsies(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/autopsies")
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix != "" {
		// Sanitize: only allow base filenames to prevent directory traversal
		if suffix != filepath.Base(suffix) || strings.Contains(suffix, "..") {
			http.Error(w, "invalid filename", http.StatusBadRequest)
			return
		}
		filePath := filepath.Join(s.gitRoot, datadir.Name(), "autopsies", suffix)
		data, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "autopsy not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	autopsyDir := filepath.Join(s.gitRoot, datadir.Name(), "autopsies")
	entries, err := os.ReadDir(autopsyDir)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	var records []autopsy.Autopsy
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(autopsyDir, entry.Name()))
		if err != nil {
			continue
		}
		var a autopsy.Autopsy
		if json.Unmarshal(data, &a) == nil {
			records = append(records, a)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp
	})
	json.NewEncoder(w).Encode(records)
}

// GET /api/tome/<questName> — quest tome (CV)
func (s *Server) handleTome(w http.ResponseWriter, r *http.Request) {
	questName := strings.TrimPrefix(r.URL.Path, "/api/tome/")
	if questName == "" {
		http.Error(w, "quest name required", http.StatusBadRequest)
		return
	}

	status, err := DiscoverQuests(s.gitRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var worktree string
	for _, q := range status.Quests {
		if q.Name == questName {
			worktree = q.Worktree
			break
		}
	}
	if worktree == "" {
		http.Error(w, "quest not found", http.StatusNotFound)
		return
	}

	tomePath := filepath.Join(worktree, datadir.Name(), "quest-tome.json")
	t, err := tome.Load(tomePath)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"quest_name":       questName,
			"phases_completed": []interface{}{},
			"gate_history":     []interface{}{},
			"files_touched":    []string{},
		})
		return
	}
	json.NewEncoder(w).Encode(t)
}

// GET /api/config — read fellowship config
func (s *Server) handleConfigRead(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"global":  nil,
		"project": nil,
	}

	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".claude", "fellowship.json")
	if data, err := os.ReadFile(globalPath); err == nil {
		var global interface{}
		if json.Unmarshal(data, &global) == nil {
			result["global"] = global
		}
	}

	projectPath := filepath.Join(s.gitRoot, ".fellowship", "config.json")
	if data, err := os.ReadFile(projectPath); err == nil {
		var project interface{}
		if json.Unmarshal(data, &project) == nil {
			result["project"] = project
		}
	}

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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		http.Error(w, "failed to marshal config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmp, configPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
