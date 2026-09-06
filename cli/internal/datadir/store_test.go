package datadir

import "testing"

func TestIsStorePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/repo/.fellowship/fellowship.db", true},
		{"/repo/.fellowship/fellowship.db-wal", true},
		{"/repo/.fellowship/fellowship.db-shm", true},
		{"/repo/.fellowship/fellowship.db-journal", true},
		{".fellowship/fellowship.db", true},
		{"/repo/queststate/fellowship.db", true},
		{"fellowship.db", true},
		{"/repo/.fellowship/notes.md", false},
		{"/repo/.fellowship/fellowship.dbx", false},
		{"/repo/src/fellowship.db.go", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsStorePath(c.path); got != c.want {
			t.Errorf("IsStorePath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
