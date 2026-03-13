package hooks

import (
	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
	"zombiezen.com/go/sqlite"
)

// FileTrack records file paths from Edit/Write tool inputs into the quest tome.
// Returns true if the tome was modified.
func FileTrack(conn *sqlite.Conn, s *state.State, input *HookInput, questName string) bool {
	filePath := input.ToolInput.FilePath
	if filePath == "" {
		filePath = input.ToolInput.NotebookPath
	}
	if filePath == "" || datadir.IsDataDirPath(filePath) {
		return false
	}

	// RecordFiles uses INSERT OR IGNORE, so duplicates are silently skipped.
	if err := tome.RecordFiles(conn, questName, []string{filePath}); err != nil {
		return false
	}
	return conn.Changes() > 0
}
