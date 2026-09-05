package herald

import "testing"

func TestValidType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  TidingType
		ok    bool
	}{
		{name: "gate type", input: "gate_approved", want: GateApproved, ok: true},
		{name: "palantir stuck", input: "palantir_stuck", want: PalantirStuck, ok: true},
		{name: "palantir bulletin", input: "palantir_bulletin", want: PalantirBulletin, ok: true},
		{name: "unknown type", input: "smoke_signal"},
		{name: "empty type", input: ""},
		{name: "case sensitive", input: "Gate_Approved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ValidType(tt.input)
			if ok != tt.ok || got != tt.want {
				t.Errorf("ValidType(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTypes(t *testing.T) {
	names := Types()
	if len(names) != len(allTypes) {
		t.Fatalf("Types() returned %d names, want %d", len(names), len(allTypes))
	}
	for _, n := range names {
		if _, ok := ValidType(n); !ok {
			t.Errorf("Types() returned %q, which ValidType rejects", n)
		}
	}
}
