// Command reassess re-applies the deterministic matching rules to analyses
// already stored in the database. It calls no model provider, enqueues no
// notification and never rewrites the model-produced analysis fields; only
// opportunity scoring and visibility are recomputed.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/matching"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type summary struct {
	DatabasePath string `json:"database_path"`
	Applied      bool   `json:"applied"`
	Total        int    `json:"total"`
	Changed      int    `json:"changed"`
	Updated      int    `json:"updated"`
	Failed       int    `json:"failed"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "reassess:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("reassess", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	databasePath := flags.String("database", "", "existing SQLite database path")
	profilePath := flags.String("profile", "", "candidate profile JSON path")
	migrationsPath := flags.String("migrations", "migrations", "migrations directory")
	apply := flags.Bool("apply", false, "apply changes to database (dry run by default)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *databasePath == "" {
		return errors.New("-database is required")
	}
	if *profilePath == "" {
		return errors.New("-profile is required")
	}

	absDBPath, err := filepath.Abs(*databasePath)
	if err != nil {
		return fmt.Errorf("resolving database path: %w", err)
	}

	stat, err := os.Stat(absDBPath)
	if err != nil {
		return fmt.Errorf("stat database: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("database %q is not a regular file", absDBPath)
	}

	candidateProfile, err := config.LoadCandidateProfile(*profilePath)
	if err != nil {
		return fmt.Errorf("loading profile: %w", err)
	}

	profile := matchingProfile(candidateProfile)

	db, err := database.Open(ctx, absDBPath, os.DirFS(*migrationsPath))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	repo, err := store.NewSQLiteRepository(db)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	analyses, err := repo.StoredAnalyses(ctx)
	if err != nil {
		return fmt.Errorf("loading stored analyses: %w", err)
	}

	sum := summary{
		DatabasePath: absDBPath,
		Applied:      *apply,
		Total:        len(analyses),
	}

	var rowErrors []error

	for _, stored := range analyses {
		newAssessment := matching.Assess(profile, matching.Input{Analysis: stored.Analysis}).Domain()

		changed := newAssessment.Visibility != stored.Current.Visibility ||
			newAssessment.Score != stored.Current.Score ||
			newAssessment.Reason != stored.Current.Reason

		if !changed {
			continue
		}

		sum.Changed++

		fmt.Fprintf(output, "listing=%s layer=%s->%s score=%d->%d reason=%s->%s\n",
			stored.ListingID,
			stored.Current.Visibility, newAssessment.Visibility,
			stored.Current.Score, newAssessment.Score,
			stored.Current.Reason, newAssessment.Reason,
		)

		if *apply {
			if err := repo.ReapplyAssessment(ctx, stored, newAssessment); err != nil {
				rowErrors = append(rowErrors, fmt.Errorf("listing %s: %w", stored.ListingID, err))
				sum.Failed++
			} else {
				sum.Updated++
			}
		}
	}

	if err := json.NewEncoder(output).Encode(sum); err != nil {
		return fmt.Errorf("encoding summary: %w", err)
	}

	return errors.Join(rowErrors...)
}

func matchingProfile(profile config.CandidateProfile) matching.Profile {
	return matching.Profile{
		ClassYear:                   profile.Education.ClassYear,
		GPA:                         profile.Education.GPA,
		FocusAreas:                  append([]string(nil), profile.FocusAreas...),
		PrimaryLocations:            append([]string(nil), profile.LocationPreferences.Primary...),
		SummerOtherCities:           profile.LocationPreferences.SummerOtherCities,
		TermTimePartTimeOtherCities: profile.LocationPreferences.TermTimePartTimeOtherCities,
	}
}
