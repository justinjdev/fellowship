package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestParseGateArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    gateArgs
		wantErr string
	}{
		{
			name: "status without flags",
			args: []string{"status"},
			want: gateArgs{sub: "status"},
		},
		{
			name: "approve with --dir",
			args: []string{"approve", "--dir", "/tmp/wt"},
			want: gateArgs{sub: "approve", dir: "/tmp/wt"},
		},
		{
			name: "reject with single-dash dir",
			args: []string{"reject", "-dir", "/tmp/wt"},
			want: gateArgs{sub: "reject", dir: "/tmp/wt"},
		},
		{
			name: "dir with equals form",
			args: []string{"status", "--dir=/tmp/wt"},
			want: gateArgs{sub: "status", dir: "/tmp/wt"},
		},
		{
			name:    "no subcommand",
			args:    nil,
			wantErr: "usage: fellowship gate",
		},
		{
			name:    "unknown subcommand",
			args:    []string{"bless"},
			wantErr: "unknown gate command: bless",
		},
		{
			name:    "unknown flag returns error instead of exiting",
			args:    []string{"approve", "--worktree", "/tmp/wt"},
			wantErr: "flag provided but not defined",
		},
		{
			name:    "missing value for --dir",
			args:    []string{"approve", "--dir"},
			wantErr: "flag needs an argument",
		},
		{
			name:    "unexpected positional argument",
			args:    []string{"approve", "quest-1"},
			wantErr: `unexpected argument "quest-1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGateArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseGateArgs(%q) = %+v, want error containing %q", tt.args, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGateArgs(%q): unexpected error: %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parseGateArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestValidateAutoApproveGates(t *testing.T) {
	tests := []struct {
		name    string
		gates   []string
		wantErr string
	}{
		{name: "nil is valid"},
		{name: "empty is valid", gates: []string{}},
		{name: "every gate-bearing phase", gates: []string{"Research", "Plan", "Implement"}},
		{name: "the terminal phase is rejected", gates: []string{"Review"}, wantErr: `invalid gates.autoApprove entry "Review"`},
		{name: "retired phase names are rejected", gates: []string{"Onboard"}, wantErr: `invalid gates.autoApprove entry "Onboard"`},
		{name: "unknown phase is rejected", gates: []string{"Ship"}, wantErr: `invalid gates.autoApprove entry "Ship"`},
		{name: "case must match", gates: []string{"research"}, wantErr: `invalid gates.autoApprove entry "research"`},
		{name: "one bad entry fails the list", gates: []string{"Plan", "Deploy"}, wantErr: `invalid gates.autoApprove entry "Deploy"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAutoApproveGates(tt.gates)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAutoApproveGates(%q) = %v, want nil", tt.gates, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateAutoApproveGates(%q) = %v, want error containing %q", tt.gates, err, tt.wantErr)
			}
		})
	}
}

func TestAutoApprovablePhases(t *testing.T) {
	got := autoApprovablePhases()
	for _, p := range got {
		if p == state.TerminalPhase {
			t.Fatalf("autoApprovablePhases() = %q, must not contain the terminal phase %q", got, state.TerminalPhase)
		}
	}
	if len(got) != len(state.Phases())-1 {
		t.Fatalf("autoApprovablePhases() = %q, want every phase but %q (%q)", got, state.TerminalPhase, state.Phases())
	}
}

// initGitRepo creates a git repository with one commit and returns its root.
func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	// macOS temp dirs are symlinked; resolve so git's toplevel matches.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-qm", "init")
	return root
}

// registerQuest records a quest in fellowship state, as `state add-quest` does.
func registerQuest(t *testing.T, d *db.DB, name, worktree string) {
	t.Helper()
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return fellowship.AddQuest(conn, fellowship.QuestEntry{
			Name:            name,
			TaskDescription: "test quest",
			Worktree:        worktree,
		})
	}); err != nil {
		t.Fatalf("registering quest: %v", err)
	}
}

func TestResolveDirQuest(t *testing.T) {
	root := initGitRepo(t)
	sub := filepath.Join(root, "pkg", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	d := db.OpenTest(t)
	registerQuest(t, d, "quest-1", root)

	tests := []struct {
		name string
		dir  string
		want string
	}{
		{name: "worktree root", dir: root, want: "quest-1"},
		{name: "subdirectory resolves via git root", dir: sub, want: "quest-1"},
		{name: "unregistered directory", dir: t.TempDir(), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDirQuest(d, tt.dir); got != tt.want {
				t.Errorf("resolveDirQuest(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestResolveInitQuestName(t *testing.T) {
	root := initGitRepo(t)
	unregistered := initGitRepo(t)

	d := db.OpenTest(t)
	registerQuest(t, d, "q1", root)

	tests := []struct {
		name      string
		flagName  string
		root      string
		want      string
		wantIsDir bool
	}{
		{
			name:     "explicit --quest wins",
			flagName: "explicit",
			root:     root,
			want:     "explicit",
		},
		{
			name:     "falls back to the registered quest name",
			flagName: "",
			root:     root,
			want:     "q1",
		},
		{
			name:      "falls back to the directory basename",
			flagName:  "",
			root:      unregistered,
			wantIsDir: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveInitQuestName(d, tt.flagName, tt.root)
			want := tt.want
			if tt.wantIsDir {
				want = filepath.Base(tt.root)
			}
			if got != want {
				t.Errorf("resolveInitQuestName(%q, %q) = %q, want %q", tt.flagName, tt.root, got, want)
			}
		})
	}
}

func TestCheckDir(t *testing.T) {
	repo := initGitRepo(t)
	other := initGitRepo(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{name: "empty dir is a no-op"},
		{name: "same repo", dir: repo},
		{name: "subdirectory of the same repo", dir: filepath.Join(repo, ".git")},
		{name: "different repo", dir: other, wantErr: "different repository"},
		{name: "missing directory", dir: filepath.Join(repo, "nope"), wantErr: "is not a directory"},
		{name: "file instead of directory", dir: filepath.Join(repo, "README.md"), wantErr: "is not a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDir(tt.dir)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDir(%q) = %v, want nil", tt.dir, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("checkDir(%q) = %v, want error containing %q", tt.dir, err, tt.wantErr)
			}
		})
	}
}

// hold/unhold used to fall back to the directory's basename when no quest was
// registered for --dir, which held whatever quest happened to share that name
// (or failed later with a "quest not found" naming a directory).
func TestResolveHoldQuest(t *testing.T) {
	root := initGitRepo(t)
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	unregistered := initGitRepo(t)

	d := db.OpenTest(t)
	registerQuest(t, d, "quest-1", root)

	tests := []struct {
		name    string
		dir     string
		want    string
		wantErr string
	}{
		{name: "registered worktree", dir: root, want: "quest-1"},
		{name: "subdirectory resolves via the git root", dir: sub, want: "quest-1"},
		{name: "unregistered directory is an error", dir: unregistered, wantErr: "no quest is registered"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveHoldQuest(d, tt.dir)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveHoldQuest(%q) = (%q, %v), want an error containing %q", tt.dir, got, err, tt.wantErr)
				}
				if got != "" {
					t.Errorf("quest name = %q, want empty on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveHoldQuest(%q): %v", tt.dir, err)
			}
			if got != tt.want {
				t.Errorf("resolveHoldQuest(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

// extractJSONFlag exists because Go's flag package stops recognizing flags at
// the first positional argument, so "group show <name> --json" would
// otherwise leave --json unparsed and silently fall back to table output.
func TestExtractJSONFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantRest []string
	}{
		{"flag before positional", []string{"--json", "smoke-co"}, true, []string{"smoke-co"}},
		{"flag after positional", []string{"smoke-co", "--json"}, true, []string{"smoke-co"}},
		{"no flag", []string{"smoke-co"}, false, []string{"smoke-co"}},
		{"only flag", []string{"--json"}, true, nil},
		{"empty", nil, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, gotRest := extractJSONFlag(tt.args)
			if gotJSON != tt.wantJSON {
				t.Errorf("extractJSONFlag(%v) json = %v, want %v", tt.args, gotJSON, tt.wantJSON)
			}
			if len(gotRest) != len(tt.wantRest) {
				t.Fatalf("extractJSONFlag(%v) rest = %v, want %v", tt.args, gotRest, tt.wantRest)
			}
			for i := range gotRest {
				if gotRest[i] != tt.wantRest[i] {
					t.Errorf("extractJSONFlag(%v) rest = %v, want %v", tt.args, gotRest, tt.wantRest)
				}
			}
		})
	}
}
