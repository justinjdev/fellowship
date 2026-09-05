package autopsy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestScan_All(t *testing.T) {
	tests := []struct {
		name      string
		opts      ScanOptions
		want      int
		wantErrIn string
	}{
		{
			name:      "no filters is still an error",
			opts:      ScanOptions{},
			wantErrIn: "--all",
		},
		{
			name: "all returns every autopsy",
			opts: ScanOptions{All: true},
			want: 2,
		},
		{
			name: "all wins over a non-matching filter",
			opts: ScanOptions{All: true, Modules: []string{"nothing-matches"}},
			want: 2,
		},
		{
			name: "filter alone still narrows",
			opts: ScanOptions{Modules: []string{"auth"}},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := db.OpenTest(t)
			if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
				for _, in := range []*CreateInput{
					{Quest: "quest-1", Trigger: "recovery", Modules: []string{"auth"}, WhatFailed: "auth issue"},
					{Quest: "quest-2", Trigger: "recovery", Modules: []string{"billing"}, WhatFailed: "billing issue"},
				} {
					if _, err := Create(conn, in); err != nil {
						return err
					}
				}

				got, err := Scan(conn, tt.opts, 90)
				if tt.wantErrIn != "" {
					if err == nil || !strings.Contains(err.Error(), tt.wantErrIn) {
						t.Fatalf("Scan() error = %v, want it to mention %q", err, tt.wantErrIn)
					}
					return nil
				}
				if err != nil {
					t.Fatalf("Scan() error = %v", err)
				}
				if len(got) != tt.want {
					t.Fatalf("Scan() returned %d autopsies, want %d", len(got), tt.want)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreate_HonorsExpiryDays(t *testing.T) {
	d := db.OpenTest(t)
	var expires string
	err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		id, err := Create(conn, &CreateInput{Quest: "q", Trigger: "recovery", WhatFailed: "x", ExpiryDays: 7})
		if err != nil {
			return err
		}
		return sqlitex.Execute(conn, `SELECT expires_at FROM autopsies WHERE id = ?`, &sqlitex.ExecOptions{
			Args:       []any{id},
			ResultFunc: func(stmt *sqlite.Stmt) error { expires = stmt.ColumnText(0); return nil },
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		t.Fatalf("parse expires_at %q: %v", expires, err)
	}
	if until := time.Until(exp); until < 6*24*time.Hour || until > 8*24*time.Hour {
		t.Errorf("expires_at %s is not about 7 days out", expires)
	}
}
