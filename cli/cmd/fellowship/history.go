// `fellowship history show`: the per-quest phase/gate/file journal.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/history"
)

func runHistory(d *db.DB, args []string) int {
	ctx := context.Background()
	if len(args) < 1 || args[0] != "show" {
		fmt.Fprintln(os.Stderr, "usage: fellowship history show [--quest <name>] [--dir <worktree>] [--json]")
		return 1
	}

	fs := flag.NewFlagSet("history show", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name (default: auto-detect from worktree)")
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args[1:])

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	questName := *quest
	if questName == "" {
		questName = resolveDirQuest(d, *dir)
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var c *history.QuestHistory
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		c, err = history.Load(conn, questName)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Printf("Quest History: %s\n", c.QuestName)
	fmt.Printf("Status:   %s\n", c.Status)
	fmt.Printf("Task:     %s\n", c.Task)
	fmt.Printf("Respawns: %d\n", c.Respawns)
	fmt.Println()

	if len(c.PhasesCompleted) > 0 {
		fmt.Println("Phases Completed:")
		for _, p := range c.PhasesCompleted {
			dur := ""
			if p.DurationS > 0 {
				dur = fmt.Sprintf(" (%ds)", p.DurationS)
			}
			fmt.Printf("  - %s at %s%s\n", p.Phase, p.CompletedAt, dur)
		}
		fmt.Println()
	}

	if len(c.GateHistory) > 0 {
		fmt.Println("Gate History:")
		for _, g := range c.GateHistory {
			reason := ""
			if g.Reason != "" {
				reason = fmt.Sprintf(" — %s", g.Reason)
			}
			fmt.Printf("  - [%s] %s %s%s\n", g.Timestamp, g.Phase, g.Action, reason)
		}
		fmt.Println()
	}

	if len(c.FilesTouched) > 0 {
		fmt.Printf("Files Touched (%d):\n", len(c.FilesTouched))
		for _, f := range c.FilesTouched {
			fmt.Printf("  - %s\n", f)
		}
	}

	return 0
}
