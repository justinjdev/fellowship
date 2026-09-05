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
	rollbackErr := fmt.Errorf("rollback")
	err = d.WithTx(context.Background(), func(conn *Conn) error {
		if err := sqlitex.Execute(conn, `INSERT INTO quest_state (quest_name, phase, created_at, updated_at) VALUES ('test', 'Research', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, nil); err != nil {
			t.Fatal(err)
		}
		return rollbackErr
	})
	if err == nil || err.Error() != rollbackErr.Error() {
		t.Fatalf("expected rollback error, got %v", err)
	}

	// Row should not exist
	var count int
	if err := d.WithConn(context.Background(), func(conn *Conn) error {
		return sqlitex.Execute(conn, "SELECT count(*) FROM quest_state", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		})
	}); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", count)
	}
}

// A panic inside the transaction body used to skip the end-of-transaction call
// entirely, returning a connection with an open transaction to the pool. The
// transaction is now ended from a defer, and an abandoned one rolls back.
func TestWithTx_PanicRollsBackAndReleasesTheTransaction(t *testing.T) {
	d, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("the panic should propagate to the caller")
			}
		}()
		_ = d.WithTx(context.Background(), func(conn *Conn) error {
			if err := sqlitex.Execute(conn, `INSERT INTO quest_state (quest_name, phase, created_at, updated_at) VALUES ('panicky', 'Research', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, nil); err != nil {
				t.Error(err)
			}
			panic("boom")
		})
	}()

	// The write is gone, and the next transaction can still begin — which it
	// could not if the abandoned one were still open on the pooled connection.
	if err := d.WithTx(context.Background(), func(conn *Conn) error {
		var count int
		if err := sqlitex.Execute(conn, "SELECT count(*) FROM quest_state", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		}); err != nil {
			return err
		}
		if count != 0 {
			t.Errorf("rows after a panicking transaction = %d, want 0", count)
		}
		return nil
	}); err != nil {
		t.Fatalf("a transaction after the panic: %v", err)
	}
}
