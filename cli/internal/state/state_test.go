package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestUpsertAndLoad(t *testing.T) {
	d := db.OpenTest(t)
	s := &state.State{
		QuestName: "quest-auth",
		Phase:     "Research",
	}

	d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := state.Upsert(conn, s); err != nil {
			t.Fatal(err)
		}

		loaded, err := state.Load(conn, "quest-auth")
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Phase != "Research" {
			t.Errorf("expected Research, got %s", loaded.Phase)
		}
		return nil
	})
}

func TestLoad_NotFound(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		_, err := state.Load(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent quest")
		}
		return nil
	})
}

func TestUpsert_Update(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		s := &state.State{QuestName: "q1", Phase: "Onboard"}
		state.Upsert(conn, s)

		s.Phase = "Research"
		s.GatePending = true
		state.Upsert(conn, s)

		loaded, _ := state.Load(conn, "q1")
		if loaded.Phase != "Research" {
			t.Errorf("expected Research, got %s", loaded.Phase)
		}
		if !loaded.GatePending {
			t.Error("expected GatePending true")
		}
		return nil
	})
}

func TestFindQuest(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		sqlitex.Execute(conn, `INSERT INTO fellowship_quests (name, worktree) VALUES ('quest-auth', '/tmp/wt/quest-auth')`, nil)

		name, err := state.FindQuest(conn, "/tmp/wt/quest-auth")
		if err != nil {
			t.Fatal(err)
		}
		if name != "quest-auth" {
			t.Errorf("expected quest-auth, got %s", name)
		}
		return nil
	})
}

func TestBoolIntConversion(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		s := &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: true,
			Held:        true,
		}
		state.Upsert(conn, s)

		loaded, _ := state.Load(conn, "q1")
		if !loaded.GatePending {
			t.Error("GatePending should be true")
		}
		if !loaded.Held {
			t.Error("Held should be true")
		}
		return nil
	})
}

func TestNextPhase(t *testing.T) {
	tests := []struct {
		current string
		want    string
		wantErr bool
	}{
		{"Onboard", "Research", false},
		{"Research", "Plan", false},
		{"Plan", "Implement", false},
		{"Implement", "Adversarial", false},
		{"Adversarial", "Review", false},
		{"Review", "Complete", false},
		{"Complete", "", true},
		{"InvalidPhase", "", true},
	}
	for _, tt := range tests {
		got, err := state.NextPhase(tt.current)
		if (err != nil) != tt.wantErr {
			t.Errorf("NextPhase(%q) error = %v, wantErr %v", tt.current, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("NextPhase(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestIsValidPhase(t *testing.T) {
	for _, p := range []string{"Onboard", "Research", "Plan", "Implement", "Adversarial", "Review", "Complete"} {
		if !state.IsValidPhase(p) {
			t.Errorf("IsValidPhase(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"", "onboard", "InvalidPhase", "Done"} {
		if state.IsValidPhase(p) {
			t.Errorf("IsValidPhase(%q) = true, want false", p)
		}
	}
}

func TestIsEarlyPhase(t *testing.T) {
	early := []string{"Onboard", "Research", "Plan"}
	late := []string{"Implement", "Adversarial", "Review", "Complete"}
	for _, p := range early {
		if !state.IsEarlyPhase(p) {
			t.Errorf("IsEarlyPhase(%q) should be true", p)
		}
	}
	for _, p := range late {
		if state.IsEarlyPhase(p) {
			t.Errorf("IsEarlyPhase(%q) should be false", p)
		}
	}
}

// --- gate state machine -----------------------------------------------------

// ptr returns a pointer to v, for building expected gate ids.
func ptr[T any](v T) *T { return &v }

func TestApprove(t *testing.T) {
	tests := []struct {
		name       string
		in         *state.State
		wantPrev   string
		wantNext   string
		wantErr    error
		wantAnyErr bool
		want       *state.State
	}{
		{
			name:     "advances phase and clears gate and prereqs",
			in:       &state.State{Phase: "Implement", GatePending: true, GateID: ptr("gate-Implement-1"), LembasCompleted: true, MetadataUpdated: true},
			wantPrev: "Implement",
			wantNext: "Adversarial",
			want:     &state.State{Phase: "Adversarial"},
		},
		{
			name:     "last gate lands on Complete",
			in:       &state.State{Phase: "Review", GatePending: true, GateID: ptr("gate-Review-1")},
			wantPrev: "Review",
			wantNext: "Complete",
			want:     &state.State{Phase: "Complete"},
		},
		{
			name:    "no gate pending",
			in:      &state.State{Phase: "Implement"},
			wantErr: state.ErrNoGatePending,
			want:    &state.State{Phase: "Implement"},
		},
		{
			name:       "no phase after Complete leaves the state untouched",
			in:         &state.State{Phase: "Complete", GatePending: true, GateID: ptr("gate-Complete-1")},
			wantAnyErr: true,
			want:       &state.State{Phase: "Complete", GatePending: true, GateID: ptr("gate-Complete-1")},
		},
		{
			name:       "unknown phase leaves the state untouched",
			in:         &state.State{Phase: "Shipping", GatePending: true, GateID: ptr("g")},
			wantAnyErr: true,
			want:       &state.State{Phase: "Shipping", GatePending: true, GateID: ptr("g")},
		},
		{
			name:    "nil state",
			wantErr: state.ErrNilState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev, next, err := state.Approve(tt.in)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Approve() error = %v, want %v", err, tt.wantErr)
				}
			case tt.wantAnyErr:
				if err == nil {
					t.Fatal("Approve() = nil error, want a phase error")
				}
			default:
				if err != nil {
					t.Fatalf("Approve() error = %v, want nil", err)
				}
			}
			if prev != tt.wantPrev || next != tt.wantNext {
				t.Errorf("Approve() = (%q, %q), want (%q, %q)", prev, next, tt.wantPrev, tt.wantNext)
			}
			if tt.in != nil {
				assertState(t, tt.in, tt.want)
			}
		})
	}
}

func TestReject(t *testing.T) {
	tests := []struct {
		name    string
		in      *state.State
		wantErr error
		want    *state.State
	}{
		{
			name: "clears the gate and keeps the phase and prereqs",
			in:   &state.State{Phase: "Review", GatePending: true, GateID: ptr("gate-Review-1"), LembasCompleted: true, MetadataUpdated: true},
			want: &state.State{Phase: "Review", LembasCompleted: true, MetadataUpdated: true},
		},
		{
			name:    "no gate pending",
			in:      &state.State{Phase: "Review"},
			wantErr: state.ErrNoGatePending,
			want:    &state.State{Phase: "Review"},
		},
		{
			name:    "nil state",
			wantErr: state.ErrNilState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := state.Reject(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Reject() error = %v, want %v", err, tt.wantErr)
			}
			if tt.in != nil {
				assertState(t, tt.in, tt.want)
			}
		})
	}
}

func TestSubmit(t *testing.T) {
	tests := []struct {
		name    string
		in      *state.State
		gateID  string
		wantErr error
		want    *state.State
	}{
		{
			name:   "sets pending and the gate id",
			in:     &state.State{Phase: "Plan", LembasCompleted: true, MetadataUpdated: true},
			gateID: "gate-Plan-42",
			want:   &state.State{Phase: "Plan", GatePending: true, GateID: ptr("gate-Plan-42"), LembasCompleted: true, MetadataUpdated: true},
		},
		{
			name:    "gate already pending",
			in:      &state.State{Phase: "Plan", GatePending: true, GateID: ptr("gate-Plan-1")},
			gateID:  "gate-Plan-2",
			wantErr: state.ErrGatePending,
			want:    &state.State{Phase: "Plan", GatePending: true, GateID: ptr("gate-Plan-1")},
		},
		{
			name:    "held quest cannot submit",
			in:      &state.State{Phase: "Plan", Held: true},
			gateID:  "gate-Plan-2",
			wantErr: state.ErrHeld,
			want:    &state.State{Phase: "Plan", Held: true},
		},
		{
			name:    "nil state",
			gateID:  "gate-Plan-2",
			wantErr: state.ErrNilState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := state.Submit(tt.in, tt.gateID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Submit() error = %v, want %v", err, tt.wantErr)
			}
			if tt.in != nil {
				assertState(t, tt.in, tt.want)
			}
		})
	}
}

func TestSubmit_EmptyGateID(t *testing.T) {
	s := &state.State{Phase: "Plan"}
	if err := state.Submit(s, ""); err == nil {
		t.Fatal("Submit() with an empty gate id = nil, want an error")
	}
	if s.GatePending {
		t.Error("a rejected submit must not set gate_pending")
	}
}

func TestReset(t *testing.T) {
	tests := []struct {
		name string
		in   *state.State
		want *state.State
	}{
		{
			name: "clears gate and prereq flags, keeps phase",
			in:   &state.State{Phase: "Adversarial", GatePending: true, GateID: ptr("gate-Adversarial-1"), LembasCompleted: true, MetadataUpdated: true},
			want: &state.State{Phase: "Adversarial"},
		},
		{
			name: "leaves hold state alone",
			in:   &state.State{Phase: "Implement", GatePending: true, Held: true, HeldReason: ptr("paused")},
			want: &state.State{Phase: "Implement", Held: true, HeldReason: ptr("paused")},
		},
		{
			name: "already clean is a no-op",
			in:   &state.State{Phase: "Onboard"},
			want: &state.State{Phase: "Onboard"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.Reset(tt.in)
			assertState(t, tt.in, tt.want)
		})
	}

	state.Reset(nil) // must not panic
}

// assertState compares the gate-relevant fields of got and want.
func assertState(t *testing.T, got, want *state.State) {
	t.Helper()
	if want == nil {
		return
	}
	if got.Phase != want.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, want.Phase)
	}
	if got.GatePending != want.GatePending {
		t.Errorf("GatePending = %v, want %v", got.GatePending, want.GatePending)
	}
	if !eqPtr(got.GateID, want.GateID) {
		t.Errorf("GateID = %v, want %v", deref(got.GateID), deref(want.GateID))
	}
	if got.LembasCompleted != want.LembasCompleted {
		t.Errorf("LembasCompleted = %v, want %v", got.LembasCompleted, want.LembasCompleted)
	}
	if got.MetadataUpdated != want.MetadataUpdated {
		t.Errorf("MetadataUpdated = %v, want %v", got.MetadataUpdated, want.MetadataUpdated)
	}
	if got.Held != want.Held {
		t.Errorf("Held = %v, want %v", got.Held, want.Held)
	}
	if !eqPtr(got.HeldReason, want.HeldReason) {
		t.Errorf("HeldReason = %v, want %v", deref(got.HeldReason), deref(want.HeldReason))
	}
}

func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// Nothing stops two quests from being registered against the same worktree
// (fellowship_quests.worktree has no unique index yet), and a hook that
// resolved to a different one on different runs would enforce a different
// quest's gate. The newest registration wins, every time.
func TestFindQuest_DuplicateWorktreeIsDeterministic(t *testing.T) {
	d := db.OpenTest(t)
	worktree := "/repo/.worktrees/quest"

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		for _, name := range []string{"quest-old", "quest-new"} {
			if err := sqlitex.Execute(conn,
				`INSERT INTO fellowship_quests (name, task_description, worktree) VALUES (:n, 't', :w)`,
				&sqlitex.ExecOptions{Named: map[string]any{":n": name, ":w": worktree}}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		var got string
		if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
			var err error
			got, err = state.FindQuest(conn, worktree)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if got != "quest-new" {
			t.Fatalf("FindQuest = %q, want the most recently registered quest-new", got)
		}
	}
}
