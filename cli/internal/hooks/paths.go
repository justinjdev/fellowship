package hooks

import "path/filepath"

// CanonicalPath resolves symlinks in p, falling back gracefully for paths that
// don't exist yet (e.g. a Write target or a not-yet-created cd target): it
// resolves the longest existing ancestor and re-appends the missing remainder.
// Returns a cleaned absolute path; an empty input returns empty. Both the
// isolation guard and the cd-guard canonicalize their inputs through this so
// symlinked repo roots (e.g. macOS /tmp -> /private/tmp) compare equal.
func CanonicalPath(p string) string {
	if p == "" {
		return ""
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
