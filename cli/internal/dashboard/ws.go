package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSEvent is a notification pushed to all connected dashboard clients.
type WSEvent struct {
	Type      string   `json:"type"`
	QuestID   string   `json:"quest_id,omitempty"`
	Phase     string   `json:"phase,omitempty"`
	Action    string   `json:"action,omitempty"`
	AlertType string   `json:"alert_type,omitempty"`
	Quests    []string `json:"quests,omitempty"`
	CommandID string   `json:"command_id,omitempty"`
	Timestamp int64    `json:"timestamp"`
}

var upgrader = websocket.Upgrader{
	// Allow all origins — the dashboard binds to localhost but may be accessed
	// from different ports or via forwarded connections during development.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub manages WebSocket connections and broadcasts events.
type Hub struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[*websocket.Conn]struct{})}
}

func (h *Hub) Add(conn *websocket.Conn) {
	h.mu.Lock()
	h.conns[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, conn)
	h.mu.Unlock()
	conn.Close()
}

func (h *Hub) Broadcast(event WSEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}

	// Snapshot connections under read lock to avoid holding the lock during writes.
	h.mu.RLock()
	snapshot := make([]*websocket.Conn, 0, len(h.conns))
	for conn := range h.conns {
		snapshot = append(snapshot, conn)
	}
	h.mu.RUnlock()

	var failed []*websocket.Conn
	for _, conn := range snapshot {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			failed = append(failed, conn)
		}
	}

	if len(failed) > 0 {
		h.mu.Lock()
		for _, conn := range failed {
			delete(h.conns, conn)
			conn.Close()
		}
		h.mu.Unlock()
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}
	h.Add(conn)

	// Read pump — just drain pings/pongs, we don't expect client messages
	go func() {
		defer h.Remove(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
