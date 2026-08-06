// Command restorecheck verifies that a SQLite backup can be restored safely.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/muzaffer/internship-tracker/internal/backup"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "restorecheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("restorecheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	snapshot := flags.String("backup", "", "SQLite snapshot path")
	migrationsPath := flags.String("migrations", "migrations", "directory containing this release's migrations")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *snapshot == "" {
		return errors.New("-backup is required")
	}
	if err := backup.Verify(ctx, *snapshot, os.DirFS(*migrationsPath)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "verified=%s\n", *snapshot)
	return err
}
