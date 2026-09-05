package datadir

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig writes a fellowship config JSON file, creating parent dirs.
func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAutoApproveGates(t *testing.T) {
	tests := []struct {
		name    string
		project string // .fellowship/config.json body ("" = absent)
		user    string // ~/.claude/fellowship.json body ("" = absent)
		want    []string
	}{
		{
			name: "no config anywhere",
			want: nil,
		},
		{
			name:    "project config only",
			project: `{"gates":{"autoApprove":["Research","Plan"]}}`,
			want:    []string{"Research", "Plan"},
		},
		{
			name: "user config only",
			user: `{"gates":{"autoApprove":["Research"]}}`,
			want: []string{"Research"},
		},
		{
			name:    "user config overrides project config",
			project: `{"gates":{"autoApprove":["Research","Plan"]}}`,
			user:    `{"gates":{"autoApprove":["Review"]}}`,
			want:    []string{"Review"},
		},
		{
			name:    "explicit empty user list clears the project list",
			project: `{"gates":{"autoApprove":["Research"]}}`,
			user:    `{"gates":{"autoApprove":[]}}`,
			want:    []string{},
		},
		{
			name:    "user config without the key leaves the project list",
			project: `{"gates":{"autoApprove":["Research"]}}`,
			user:    `{"dataDir":".fellowship"}`,
			want:    []string{"Research"},
		},
		{
			name:    "malformed project config is ignored",
			project: `{not json`,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			root := t.TempDir()

			if tt.project != "" {
				writeConfig(t, filepath.Join(root, DefaultName, "config.json"), tt.project)
			}
			if tt.user != "" {
				writeConfig(t, filepath.Join(home, ".claude", "fellowship.json"), tt.user)
			}

			got := AutoApproveGates(root)
			if len(got) != len(tt.want) {
				t.Fatalf("AutoApproveGates() = %#v, want %#v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("AutoApproveGates() = %#v, want %#v", got, tt.want)
				}
			}
			if tt.want == nil && got != nil {
				t.Fatalf("AutoApproveGates() = %#v, want nil", got)
			}
		})
	}
}

func TestAutoApproveGates_EmptyRootSkipsProjectConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, filepath.Join(home, ".claude", "fellowship.json"), `{"gates":{"autoApprove":["Plan"]}}`)

	got := AutoApproveGates("")
	if len(got) != 1 || got[0] != "Plan" {
		t.Fatalf("AutoApproveGates(\"\") = %#v, want [Plan]", got)
	}
}
