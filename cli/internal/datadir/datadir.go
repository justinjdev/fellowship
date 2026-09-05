package datadir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/justinjdev/fellowship/cli/internal/gitutil"
)

// DefaultName is the default directory name for fellowship working files.
const DefaultName = ".fellowship"

// StoreFileName is the SQLite store's file name inside the data directory.
const StoreFileName = "fellowship.db"

// IsStorePath reports whether a path names the SQLite store or one of the
// sidecar files SQLite keeps beside it (-wal, -shm, -journal).
//
// The data directory as a whole is exempt from the write guards — teammates
// legitimately keep coordination files there — but the store is not a
// coordination file: it IS the enforcement state, and a session that can write
// it can rewrite its own phase, clear its own gate, or name itself the lead.
// Nothing legitimate edits it through Edit/Write; the CLI is the only writer.
func IsStorePath(path string) bool {
	base := filepath.Base(filepath.Clean(filepath.FromSlash(path)))
	return base == StoreFileName || strings.HasPrefix(base, StoreFileName+"-")
}

// cfg holds the subset of fellowship config the CLI cares about.
type cfg struct {
	DataDir  string `json:"dataDir"`
	Failures struct {
		ExpiryDays int `json:"expiryDays"`
	} `json:"failures"`
	Gates struct {
		// Pointer so an explicit empty list in the user config can override a
		// non-empty project list.
		AutoApprove *[]string `json:"autoApprove"`
	} `json:"gates"`
}

var (
	nameOnce   sync.Once
	cachedName string
)

// Name returns the configured data directory name for the repo the process is
// standing in. Merge order: defaults → project (.fellowship/config.json) → user
// (~/.claude/fellowship.json). User config always wins. Result is cached after
// the first call.
//
// The repo is resolved through gitutil.MainRepoRoot, the same lookup
// db.StorePath makes: the project config lives in the MAIN worktree, so
// resolving the session's own top-level meant a session inside a linked
// worktree read no project config at all and silently fell back to
// ".fellowship" while the store sat in the configured directory.
//
// Code that already knows which repo it means — every hook, which resolves the
// main root anyway — should call Resolve(mainRoot) rather than this.
func Name() string {
	nameOnce.Do(func() {
		root, err := gitRoot()
		if err != nil {
			root = ""
		}
		cachedName = Resolve(root)
	})
	return cachedName
}

// Resolve returns the data directory name for the repo rooted at root, reading
// the configs each time instead of caching. Callers that already know which
// repo they mean — the store path resolver above all, which must land in the
// same directory the guards look in — use this rather than Name(), whose git
// lookup is relative to the process working directory.
//
// A name containing a path separator or ".." is rejected: the data directory is
// a single directory name inside the repo, not a path.
func Resolve(root string) string {
	dataDir := ""
	if root != "" {
		if p := readConfigFile(filepath.Join(root, DefaultName, "config.json")); p.DataDir != "" {
			dataDir = p.DataDir
		}
	}
	if u := readUserConfig(); u.DataDir != "" {
		dataDir = u.DataDir
	}

	if dataDir == "" || strings.ContainsAny(dataDir, "/\\") || strings.Contains(dataDir, "..") {
		return DefaultName
	}
	return dataDir
}

// IsDataDirPath reports whether the given path is inside the fellowship data
// directory of the repo the process is standing in. Callers that already know
// the repo — the hooks — should use IsPathIn with the name resolved from the
// main repo root.
func IsDataDirPath(path string) bool {
	return IsPathIn(path, Name())
}

// IsPathIn reports whether path lies inside a data directory named dataDirName.
//
// The match is on a path segment anywhere in the path, not on the repo-relative
// location: a hook is handed whatever path the tool call carried, which may be
// relative to a worktree the hook cannot resolve. An empty name matches
// nothing.
func IsPathIn(path, dataDirName string) bool {
	if dataDirName == "" || path == "" {
		return false
	}
	// Normalize to forward slashes for consistent matching across platforms.
	p := filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(p, "/"+dataDirName+"/") || strings.HasPrefix(p, dataDirName+"/")
}

// readUserConfig reads ~/.claude/fellowship.json.
func readUserConfig() cfg {
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg{}
	}
	return readConfigFile(filepath.Join(home, ".claude", "fellowship.json"))
}

// readConfigFile parses a fellowship JSON config file, returning empty cfg on any error.
func readConfigFile(path string) cfg {
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg{}
	}
	var c cfg
	if json.Unmarshal(data, &c) != nil {
		return cfg{}
	}
	return c
}

// gitRootFunc is the function used to find the git repository root.
// It is a variable so tests can override it without spawning a subprocess.
var gitRootFunc = func() (string, error) {
	// The working directory has to be named explicitly: `git rev-parse
	// --git-common-dir` answers relatively in the main worktree (".git"), and
	// MainRepoRoot can only absolutize that against the directory it was given.
	// Passing "" made it return "." — a root that is only correct for as long
	// as the process never changes directory.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return gitutil.MainRepoRoot(cwd)
}

func gitRoot() (string, error) {
	return gitRootFunc()
}

// AutoApproveGates returns the merged gates.autoApprove list.
// Merge order: project (<root>/.fellowship/config.json) → user
// (~/.claude/fellowship.json). The user config wins when it sets the key at
// all, including to an empty list. Returns nil when neither config sets it.
func AutoApproveGates(root string) []string {
	var gates []string
	if root != "" {
		if p := readConfigFile(filepath.Join(root, DefaultName, "config.json")); p.Gates.AutoApprove != nil {
			gates = *p.Gates.AutoApprove
		}
	}
	if u := readUserConfig(); u.Gates.AutoApprove != nil {
		gates = *u.Gates.AutoApprove
	}
	return gates
}

// FailuresExpiryDays resolves failures.expiryDays with the same precedence as the
// other settings: defaults, then <root>/.fellowship/config.json, then
// ~/.claude/fellowship.json. Returns defaultDays when nothing sets it.
func FailuresExpiryDays(root string, defaultDays int) int {
	days := defaultDays
	if root != "" {
		if p := readConfigFile(filepath.Join(root, DefaultName, "config.json")); p.Failures.ExpiryDays > 0 {
			days = p.Failures.ExpiryDays
		}
	}
	if u := readUserConfig(); u.Failures.ExpiryDays > 0 {
		days = u.Failures.ExpiryDays
	}
	return days
}
