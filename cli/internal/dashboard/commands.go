package dashboard

import (
	"encoding/json"
	"net/http"
)

type SpawnQuestRequest struct {
	Task    string `json:"task"`
	Branch  string `json:"branch,omitempty"`
	Company string `json:"company,omitempty"`
}

type SpawnScoutRequest struct {
	Question string `json:"question"`
}

type QuestIDRequest struct {
	QuestID string `json:"quest_id"`
}

type QueuedResponse struct {
	Queued    bool   `json:"queued"`
	CommandID string `json:"command_id"`
}

func (s *Server) handleSpawnQuest(w http.ResponseWriter, r *http.Request) {
	var req SpawnQuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Task == "" {
		http.Error(w, "task is required", http.StatusBadRequest)
		return
	}
	params, _ := json.Marshal(req)
	cmd, err := EnqueueCommand(s.gitRoot, ActionSpawnQuest, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hub.Broadcast(WSEvent{Type: "command-queued", CommandID: cmd.ID})
	json.NewEncoder(w).Encode(QueuedResponse{Queued: true, CommandID: cmd.ID})
}

func (s *Server) handleSpawnScout(w http.ResponseWriter, r *http.Request) {
	var req SpawnScoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Question == "" {
		http.Error(w, "question is required", http.StatusBadRequest)
		return
	}
	params, _ := json.Marshal(req)
	cmd, err := EnqueueCommand(s.gitRoot, ActionSpawnScout, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hub.Broadcast(WSEvent{Type: "command-queued", CommandID: cmd.ID})
	json.NewEncoder(w).Encode(QueuedResponse{Queued: true, CommandID: cmd.ID})
}

func (s *Server) handleKillQuest(w http.ResponseWriter, r *http.Request) {
	var req QuestIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.QuestID == "" {
		http.Error(w, "quest_id is required", http.StatusBadRequest)
		return
	}
	params, _ := json.Marshal(req)
	cmd, err := EnqueueCommand(s.gitRoot, ActionKillQuest, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hub.Broadcast(WSEvent{Type: "command-queued", CommandID: cmd.ID})
	json.NewEncoder(w).Encode(QueuedResponse{Queued: true, CommandID: cmd.ID})
}

func (s *Server) handleRestartQuest(w http.ResponseWriter, r *http.Request) {
	var req QuestIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.QuestID == "" {
		http.Error(w, "quest_id is required", http.StatusBadRequest)
		return
	}
	params, _ := json.Marshal(req)
	cmd, err := EnqueueCommand(s.gitRoot, ActionRestartQuest, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hub.Broadcast(WSEvent{Type: "command-queued", CommandID: cmd.ID})
	json.NewEncoder(w).Encode(QueuedResponse{Queued: true, CommandID: cmd.ID})
}

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	q, err := LoadCommandQueue(s.gitRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(q.Commands)
}
