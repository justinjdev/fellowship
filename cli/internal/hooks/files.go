package hooks

import (
	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"zombiezen.com/go/sqlite"
)

// FileTrack records file paths from Edit/Write tool inputs into the quest
// history. Returns true if the history was modified. dataDirName is the data
// directory of the MAIN repo, whose coordination writes are not quest files;
// empty falls back to the process-wide lookup.
func FileTrack(conn *sqlite.Conn, s *state.State, input *HookInput, questName, dataDirName string) bool {
	filePath := TargetPath(input)
	if dataDirName == "" {
		dataDirName = datadir.Name()
	}
	if filePath == "" || datadir.IsPathIn(filePath, dataDirName) {
		return false
	}

	// RecordFiles uses INSERT OR IGNORE, so duplicates are silently skipped.
	if err := history.RecordFiles(conn, questName, []string{filePath}); err != nil {
		return false
	}
	return conn.Changes() > 0
}
