// `fellowship health`: quest health sweep (stalled/zombie/struggling).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/health"
)

func runHealth(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	threshold := fs.Int("threshold", 10, "Gate pending timeout in minutes")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	opts := health.DefaultOptions()
	opts.GateThreshold = time.Duration(*threshold) * time.Minute

	var report *health.HealthReport
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		report, err = health.Sweep(conn, opts)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Print(health.FormatTable(report))
	return 0
}
