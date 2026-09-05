package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// --- fixtures ---------------------------------------------------------------

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// newMainRepo creates a git repo with one commit and returns its root.
func newMainRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-qm", "init")
	// t.TempDir() may itself sit behind a symlink; hooks compare resolved paths.
	return state.CanonicalWorktree(root)
}

// addWorktree creates a real git worktree of root and returns its path.
func addWorktree(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(root), name)
	git(t, root, "worktree", "add", "-q", "-b", name, path)
	return state.CanonicalWorktree(path)
}

// fellowshipWith opens an in-memory store holding a fellowship rooted at root,
// plus the quests described by quests (name -> registered worktree path).
func fellowshipWith(t *testing.T, root string, quests map[string]string) *db.DB {
	t.Helper()
	d := db.OpenTest(t)
	err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := dashboard.InitFellowship(conn, "test-fellowship", root, "main"); err != nil {
			return err
		}
		for name, wt := range quests {
			if err := dashboard.AddQuest(conn, dashboard.QuestEntry{
				Name: name, TaskDescription: "t", Worktree: wt, Branch: name,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func setQuestState(t *testing.T, d *db.DB, s *state.State) {
	t.Helper()
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, s)
	}); err != nil {
		t.Fatal(err)
	}
}

func bashInput(command string) io.Reader {
	return strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":` + quote(command) + `}}`)
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// --- store-open posture -----------------------------------------------------

// A repo with no fellowship store is an ordinary repo: hooks allow, and merely
// invoking the binary must not create a store.
func TestStoreOpen_NoStoreAllowsHooksAndCreatesNothing(t *testing.T) {
	root := newMainRepo(t)

	d, err := openStore(root, []string{"hook", "gate-guard"})
	if err == nil {
		d.Close()
		t.Fatal("expected no-store error")
	}
	if got := storeOpenExit(root, []string{"hook", "gate-guard"}, err); got != 0 {
		t.Errorf("gate-guard with no store: exit %d, want 0 (allow)", got)
	}
	if got := storeOpenExit(root, []string{"hook", "worktree-guard"}, err); got != 0 {
		t.Errorf("worktree-guard with no store: exit %d, want 0 (allow)", got)
	}
	if got := storeOpenExit(root, []string{"status"}, err); got != 1 {
		t.Errorf("status with no store: exit %d, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".fellowship")); err == nil {
		t.Error("opening the store for a hook created a .fellowship directory")
	}
}

// A store that exists but cannot be read means enforcement state is unknown.
// Gate hooks block; the fail-open worktree-guard still allows.
func TestStoreOpen_CorruptStoreFailsClosedForGateHooks(t *testing.T) {
	root := newMainRepo(t)
	if err := os.MkdirAll(filepath.Join(root, ".fellowship"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fellowship", "fellowship.db"),
		[]byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := openStore(root, []string{"hook", "gate-guard"})
	if err == nil {
		d.Close()
		t.Fatal("expected an error opening a corrupt store")
	}
	for _, name := range []string{"gate-guard", "gate-submit", "gate-prereq", "completion-guard", "metadata-track", "file-track"} {
		if got := storeOpenExit(root, []string{"hook", name}, err); got != 2 {
			t.Errorf("%s with a corrupt store: exit %d, want 2 (block)", name, got)
		}
	}
	if got := storeOpenExit(root, []string{"hook", "worktree-guard"}, err); got != 0 {
		t.Errorf("worktree-guard with a corrupt store: exit %d, want 0 (allow)", got)
	}
}

// Only explicit initialization may create the store.
func TestStoreCreatingCommand(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"init"}, true},
		{[]string{"init", "--phase", "Implement"}, true},
		{[]string{"state", "init", "--name", "f"}, true},
		{[]string{"state", "add-quest"}, false},
		{[]string{"hook", "gate-guard"}, false},
		{[]string{"status"}, false},
		{[]string{"gate", "approve"}, false},
	}
	for _, c := range cases {
		if got := storeCreatingCommand(c.args); got != c.want {
			t.Errorf("storeCreatingCommand(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestIsKnownCommand(t *testing.T) {
	if isKnownCommand("bogus") {
		t.Error("an unknown command must be rejected before the store is touched")
	}
	for _, name := range []string{"hook", "init", "state", "gate", "status", "version", "migrate"} {
		if !isKnownCommand(name) {
			t.Errorf("%q should be a known command", name)
		}
	}
}

// --- runHookWith ------------------------------------------------------------

func TestRunHookWith(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-alpha")
	stray := addWorktree(t, root, "quest-stray")

	cases := []struct {
		name  string
		hook  string
		cwd   string
		stdin io.Reader
		// setup registers quests and (optionally) quest state.
		setup func(t *testing.T) *db.DB
		want  int
	}{
		{
			name: "quest row absent allows so the teammate can run init",
			hook: "gate-guard",
			cwd:  worktree,
			// registered worktree, but no quest_state row yet
			setup: func(t *testing.T) *db.DB {
				return fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
			},
			stdin: bashInput("ls"),
			want:  0,
		},
		{
			name: "held quest blocks",
			hook: "gate-guard",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				reason := "paused"
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement", Held: true, HeldReason: &reason})
				return d
			},
			stdin: bashInput("ls"),
			want:  2,
		},
		{
			name: "pending gate allows a read-only fellowship command",
			hook: "gate-guard",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement", GatePending: true})
				return d
			},
			stdin: bashInput("fellowship status --json"),
			want:  0,
		},
		{
			name: "pending gate blocks self-approval",
			hook: "gate-guard",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement", GatePending: true})
				return d
			},
			stdin: bashInput("fellowship gate approve"),
			want:  2,
		},
		{
			name: "pending gate blocks ordinary work",
			hook: "gate-guard",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement", GatePending: true})
				return d
			},
			stdin: bashInput("go test ./..."),
			want:  2,
		},
		{
			name: "lead session in the main root is allowed",
			hook: "gate-guard",
			cwd:  root,
			setup: func(t *testing.T) *db.DB {
				return fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
			},
			stdin: bashInput("ls"),
			want:  0,
		},
		{
			name: "unregistered worktree during a fellowship blocks",
			hook: "gate-guard",
			cwd:  stray,
			setup: func(t *testing.T) *db.DB {
				return fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
			},
			stdin: bashInput("ls"),
			want:  2,
		},
		{
			name: "malformed JSON is tolerated by gate-guard",
			hook: "gate-guard",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement"})
				return d
			},
			stdin: strings.NewReader("{not json"),
			want:  0,
		},
		{
			name: "malformed JSON blocks the recording hooks",
			hook: "completion-guard",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement"})
				return d
			},
			stdin: strings.NewReader("{not json"),
			want:  2,
		},
		{
			name: "unknown hook name blocks",
			hook: "nonexistent-hook",
			cwd:  worktree,
			setup: func(t *testing.T) *db.DB {
				d := fellowshipWith(t, root, map[string]string{"quest-alpha": worktree})
				setQuestState(t, d, &state.State{QuestName: "quest-alpha", Phase: "Implement"})
				return d
			},
			stdin: bashInput("ls"),
			want:  2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := c.setup(t)
			if got := runHookWith(c.hook, c.stdin, c.cwd, d); got != c.want {
				t.Errorf("runHookWith(%s) = %d, want %d", c.hook, got, c.want)
			}
		})
	}
}

// A quest registered with a relative path must still be found from the
// worktree's absolute git top-level, or the gate would silently not apply.
func TestRunHookWith_RelativeRegisteredWorktree(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-rel")
	t.Chdir(filepath.Dir(worktree))

	d := fellowshipWith(t, root, map[string]string{"quest-rel": "quest-rel"})
	setQuestState(t, d, &state.State{QuestName: "quest-rel", Phase: "Implement", GatePending: true})

	if got := runHookWith("gate-guard", bashInput("go build ./..."), worktree, d); got != 2 {
		t.Errorf("gate should apply to a relatively-registered worktree: exit %d, want 2", got)
	}
}
