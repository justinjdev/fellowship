package datadir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DefaultName is the default directory name for fellowship working files.
const DefaultName = ".fellowship"

// Name returns the configured data directory name, reading from
// ~/.claude/fellowship.json if it exists. Falls back to DefaultName.
func Name() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultName
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "fellowship.json"))
	if err != nil {
		return DefaultName
	}
	var cfg struct {
		DataDir string `json:"dataDir"`
	}
	if json.Unmarshal(data, &cfg) != nil || cfg.DataDir == "" {
		return DefaultName
	}
	return cfg.DataDir
}

// IsDataDirPath reports whether the given path is inside the fellowship data directory.
func IsDataDirPath(path string) bool {
	name := Name()
	return strings.Contains(path, "/"+name+"/") || strings.HasPrefix(path, name+"/")
}
