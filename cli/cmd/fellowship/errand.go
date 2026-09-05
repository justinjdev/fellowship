// `fellowship errand ...`: the side-channel task list quests share.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/todo"
)

func runErrand(d *db.DB, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand <init|list|add|update|show>")
		return 1
	}

	switch args[0] {
	case "init":
		return runErrandInit(d, args[1:])
	case "list":
		return runErrandList(d, args[1:])
	case "add":
		return runErrandAdd(d, args[1:])
	case "update":
		return runErrandUpdate(d, args[1:])
	case "show":
		return runErrandShow(d, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown errand command: %s\n", args[0])
		return 1
	}
}

func runErrandInit(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand init", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	task := fs.String("task", "", "Task description")
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
		fmt.Fprintln(os.Stderr, "usage: fellowship errand init --quest <name> [--dir <worktree>] [--task \"desc\"]")
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return todo.Init(conn, questName, *task)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Errand tracking initialized for quest %q\n", questName)
	return 0
}

func runErrandList(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand list", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
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
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var items []todo.Todo
	var done, total int
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		items, err = todo.List(conn, questName)
		if err != nil {
			return err
		}
		done, total, err = todo.Progress(conn, questName)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if len(items) == 0 {
		fmt.Println("No errands.")
		return 0
	}

	for _, item := range items {
		phase := ""
		if item.Phase != "" {
			phase = fmt.Sprintf(" [%s]", item.Phase)
		}
		deps := ""
		if len(item.DependsOn) > 0 {
			deps = fmt.Sprintf(" (depends: %s)", strings.Join(item.DependsOn, ", "))
		}
		fmt.Printf("%-6s %-8s %s%s%s\n", item.ID, item.Status, item.Description, phase, deps)
	}

	fmt.Printf("\nProgress: %d/%d done\n", done, total)
	return 0
}

func runErrandAdd(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand add", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	phase := fs.String("phase", "", "Quest phase")
	fs.Parse(args)

	desc := strings.Join(fs.Args(), " ")
	if desc == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand add [--quest <name>] [--dir <worktree>] \"description\"")
		return 1
	}

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

	var id string
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		var err error
		id, err = todo.Add(conn, questName, desc, *phase)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Added %s: %s\n", id, desc)
	return 0
}

func runErrandUpdate(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand update", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fellowship errand update [--quest <name>] [--dir <worktree>] <id> <status>")
		return 1
	}

	id := remaining[0]
	statusStr := remaining[1]

	ws, ok := todo.ValidStatus(statusStr)
	if !ok {
		fmt.Fprintf(os.Stderr, "fellowship: invalid status %q (use: %s)\n", statusStr, strings.Join(todo.Statuses(), ", "))
		return 1
	}

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

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		return todo.UpdateStatus(conn, questName, id, ws)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	fmt.Printf("Updated %s → %s\n", id, statusStr)
	return 0
}

func runErrandShow(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("errand show", flag.ExitOnError)
	quest := fs.String("quest", "", "Quest name")
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
		fmt.Fprintln(os.Stderr, "fellowship: no quest found. Use --quest <name>.")
		return 1
	}

	var list *todo.QuestTodoList
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		items, err := todo.List(conn, questName)
		if err != nil {
			return err
		}
		list = &todo.QuestTodoList{
			QuestName: questName,
			Items:     items,
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	data, _ := json.MarshalIndent(list, "", "  ")
	fmt.Println(string(data))
	return 0
}
