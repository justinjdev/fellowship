package state

import "path/filepath"

// CanonicalWorktree normalizes a worktree path so that the value written by
// `fellowship state add-quest --worktree ...` and the value a hook derives from
// `git rev-parse --show-toplevel` compare equal. It makes the path absolute
// (relative to the process working directory, which for the CLI is the caller's
// cwd) and resolves symlinks, so "../wt-1" and a /tmp path that is really
// /private/tmp both land on the same string.
//
// Paths that do not exist yet are handled by resolving the longest existing
// ancestor and re-appending the missing remainder, so registering a worktree
// before it is created still produces the value the hook will later look up.
// An empty path stays empty.
//
// This deliberately duplicates the shape of hooks.CanonicalPath rather than
// calling it: the hooks package imports state, and hooks.CanonicalPath must
// keep its "leave relative paths relative" behavior for cd-target matching.
func CanonicalWorktree(p string) string {
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	rem := ""
	cur := p
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			if rem == "" {
				return resolved
			}
			return filepath.Join(resolved, rem)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p // reached the root without resolving; use as-is
		}
		rem = filepath.Join(filepath.Base(cur), rem)
		cur = parent
	}
}
