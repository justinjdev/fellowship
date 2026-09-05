// `fellowship group ...`: grouped quest/scout progress and batch approval.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/group"
)

func runGroup(d *db.DB, args []string) int {
	ctx := context.Background()
	sub := args[0]
	rest := args[1:]

	switch sub {
	case "list":
		fs := flag.NewFlagSet("group list", flag.ExitOnError)
		fs.Parse(rest)

		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			return group.List(conn)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	case "show":
		// --json can come before or after the positional name (docs write it
		// both ways), and Go's flag package only recognizes flags up to the
		// first positional argument — so pull it out before handing the rest
		// to a FlagSet.
		jsonOut, positional := extractJSONFlag(rest)
		fs := flag.NewFlagSet("group show", flag.ExitOnError)
		fs.Parse(positional)

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fellowship group show <name> [--json]")
			return 1
		}
		name := fs.Arg(0)

		if jsonOut {
			var detail *group.Detail
			if err := d.WithConn(ctx, func(conn *db.Conn) error {
				var err error
				detail, err = group.LoadDetail(conn, name)
				return err
			}); err != nil {
				fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
				return 1
			}
			data, _ := json.MarshalIndent(detail, "", "  ")
			fmt.Println(string(data))
			return 0
		}

		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			return group.Show(conn, name)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	case "approve":
		fs := flag.NewFlagSet("group approve", flag.ExitOnError)
		fs.Parse(rest)

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fellowship group approve <name>")
			return 1
		}
		name := fs.Arg(0)

		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			return group.Approve(conn, name)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown group command: %s\n", sub)
		return 1
	}
}
