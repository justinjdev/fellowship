// `fellowship notes ...`: the shared notice board.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/notes"
)

func runNotes(d *db.DB, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fellowship notes <post|scan|list|clear>")
		return 1
	}
	switch args[0] {
	case "post":
		return runNotesPost(d, args[1:])
	case "scan":
		return runNotesScan(d, args[1:])
	case "list":
		return runNotesList(d, args[1:])
	case "clear":
		return runNotesClear(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown notes command: %s\n", args[0])
		return 1
	}
}

func runNotesPost(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("notes post", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	topic := fs.String("topic", "", "Topic tag")
	files := fs.String("files", "", "Comma-separated file paths")
	discovery := fs.String("discovery", "", "Discovery description")
	fs.Parse(args)

	if *quest == "" || *topic == "" || *discovery == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship notes post --quest NAME --topic TOPIC --discovery \"TEXT\" [--files FILE,FILE]")
		return 1
	}

	fileList := splitCSV(*files)

	entry := notes.Entry{
		Quest:     *quest,
		Topic:     *topic,
		Files:     fileList,
		Discovery: *discovery,
	}
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return notes.Post(conn, entry)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Posted to notes: [%s] %s\n", *topic, *discovery)
	return 0
}

func runNotesScan(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("notes scan", flag.ExitOnError)
	files := fs.String("files", "", "Comma-separated file paths to match")
	topics := fs.String("topics", "", "Comma-separated topics to match")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	fileList := splitCSV(*files)
	topicList := splitCSV(*topics)

	var entries []notes.Entry
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		entries, err = notes.Scan(conn, fileList, topicList)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(entries) == 0 {
		fmt.Println("No matching notes entries.")
		return 0
	}

	for _, e := range entries {
		fmt.Printf("[%s] %s (%s): %s\n", e.Topic, e.Quest, strings.Join(e.Files, ", "), e.Discovery)
	}
	fmt.Printf("\n%d entries found.\n", len(entries))
	return 0
}

func runNotesList(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("notes list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	var entries []notes.Entry
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		entries, err = notes.Load(conn)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		if entries == nil {
			entries = []notes.Entry{}
		}
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(entries) == 0 {
		fmt.Println("No notes entries.")
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tQUEST\tTOPIC\tDISCOVERY")
	for _, e := range entries {
		ts := e.Timestamp
		if len(ts) > 19 {
			ts = ts[:19]
		}
		disc := e.Discovery
		if len(disc) > 60 {
			disc = disc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ts, e.Quest, e.Topic, disc)
	}
	w.Flush()
	fmt.Printf("\n%d entries total.\n", len(entries))
	return 0
}

func runNotesClear(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("notes clear", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: fellowship notes clear")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return notes.Clear(conn)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Println("Notes cleared.")
	return 0
}
