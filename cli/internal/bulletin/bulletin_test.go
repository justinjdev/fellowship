package bulletin

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

func TestPostAndLoad(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{
			Timestamp: "2026-01-01T00:00:00Z", Quest: "q1",
			Topic: "auth", Files: []string{"auth.go"}, Discovery: "needs refactor",
		})
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1, got %d", len(entries))
		}
		if entries[0].Topic != "auth" {
			t.Error("topic mismatch")
		}
		return nil
	})
}

func TestScan_ByFile(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{
			Quest: "q1", Topic: "auth", Files: []string{"src/auth.go"}, Discovery: "d1",
			Timestamp: "2026-01-01T00:00:00Z",
		})
		Post(conn, Entry{
			Quest: "q2", Topic: "db", Files: []string{"src/db.go"}, Discovery: "d2",
			Timestamp: "2026-01-01T00:00:00Z",
		})
		matches, _ := Scan(conn, []string{"src/auth.go"}, nil)
		if len(matches) != 1 {
			t.Fatalf("expected 1, got %d", len(matches))
		}
		return nil
	})
}

func TestClear(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{
			Quest: "q1", Topic: "t", Discovery: "d", Timestamp: "2026-01-01T00:00:00Z",
		})
		Clear(conn)
		entries, _ := Load(conn)
		if len(entries) != 0 {
			t.Fatalf("expected 0, got %d", len(entries))
		}
		return nil
	})
}

func TestPostSetsTimestamp(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{Quest: "q1", Topic: "t", Discovery: "d"})
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1, got %d", len(entries))
		}
		if entries[0].Timestamp == "" {
			t.Error("expected timestamp to be set")
		}
		return nil
	})
}

func TestPostPreservesTimestamp(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{
			Timestamp: "2026-01-01T00:00:00Z", Quest: "q", Topic: "t", Discovery: "d",
		})
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if entries[0].Timestamp != "2026-01-01T00:00:00Z" {
			t.Errorf("expected preserved timestamp, got %s", entries[0].Timestamp)
		}
		return nil
	})
}

func TestScanByTopic(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1", Timestamp: "2026-01-01T00:00:00Z"})
		Post(conn, Entry{Quest: "q2", Topic: "db", Files: []string{"src/db/conn.go"}, Discovery: "d2", Timestamp: "2026-01-01T00:00:00Z"})
		Post(conn, Entry{Quest: "q3", Topic: "Auth", Files: []string{"src/auth/session.go"}, Discovery: "d3", Timestamp: "2026-01-01T00:00:00Z"})

		entries, err := Scan(conn, nil, []string{"auth"})
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries matching topic 'auth', got %d", len(entries))
		}
		return nil
	})
}

func TestScanByFilesPathBoundary(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1", Timestamp: "2026-01-01T00:00:00Z"})
		Post(conn, Entry{Quest: "q2", Topic: "authz", Files: []string{"src/authz/login.go"}, Discovery: "d2", Timestamp: "2026-01-01T00:00:00Z"})

		// "src/auth" should match "src/auth/jwt.go" but NOT "src/authz/login.go"
		entries, err := Scan(conn, []string{"src/auth"}, nil)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry (path boundary match), got %d", len(entries))
		}
		if entries[0].Quest != "q1" {
			t.Errorf("expected quest q1, got %s", entries[0].Quest)
		}
		return nil
	})
}

func TestScanNoFilters(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		Post(conn, Entry{Quest: "q1", Topic: "auth", Discovery: "d1", Timestamp: "2026-01-01T00:00:00Z"})
		Post(conn, Entry{Quest: "q2", Topic: "db", Discovery: "d2", Timestamp: "2026-01-01T00:00:00Z"})

		entries, err := Scan(conn, nil, nil)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected all 2 entries with no filters, got %d", len(entries))
		}
		return nil
	})
}

func TestLoadEmpty(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if entries != nil {
			t.Errorf("expected nil entries, got %v", entries)
		}
		return nil
	})
}
