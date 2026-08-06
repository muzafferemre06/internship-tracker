// Command backup creates one consistent, pre-deploy SQLite snapshot.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/muzaffer/internship-tracker/internal/backup"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "backup:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	databasePath := flags.String("database", "", "existing SQLite database path")
	directory := flags.String("directory", "", "directory for the generated snapshot")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *databasePath == "" || *directory == "" {
		return fmt.Errorf("-database and -directory are required")
	}

	source, err := backup.OpenSource(*databasePath)
	if err != nil {
		return err
	}
	defer source.Close()

	result, err := backup.Create(ctx, source, *directory, now())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "snapshot=%s\n", result.Path)
	return err
}
