package hooks

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

// GatherEnrichment collects quest metrics from the worktree directory
// and returns a formatted enrichment block to append to gate messages.
// Returns empty string if no data sources are available.
func GatherEnrichment(dir string) string {
	errandStr := gatherErrandProgress(dir)
	filesStr := gatherFilesTouched(dir)
	diffStr := gatherDiffStats(dir)
	durationStr := gatherPhaseDuration(dir)

	block := buildEnrichmentBlock(errandStr, filesStr, diffStr, durationStr)
	return block
}

func gatherErrandProgress(dir string) string {
	path, err := errand.FindErrands(dir)
	if err != nil || path == "" {
		return ""
	}
	el, err := errand.Load(path)
	if err != nil {
		return ""
	}
	done, total := errand.Progress(el)
	return formatErrandProgress(done, total)
}

func gatherFilesTouched(dir string) string {
	path, err := tome.FindTome(dir)
	if err != nil || path == "" {
		return ""
	}
	t, err := tome.Load(path)
	if err != nil {
		return ""
	}
	return formatFilesTouched(t.FilesTouched)
}

func gatherDiffStats(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		return ""
	}
	return parseDiffStats(string(out))
}

func gatherPhaseDuration(dir string) string {
	tidings, err := herald.Read(dir, 0)
	if err != nil || len(tidings) == 0 {
		return ""
	}
	// Find the most recent gate_approved tiding (marks phase entry).
	for i := len(tidings) - 1; i >= 0; i-- {
		if tidings[i].Type == herald.GateApproved {
			ts, err := time.Parse(time.RFC3339, tidings[i].Timestamp)
			if err != nil {
				continue
			}
			dur := time.Since(ts)
			if dur < time.Minute {
				return fmt.Sprintf("%ds", int(dur.Seconds()))
			}
			return fmt.Sprintf("%dm", int(dur.Minutes()))
		}
	}
	return ""
}

func formatErrandProgress(done, total int) string {
	if total == 0 {
		return "no errands"
	}
	return fmt.Sprintf("%d/%d done", done, total)
}

func formatFilesTouched(files []string) string {
	if len(files) == 0 {
		return "none"
	}
	const maxShow = 5
	if len(files) <= maxShow {
		return fmt.Sprintf("%d (%s)", len(files), strings.Join(files, ", "))
	}
	return fmt.Sprintf("%d (%s, ...)", len(files), strings.Join(files[:maxShow], ", "))
}

var diffSummaryRe = regexp.MustCompile(`(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

func parseDiffStats(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return "no changes"
	}
	m := diffSummaryRe.FindStringSubmatch(output)
	if m == nil {
		return "no changes"
	}
	files := m[1]
	ins := m[2]
	del := m[3]
	if ins == "" {
		ins = "0"
	}
	if del == "" {
		del = "0"
	}
	return fmt.Sprintf("+%s -%s across %s files", ins, del, files)
}

func buildEnrichmentBlock(errands, files, diff, duration string) string {
	var lines []string
	if errands != "" {
		lines = append(lines, fmt.Sprintf("- **Errands:** %s", errands))
	}
	if files != "" {
		lines = append(lines, fmt.Sprintf("- **Files touched:** %s", files))
	}
	if diff != "" {
		lines = append(lines, fmt.Sprintf("- **Diff:** %s", diff))
	}
	if duration != "" {
		lines = append(lines, fmt.Sprintf("- **Phase duration:** %s", duration))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n---\n## Gate Context (auto-generated)\n" + strings.Join(lines, "\n")
}
