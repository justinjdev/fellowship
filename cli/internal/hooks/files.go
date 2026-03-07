package hooks

import (
	"github.com/justinjdev/fellowship/cli/internal/tome"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// FileTrack records file paths from Edit/Write tool inputs into the quest tome.
// Returns true if the tome was modified.
func FileTrack(s *state.State, input *HookInput, tomePath string) bool {
	filePath := input.ToolInput.FilePath
	if filePath == "" {
		filePath = input.ToolInput.NotebookPath
	}
	if filePath == "" || isTmpPath(filePath) {
		return false
	}

	c := tome.LoadOrCreate(tomePath)
	before := len(c.FilesTouched)
	tome.RecordFiles(c, []string{filePath})
	if len(c.FilesTouched) == before {
		return false
	}

	if err := tome.Save(tomePath, c); err != nil {
		return false
	}
	return true
}
