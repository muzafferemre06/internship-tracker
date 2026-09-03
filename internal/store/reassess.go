package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

// StoredAnalysis is a persisted analysis reloaded for deterministic
// re-assessment. It carries no model usage or notification state.
type StoredAnalysis struct {
	ListingID     string
	OpportunityID string
	Analysis      domain.ListingAnalysis
	Current       struct {
		Visibility domain.VisibilityLayer
		Score      int
		Reason     string
	}
}

// StoredAnalyses returns every successfully processed analysis joined to its
// canonical opportunity, ordered by listing_id for stable output.
func (r *SQLiteRepository) StoredAnalyses(ctx context.Context) ([]StoredAnalysis, error) {
	query := `
		SELECT
			la.listing_id,
			COALESCE(la.opportunity_type, '') AS opportunity_type,
			COALESCE(la.is_application_open, 0) AS is_application_open,
			COALESCE(la.is_relevant, 0) AS is_relevant,
			la.matching_areas_json,
			la.class_year_requirement,
			la.gpa_requirement,
			COALESCE(la.location, '') AS location,
			COALESCE(la.work_model, '') AS work_model,
			COALESCE(la.eligibility_status, '') AS eligibility_status,
			la.application_deadline,
			COALESCE(la.summary, '') AS summary,
			COALESCE(la.confidence, 0) AS confidence,
			la.needs_user_decision,
			COALESCE(la.decision_question, '') AS decision_question,
			COALESCE(la.provider, '') AS provider,
			COALESCE(la.model, '') AS model,
			lo.opportunity_id,
			o.visibility_layer,
			o.match_score,
			o.assessment_reason
		FROM listing_analyses la
		INNER JOIN listing_opportunities lo ON lo.listing_id = la.listing_id
		INNER JOIN opportunities o ON o.id = lo.opportunity_id
		WHERE la.processing_status = 'processed'
		ORDER BY la.listing_id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying stored analyses: %w", err)
	}
	defer rows.Close()

	var results []StoredAnalysis
	for rows.Next() {
		var sa StoredAnalysis
		var isAppOpen, isRelevant, needsUserDecision int
		var matchingAreasJSON []byte
		var classYear sql.NullInt64
		var gpa sql.NullFloat64
		var applicationDeadline sql.NullString
		var oppType string

		err := rows.Scan(
			&sa.ListingID, &oppType, &isAppOpen, &isRelevant, &matchingAreasJSON,
			&classYear, &gpa, &sa.Analysis.Location, &sa.Analysis.WorkModel,
			&sa.Analysis.Eligibility, &applicationDeadline, &sa.Analysis.Summary,
			&sa.Analysis.Confidence, &needsUserDecision, &sa.Analysis.DecisionQuestion,
			&sa.Analysis.Provider, &sa.Analysis.Model, &sa.OpportunityID,
			&sa.Current.Visibility, &sa.Current.Score, &sa.Current.Reason,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning stored analysis: %w", err)
		}

		sa.Analysis.OpportunityType = domain.OpportunityType(oppType)
		sa.Analysis.ApplicationOpen = isAppOpen == 1
		sa.Analysis.Relevant = isRelevant == 1
		sa.Analysis.NeedsUserDecision = needsUserDecision == 1

		if len(matchingAreasJSON) > 0 {
			if err := json.Unmarshal(matchingAreasJSON, &sa.Analysis.MatchingAreas); err != nil {
				return nil, fmt.Errorf("unmarshaling matching areas for listing %s: %w", sa.ListingID, err)
			}
		}

		if classYear.Valid {
			cy := int(classYear.Int64)
			sa.Analysis.ClassRequirement = &cy
		}
		if gpa.Valid {
			sa.Analysis.GPARequirement = &gpa.Float64
		}
		if applicationDeadline.Valid {
			t, err := time.Parse(time.RFC3339Nano, applicationDeadline.String)
			if err != nil {
				return nil, fmt.Errorf("parsing application deadline for listing %s: %w", sa.ListingID, err)
			}
			sa.Analysis.ApplicationDueAt = &t
		}

		results = append(results, sa)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stored analyses: %w", err)
	}

	return results, nil
}

// ReapplyAssessment recomputes the stored opportunity assessment for one
// listing. It updates only opportunity scoring and evidence rows; it never
// enqueues a notification and never contacts a model provider.
func (r *SQLiteRepository) ReapplyAssessment(ctx context.Context, stored StoredAnalysis, assessment domain.MatchAssessment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	analysisCopy := stored.Analysis
	analysisCopy.Assessment = assessment

	if err := r.persistOpportunityAssessment(ctx, tx, stored.ListingID, stored.OpportunityID, analysisCopy); err != nil {
		return fmt.Errorf("persisting opportunity assessment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// CountProcessedAnalyses reports how many processed analyses match the
// optional provider filter, without modifying anything.
func (r *SQLiteRepository) CountProcessedAnalyses(ctx context.Context, provider string) (int64, error) {
	query := `SELECT COUNT(*) FROM listing_analyses WHERE processing_status = 'processed'`
	var args []any

	provider = strings.TrimSpace(provider)
	if provider != "" {
		query += ` AND provider = ?`
		args = append(args, provider)
	}

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting processed analyses: %w", err)
	}

	return count, nil
}

// InvalidateProcessedAnalyses marks stored analyses as pending so the existing
// retry path re-extracts them with the currently configured model provider.
// Only the processing status is touched: the previous extraction values stay
// in place until a successful re-analysis overwrites them, so a failed or
// interrupted pass never leaves a listing without data. Returns how many rows
// were marked.
func (r *SQLiteRepository) InvalidateProcessedAnalyses(ctx context.Context, provider string) (int64, error) {
	query := `UPDATE listing_analyses SET processing_status = 'pending', retry_count = 0 WHERE processing_status = 'processed'`
	var args []any

	provider = strings.TrimSpace(provider)
	if provider != "" {
		query += ` AND provider = ?`
		args = append(args, provider)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("invalidating processed analyses: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting rows affected: %w", err)
	}

	return rows, nil
}
