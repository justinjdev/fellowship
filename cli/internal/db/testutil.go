package db

import "testing"

// OpenTest returns an in-memory DB with the full schema applied.
// The DB is automatically closed when the test ends.
func OpenTest(t *testing.T) *DB {
	t.Helper()
	d, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
