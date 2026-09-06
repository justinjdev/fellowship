// The two teammate-side commands that carry enforcement the agent-teams API
// used to route through task updates: `phase confirm` (a gate prerequisite)
// and `complete` (the end of a quest).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func runPhase(d *db.DB, args []string) int {
	switch args[0] {
	case "confirm":
		return runPhaseConfirm(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown phase command: %s\nusage: fellowship phase confirm --dir <worktree> --phase <phase>\n", args[0])
		return 1
	}
}

// runPhaseConfirm records the phase-confirmation prerequisite for a gate.
//
// The phase named must be a quest phase and must be the one the quest is in:
// the confirmation is the teammate saying "I know which phase I am closing",
// and it is never a way to move the phase — a mismatch is refused, and the
// row is left exactly as it was.
func runPhaseConfirm(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("phase confirm", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	phase := fs.String("phase", "", "The phase the quest is in (required)")
	fs.Parse(args)

	if *phase == "" {
		fmt.Fprintf(os.Stderr, "usage: fellowship phase confirm --dir <worktree> --phase <%s>\n", strings.Join(state.Phases(), "|"))
		return 1
	}
	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	questName := resolveDirQuest(d, *dir)
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest state found")
		return 1
	}

	var notice string
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		s, err := state.Load(conn, questName)
		if err != nil {
			return err
		}
		recorded, n := hooks.ConfirmPhase(s, *phase)
		if !recorded {
			notice = n
			return nil
		}
		if err := state.Upsert(conn, s); err != nil {
			return err
		}
		return events.Record(conn, events.Event{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     questName,
			Type:      events.MetadataUpdated,
			Phase:     s.Phase,
			Detail:    "Phase confirmed: " + s.Phase,
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if notice != "" {
		fmt.Fprintln(os.Stderr, notice)
		return 1
	}
	fmt.Printf("Phase %s confirmed for %s.\n", *phase, questName)
	return 0
}

// runComplete ends a quest. It is allowed only in the terminal phase with no
// gate pending and no hold — hooks.CompletionCheck, the same rule gate-guard
// applies to the Bash form — and it marks the quest's history and its
// fellowship entry completed in one transaction.
func runComplete(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("complete", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	questName := resolveDirQuest(d, *dir)
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest state found")
		return 1
	}

	var refusal string
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		s, err := state.Load(conn, questName)
		if err != nil {
			return err
		}
		if result := hooks.CompletionCheck(s); result.Block {
			refusal = result.Message
			return nil
		}
		if err := hooks.MarkHistoryCompleted(conn, questName); err != nil {
			return err
		}
		if err := fellowship.UpdateQuest(conn, questName, map[string]any{"status": "completed"}); err != nil {
			return err
		}
		return events.Record(conn, events.Event{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     questName,
			Type:      events.QuestCompleted,
			Phase:     s.Phase,
			Detail:    "Quest completed",
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if refusal != "" {
		fmt.Fprintln(os.Stderr, refusal)
		return 1
	}
	fmt.Printf("Quest %s completed.\n", questName)
	return 0
}
