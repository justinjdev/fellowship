// `fellowship events ...`: the quest event log (activity tidings).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
)

func runEvents(d *db.DB, args []string) int {
	ctx := context.Background()
	if len(args) > 0 && args[0] == "post" {
		return runEventsPost(d, args[1:])
	}

	fs := flag.NewFlagSet("events", flag.ExitOnError)
	problems := fs.Bool("problems", false, "Show only detected problems")
	quest := fs.String("quest", "", "Show events for one quest only")
	limit := fs.Int("limit", 20, "Maximum events to show (0 for all)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	if *problems {
		var detected []events.Problem
		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			var err error
			detected, err = events.DetectProblems(conn)
			return err
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		if *jsonOut {
			data, _ := json.MarshalIndent(detected, "", "  ")
			fmt.Println(string(data))
			return 0
		}
		if len(detected) == 0 {
			fmt.Println("No problems detected.")
			return 0
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(tw, "SEVERITY\tQUEST\tTYPE\tMESSAGE\n")
		for _, p := range detected {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Severity, p.Quest, p.Type, p.Message)
		}
		tw.Flush()
		return 0
	}

	var evts []events.Event
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		if *quest != "" {
			evts, err = events.Read(conn, *quest, *limit)
		} else {
			evts, err = events.ReadAll(conn, *limit)
		}
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(evts, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(evts) == 0 {
		fmt.Println("No events.")
		return 0
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "TIME\tQUEST\tTYPE\tPHASE\tDETAIL\n")
	for _, e := range evts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.Timestamp, e.Quest, e.Type, e.Phase, e.Detail)
	}
	tw.Flush()
	return 0
}

// runEventsPost records an event. It exists so agents (notably the palantir
// monitor) can append structured activity to the event log without
// hand-rolling JSON lines.
func runEventsPost(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("events post", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest the event is about (required)")
	eventType := fs.String("type", "", "Event type (required)")
	phase := fs.String("phase", "", "Quest phase")
	detail := fs.String("detail", "", "Detail text (required); \"-\" reads it from stdin")
	fs.Parse(args)

	// The detail is often text written by another agent (a notes entry, an
	// alert). Reading it from stdin lets the caller pass it without ever
	// putting it on a shell command line, where its metacharacters would run.
	if *detail == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: reading --detail from stdin: %v\n", err)
			return 1
		}
		*detail = strings.TrimRight(string(data), "\n")
	}

	if *quest == "" || *eventType == "" || *detail == "" {
		fmt.Fprintln(os.Stderr, `usage: fellowship events post --quest <name> --type <type> --detail "TEXT" [--phase PHASE]   (--detail - reads the text from stdin)`)
		return 1
	}

	tt, ok := events.ValidType(*eventType)
	if !ok {
		fmt.Fprintf(os.Stderr, "fellowship: invalid event type %q (valid: %s)\n", *eventType, strings.Join(events.Types(), ", "))
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return events.Record(conn, events.Event{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     *quest,
			Type:      tt,
			Phase:     *phase,
			Detail:    *detail,
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Recorded %s event for %s.\n", tt, *quest)
	return 0
}
