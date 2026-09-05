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

// cfg holds the subset of fellowship config the CLI cares about.
type cfg struct {
	DataDir string `json:"dataDir"`
	Autopsy struct {
		ExpiryDays int `json:"expiryDays"`
	} `json:"autopsy"`
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

// Name returns the configured data directory name.
// Merge order: defaults → project (.fellowship/config.json) → user (~/.claude/fellowship.json).
// User config always wins. Result is cached after the first call.
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

// IsDataDirPath reports whether the given path is inside the fellowship data directory.
func IsDataDirPath(path string) bool {
	name := Name()
	// Normalize to forward slashes for consistent matching across platforms.
	p := filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(p, "/"+name+"/") || strings.HasPrefix(p, name+"/")
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
	return gitutil.TopLevel("")
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

// AutopsyExpiryDays resolves autopsy.expiryDays with the same precedence as the
// other settings: defaults, then <root>/.fellowship/config.json, then
// ~/.claude/fellowship.json. Returns defaultDays when nothing sets it.
func AutopsyExpiryDays(root string, defaultDays int) int {
	days := defaultDays
	if root != "" {
		if p := readConfigFile(filepath.Join(root, DefaultName, "config.json")); p.Autopsy.ExpiryDays > 0 {
			days = p.Autopsy.ExpiryDays
		}
	}
	if u := readUserConfig(); u.Autopsy.ExpiryDays > 0 {
		days = u.Autopsy.ExpiryDays
	}
	return days
}
