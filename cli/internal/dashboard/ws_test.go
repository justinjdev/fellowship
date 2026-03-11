package dashboard

import (
	"testing"
	"time"
)

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	hub.Broadcast(WSEvent{Type: "test", Timestamp: time.Now().Unix()})
}
