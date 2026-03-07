package status

import "testing"

func TestClassifyQuest(t *testing.T) {
	tests := []struct {
		name string
		q    QuestInfo
		want string
	}{
		{
			name: "merged quest is complete",
			q:    QuestInfo{Merged: true},
			want: "complete",
		},
		{
			name: "merged takes priority over checkpoint",
			q:    QuestInfo{Merged: true, HasCheckpoint: true},
			want: "complete",
		},
		{
			name: "checkpoint without merge is resumable",
			q:    QuestInfo{HasCheckpoint: true},
			want: "resumable",
		},
		{
			name: "no merge no checkpoint is stale",
			q:    QuestInfo{},
			want: "stale",
		},
		{
			name: "uncommitted changes alone is stale",
			q:    QuestInfo{HasUncommitted: true},
			want: "stale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyQuest(tt.q)
			if got != tt.want {
				t.Errorf("ClassifyQuest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMergedBranches(t *testing.T) {
	tests := []struct {
		name string
		input string
		want  []string
	}{
		{
			name: "filters to fellowship prefix only",
			input: "  main\n  fellowship/quest-1\n  feature/other\n  fellowship/quest-2\n",
			want:  []string{"fellowship/quest-1", "fellowship/quest-2"},
		},
		{
			name: "handles star prefix for current branch",
			input: "* fellowship/quest-active\n  fellowship/quest-done\n  main\n",
			want:  []string{"fellowship/quest-active", "fellowship/quest-done"},
		},
		{
			name: "empty input returns empty slice",
			input: "",
			want:  []string{},
		},
		{
			name: "no fellowship branches returns empty slice",
			input: "  main\n  develop\n  feature/foo\n",
			want:  []string{},
		},
		{
			name: "handles extra whitespace",
			input: "    fellowship/quest-1   \n",
			want:  []string{"fellowship/quest-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMergedBranches(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseMergedBranches() returned %d items, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseMergedBranches()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
