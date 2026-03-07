package hooks

import (
	"github.com/justinjdev/fellowship/cli/internal/cv"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// FileTrack records file paths from Edit/Write tool inputs into the quest CV.
// Returns true if the CV was modified.
func FileTrack(s *state.State, input *HookInput, cvPath string) bool {
	filePath := input.ToolInput.FilePath
	if filePath == "" {
		filePath = input.ToolInput.NotebookPath
	}
	if filePath == "" || isTmpPath(filePath) {
		return false
	}

	c := cv.LoadOrCreate(cvPath)
	before := len(c.FilesTouched)
	cv.RecordFiles(c, []string{filePath})
	if len(c.FilesTouched) == before {
		return false
	}

	if err := cv.Save(cvPath, c); err != nil {
		return false
	}
	return true
}
