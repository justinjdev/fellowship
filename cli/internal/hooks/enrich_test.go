package hooks

import (
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

func TestGatherEnrichment_EmptyDB(t *testing.T) {
	// With an empty DB and non-existent directory, should return a minimal enrichment block
	d := db.OpenTest(t)
	var result string
	d.WithConn(t.Context(), func(conn *db.Conn) error {
		result = GatherEnrichment(conn, "nonexistent-quest", "/nonexistent/path")
		return nil
	})
	// Even with no quest data, some fields produce default values (e.g., "none" for files)
	if result != "" && !strings.Contains(result, "Gate Context") {
		t.Errorf("expected empty or valid enrichment block, got: %q", result)
	}
}

func TestFormatErrandProgress_WithErrands(t *testing.T) {
	got := formatErrandProgress(3, 5)
	if got != "3/5 done" {
		t.Errorf("got %q, want %q", got, "3/5 done")
	}
}

func TestFormatErrandProgress_AllDone(t *testing.T) {
	got := formatErrandProgress(5, 5)
	if got != "5/5 done" {
		t.Errorf("got %q, want %q", got, "5/5 done")
	}
}

func TestFormatErrandProgress_None(t *testing.T) {
	got := formatErrandProgress(0, 0)
	if got != "no errands" {
		t.Errorf("got %q, want %q", got, "no errands")
	}
}

func TestFormatFilesTouched_Multiple(t *testing.T) {
	files := []string{"src/main.go", "src/auth.go", "src/db.go"}
	got := formatFilesTouched(files)
	if !strings.HasPrefix(got, "3") {
		t.Errorf("should start with count, got: %q", got)
	}
	if !strings.Contains(got, "src/main.go") {
		t.Errorf("should contain file names, got: %q", got)
	}
}

func TestFormatFilesTouched_Empty(t *testing.T) {
	got := formatFilesTouched(nil)
	if got != "none" {
		t.Errorf("got %q, want %q", got, "none")
	}
}

func TestFormatFilesTouched_TruncatesLongList(t *testing.T) {
	files := make([]string, 20)
	for i := range files {
		files[i] = "file" + string(rune('a'+i)) + ".go"
	}
	got := formatFilesTouched(files)
	if !strings.Contains(got, "...") {
		t.Errorf("should truncate long list, got: %q", got)
	}
}

func TestParseDiffStats(t *testing.T) {
	// Typical git diff --stat output
	output := ` src/main.go   | 10 ++++------
 src/auth.go   |  5 +++++
 2 files changed, 9 insertions(+), 6 deletions(-)
`
	got := parseDiffStats(output)
	if !strings.Contains(got, "+9") {
		t.Errorf("should contain insertions, got: %q", got)
	}
	if !strings.Contains(got, "-6") {
		t.Errorf("should contain deletions, got: %q", got)
	}
	if !strings.Contains(got, "2 files") {
		t.Errorf("should contain file count, got: %q", got)
	}
}

func TestParseDiffStats_Empty(t *testing.T) {
	got := parseDiffStats("")
	if got != "no changes" {
		t.Errorf("got %q, want %q", got, "no changes")
	}
}

func TestBuildEnrichmentBlock(t *testing.T) {
	block := buildEnrichmentBlock("3/5 done", "2 (src/main.go, src/auth.go)", "+10 -5 across 2 files", "8m")
	if !strings.Contains(block, "Gate Context") {
		t.Error("should contain header")
	}
	if !strings.Contains(block, "3/5 done") {
		t.Error("should contain errand progress")
	}
	if !strings.Contains(block, "+10 -5") {
		t.Error("should contain diff stats")
	}
}

func TestBuildEnrichmentBlock_SkipsMissingFields(t *testing.T) {
	block := buildEnrichmentBlock("", "none", "", "")
	if strings.Contains(block, "Errands") {
		t.Error("should skip empty errands line")
	}
	if strings.Contains(block, "Diff") {
		t.Error("should skip empty diff line")
	}
	if strings.Contains(block, "Phase duration") {
		t.Error("should skip empty duration line")
	}
}
