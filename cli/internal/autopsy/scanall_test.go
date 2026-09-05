package autopsy

import (
	"context"
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
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
