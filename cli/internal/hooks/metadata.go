package hooks

import "github.com/justinjdev/fellowship/cli/internal/state"

func MetadataTrack(s *state.State, input *HookInput) bool {
	if input.ToolInput.Metadata != nil && input.ToolInput.Metadata.Phase != "" {
		s.MetadataUpdated = true
		return true
	}
	return false
}
