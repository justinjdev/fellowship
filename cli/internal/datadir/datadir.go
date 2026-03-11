package datadir

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultName is the default directory name for fellowship working files.
const DefaultName = ".fellowship"

// cfg holds the subset of fellowship config the CLI cares about.
type cfg struct {
	DataDir string `json:"dataDir"`
	Autopsy struct {
		ExpiryDays int `json:"expiryDays"`
	} `json:"autopsy"`
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
		dataDir := ""

		if p := readProjectConfig(); p.DataDir != "" {
			dataDir = p.DataDir
		}
		if u := readUserConfig(); u.DataDir != "" {
			dataDir = u.DataDir
		}

		if dataDir == "" || strings.ContainsAny(dataDir, "/\\") || strings.Contains(dataDir, "..") {
			cachedName = DefaultName
			return
		}
		cachedName = dataDir
	})
	return cachedName
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

// readProjectConfig reads .fellowship/config.json from the git root.
// Returns empty config if git root cannot be determined or the file does not exist.
func readProjectConfig() cfg {
	root, err := gitRoot()
	if err != nil {
		return cfg{}
	}
	return readConfigFile(filepath.Join(root, DefaultName, "config.json"))
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
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRoot() (string, error) {
	return gitRootFunc()
}

// AutopsyExpiryDays reads autopsy.expiryDays from ~/.claude/fellowship.json.
// Returns the provided defaultDays if not configured or on any error.
func AutopsyExpiryDays(defaultDays int) int {
	c := readUserConfig()
	if c.Autopsy.ExpiryDays <= 0 {
		return defaultDays
	}
	return c.Autopsy.ExpiryDays
}
