package main

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
)

// The unregistered-worktree block and the worktree-guard's arming condition are
// the same question — "is a fellowship running?" — and used to have two
// answers. The block asked whether a fellowship row existed, which is sticky:
// the row is never deleted, so every linked worktree of the repo stayed
// unusable forever after a single `state init`.
func TestUnregisteredQuestWorktree_TracksLiveQuests(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-live-beta")
	stray := addWorktree(t, root, "quest-stray-beta")

	d := fellowshipWith(t, root, map[string]string{"quest-live-beta": worktree})

	// A live quest: the stray worktree is a teammate somewhere it should not be.
	if !unregisteredQuestWorktree(context.Background(), d, stray, stray) {
		t.Error("a stray worktree during a live fellowship should block")
	}
	if got := runHookWith("gate-guard", bashInput("ls"), stray, d); got != 2 {
		t.Errorf("gate-guard in a stray worktree: exit %d, want 2", got)
	}

	// The quest finishes. Nothing is running any more.
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return fellowship.UpdateQuest(conn, "quest-live-beta", map[string]any{"status": "completed"})
	}); err != nil {
		t.Fatal(err)
	}
	if unregisteredQuestWorktree(context.Background(), d, stray, stray) {
		t.Error("no quest is live: an unrelated worktree must not be blocked")
	}
	if got := runHookWith("gate-guard", bashInput("ls"), stray, d); got != 0 {
		t.Errorf("gate-guard after the fellowship finished: exit %d, want 0", got)
	}

	// The main tree is never this rule's business.
	if unregisteredQuestWorktree(context.Background(), d, root, root) {
		t.Error("the main tree must never read as an unregistered worktree")
	}
}
