// `fellowship failures ...`: post-mortem records and the scans over them.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/failures"
)

func runFailures(d *db.DB, args []string) int {
	switch args[0] {
	case "create":
		return runFailuresCreate(d, args[1:])
	case "scan":
		return runFailuresScan(d, args[1:])
	case "infer":
		return runFailuresInfer(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown failures command: %s\n", args[0])
		return 1
	}
}

func runFailuresCreate(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("failures create", flag.ExitOnError)
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	var input failures.CreateInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: reading JSON from stdin: %v\n", err)
		return 1
	}

	input.ExpiryDays = datadir.FailuresExpiryDays(gitRootOrCwd(), failures.DefaultExpiryDays)

	var id int64
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		var err error
		id, err = failures.Create(conn, &input)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Failure record created (id=%d)\n", id)
	return 0
}

func runFailuresScan(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("failures scan", flag.ExitOnError)
	files := fs.String("files", "", "Comma-separated file paths to match")
	modules := fs.String("modules", "", "Comma-separated module names to match")
	tags := fs.String("tags", "", "Comma-separated tags to match")
	all := fs.Bool("all", false, "Return every unexpired failure record (ignores the other filters)")
	dir := fs.String("dir", "", "Repo or worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	opts := failures.ScanOptions{All: *all}
	if *files != "" {
		opts.Files = strings.Split(*files, ",")
	}
	if *modules != "" {
		opts.Modules = strings.Split(*modules, ",")
	}
	if *tags != "" {
		opts.Tags = strings.Split(*tags, ",")
	}

	expiryDays := datadir.FailuresExpiryDays(gitRootOrCwd(), failures.DefaultExpiryDays)

	var matches []failures.Failure
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		matches, err = failures.Scan(conn, opts, expiryDays)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(matches, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runFailuresInfer(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("failures infer", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name (default: the quest registered for --dir)")
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	fs.Parse(args)

	if err := checkDir(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	questName := *quest
	if questName == "" {
		questName = resolveDirQuest(d, *dir)
	}
	if questName == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship failures infer --quest <name> | --dir <worktree>")
		return 1
	}

	var id int64
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		var err error
		id, err = failures.Infer(conn, questName)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Inferred failure record created (id=%d)\n", id)
	return 0
}
