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

// wsConn wraps a websocket.Conn with a per-connection write mutex and
// idempotent Close. gorilla/websocket supports one concurrent reader and
// one concurrent writer — the write mutex serializes all writes to this
// connection across concurrent Broadcast calls.
type wsConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	once    sync.Once
}

func (c *wsConn) writeMessage(messageType int, data []byte, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.conn.SetWriteDeadline(deadline)
	return c.conn.WriteMessage(messageType, data)
}

func (c *wsConn) close() {
	c.once.Do(func() { c.conn.Close() })
}

// Hub manages WebSocket connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	conns   map[*wsConn]struct{}
	logFunc func(source, handler, message string)
}

func NewHub() *Hub {
	return &Hub{conns: make(map[*wsConn]struct{})}
}

// SetLogFunc sets the error logging callback for the hub.
func (h *Hub) SetLogFunc(fn func(source, handler, message string)) {
	h.logFunc = fn
}

func (h *Hub) add(c *wsConn) {
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *wsConn) {
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
	c.close()
}

func (h *Hub) Broadcast(event WSEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		// Skip logFunc for "error-logged" events to prevent infinite recursion:
		// logError -> Broadcast("error-logged") -> marshal fails -> logFunc -> logError -> ...
		if h.logFunc != nil && event.Type != "error-logged" {
			h.logFunc("websocket", "Broadcast", "marshal error: "+err.Error())
		}
		return
	}

	// Snapshot connections under read lock to avoid holding the lock during writes.
	h.mu.RLock()
	snapshot := make([]*wsConn, 0, len(h.conns))
	for c := range h.conns {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	var failed []*wsConn
	deadline := time.Now().Add(5 * time.Second)
	for _, c := range snapshot {
		if err := c.writeMessage(websocket.TextMessage, data, deadline); err != nil {
			failed = append(failed, c)
		}
	}

	if len(failed) > 0 {
		h.mu.Lock()
		for _, c := range failed {
			delete(h.conns, c)
			c.close()
		}
		h.mu.Unlock()
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	raw, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		if h.logFunc != nil {
			h.logFunc("websocket", "HandleWS", "upgrade error: "+err.Error())
		}
		return
	}
	raw.SetReadLimit(4096) // we don't expect meaningful client messages
	c := &wsConn{conn: raw}
	h.add(c)

	// Read pump — just drain pings/pongs, we don't expect client messages
	go func() {
		defer h.remove(c)
		for {
			if _, _, err := raw.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
