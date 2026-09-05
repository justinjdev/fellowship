package status

import "testing"

func TestBaseBranchOrDefault(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty falls back to main", input: "", want: "main"},
		{name: "whitespace falls back to main", input: "  ", want: "main"},
		{name: "stored branch is used", input: "develop", want: "develop"},
		{name: "slashes are preserved", input: "release/2.x", want: "release/2.x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BaseBranchOrDefault(tt.input); got != tt.want {
				t.Errorf("BaseBranchOrDefault(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
