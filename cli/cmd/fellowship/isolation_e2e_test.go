package main

import (
	"context"
	"path/filepath"
	"testing"
)

// End to end: the teammate never leaves its own worktree — it just names an
// absolute path in the main tree. The guard used to stop looking as soon as the
// session's top-level was not the main root.
func TestRunWorktreeGuard_AbsolutePathIntoTheMainTree(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-abs-alpha")
	d := fellowshipWith(t, root, map[string]string{"quest-abs-alpha": worktree})
	recordLead(t, d, root, "lead-session")

	mainTarget := filepath.Join(root, "src", "main.go")
	ownTarget := filepath.Join(worktree, "src", "main.go")

	cases := []struct {
		name    string
		cwd     string
		session string
		target  string
		want    int
	}{
		{
			name:    "teammate in its worktree writing into the main tree",
			cwd:     worktree,
			session: "teammate-session",
			target:  mainTarget,
			want:    2,
		},
		{
			name:    "teammate writing inside its own worktree",
			cwd:     worktree,
			session: "teammate-session",
			target:  ownTarget,
			want:    0,
		},
		{
			name:    "the lead writing in the main tree",
			cwd:     root,
			session: "lead-session",
			target:  mainTarget,
			want:    0,
		},
		{
			name:    "the lead reaching into a quest worktree is isolation's business, not this guard's",
			cwd:     root,
			session: "lead-session",
			target:  ownTarget,
			want:    0,
		},
		{
			name:    "teammate hand-editing the main tree's data directory",
			cwd:     worktree,
			session: "teammate-session",
			target:  filepath.Join(root, ".fellowship", "notes.md"),
			want:    2,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := runWorktreeGuard(context.Background(), d, c.cwd, writeInput(c.session, c.target))
			if got != c.want {
				t.Errorf("runWorktreeGuard = %d, want %d", got, c.want)
			}
		})
	}
}
