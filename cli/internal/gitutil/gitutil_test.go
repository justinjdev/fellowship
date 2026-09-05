package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

// newRepo creates a repo with one commit and returns its resolved root.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-qm", "init")
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// The two resolvers differ exactly where it matters: inside a linked worktree,
// TopLevel is the worktree and MainRepoRoot is the repo the store belongs to.
func TestTopLevelAndMainRepoRoot(t *testing.T) {
	root := newRepo(t)
	sub := filepath.Join(root, "pkg", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(root), "quest-alpha")
	git(t, root, "worktree", "add", "-q", "-b", "quest-alpha", worktree)
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name         string
		dir          string
		wantTop      string
		wantMainRoot string
	}{
		{name: "main repo root", dir: root, wantTop: root, wantMainRoot: root},
		{name: "subdirectory", dir: sub, wantTop: root, wantMainRoot: root},
		{name: "linked worktree", dir: resolvedWorktree, wantTop: resolvedWorktree, wantMainRoot: root},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			top, err := TopLevel(c.dir)
			if err != nil {
				t.Fatalf("TopLevel(%q): %v", c.dir, err)
			}
			if top != c.wantTop {
				t.Errorf("TopLevel(%q) = %q, want %q", c.dir, top, c.wantTop)
			}
			mainRoot, err := MainRepoRoot(c.dir)
			if err != nil {
				t.Fatalf("MainRepoRoot(%q): %v", c.dir, err)
			}
			if mainRoot != c.wantMainRoot {
				t.Errorf("MainRepoRoot(%q) = %q, want %q", c.dir, mainRoot, c.wantMainRoot)
			}
		})
	}
}

// Outside a repository both resolvers report an error rather than guessing.
func TestResolversOutsideARepo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if _, err := TopLevel(dir); err == nil {
		t.Error("TopLevel outside a repo should fail")
	}
	if _, err := MainRepoRoot(dir); err == nil {
		t.Error("MainRepoRoot outside a repo should fail")
	}
}
