package db

import (
	"context"
	"fmt"
	"testing"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestOpenMemory(t *testing.T) {
	d, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Verify schema was applied — quest_state table should exist
	err = d.WithConn(context.Background(), func(conn *Conn) error {
		return sqlitex.Execute(conn, "SELECT count(*) FROM quest_state", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				_ = stmt.ColumnInt(0)
				return nil
			},
		})
	})
	if err != nil {
		t.Fatalf("schema not applied: %v", err)
	}
}

func TestOpenMemory_ForeignKeys(t *testing.T) {
	d, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Foreign keys should be enforced
	err = d.WithConn(context.Background(), func(conn *Conn) error {
		return sqlitex.Execute(conn, `INSERT INTO quest_phases (quest_name, phase, completed_at) VALUES ('nonexistent', 'Research', '2026-01-01T00:00:00Z')`, nil)
	})
	if err == nil {
		t.Fatal("expected FK violation error, got nil")
	}
}

func TestWithTx_Rollback(t *testing.T) {
	d, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Insert a row, then roll back
	_ = d.WithTx(context.Background(), func(conn *Conn) error {
		sqlitex.Execute(conn, `INSERT INTO quest_state (quest_name, phase, created_at, updated_at) VALUES ('test', 'Onboard', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, nil)
		return fmt.Errorf("rollback")
	})

	// Row should not exist
	var count int
	d.WithConn(context.Background(), func(conn *Conn) error {
		return sqlitex.Execute(conn, "SELECT count(*) FROM quest_state", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		})
	})
	if count != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", count)
	}
}
