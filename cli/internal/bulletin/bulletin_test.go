package bulletin

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

func TestPostAndLoad(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{
			Timestamp: "2026-01-01T00:00:00Z", Quest: "q1",
			Topic: "auth", Files: []string{"auth.go"}, Discovery: "needs refactor",
		}); err != nil {
			t.Fatal(err)
		}
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
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScan_ByFile(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{
			Quest: "q1", Topic: "auth", Files: []string{"src/auth.go"}, Discovery: "d1",
			Timestamp: "2026-01-01T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
		if err := Post(conn, Entry{
			Quest: "q2", Topic: "db", Files: []string{"src/db.go"}, Discovery: "d2",
			Timestamp: "2026-01-01T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
		matches, err := Scan(conn, []string{"src/auth.go"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1, got %d", len(matches))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestClear(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{
			Quest: "q1", Topic: "t", Discovery: "d", Timestamp: "2026-01-01T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
		if err := Clear(conn); err != nil {
			t.Fatal(err)
		}
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected 0, got %d", len(entries))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostSetsTimestamp(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{Quest: "q1", Topic: "t", Discovery: "d"}); err != nil {
			t.Fatal(err)
		}
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
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostPreservesTimestamp(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{
			Timestamp: "2026-01-01T00:00:00Z", Quest: "q", Topic: "t", Discovery: "d",
		}); err != nil {
			t.Fatal(err)
		}
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if entries[0].Timestamp != "2026-01-01T00:00:00Z" {
			t.Errorf("expected preserved timestamp, got %s", entries[0].Timestamp)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScanByTopic(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}
		if err := Post(conn, Entry{Quest: "q2", Topic: "db", Files: []string{"src/db/conn.go"}, Discovery: "d2", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}
		if err := Post(conn, Entry{Quest: "q3", Topic: "Auth", Files: []string{"src/auth/session.go"}, Discovery: "d3", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}

		entries, err := Scan(conn, nil, []string{"auth"})
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries matching topic 'auth', got %d", len(entries))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScanByFilesPathBoundary(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{Quest: "q1", Topic: "auth", Files: []string{"src/auth/jwt.go"}, Discovery: "d1", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}
		if err := Post(conn, Entry{Quest: "q2", Topic: "authz", Files: []string{"src/authz/login.go"}, Discovery: "d2", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}

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
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScanNoFilters(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Post(conn, Entry{Quest: "q1", Topic: "auth", Discovery: "d1", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}
		if err := Post(conn, Entry{Quest: "q2", Topic: "db", Discovery: "d2", Timestamp: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}

		entries, err := Scan(conn, nil, nil)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected all 2 entries with no filters, got %d", len(entries))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEmpty(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		entries, err := Load(conn)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("expected empty entries, got %v", entries)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
