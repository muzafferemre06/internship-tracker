package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/matching"
)

// promotableToNotification reports whether an analysis qualifies for the
// trusted-source notification promotion, ignoring the source trust level
// (which requires a database lookup).
func promotableToNotification(analysis *domain.ListingAnalysis) bool {
	if analysis == nil || analysis.Confidence < 0.80 || analysis.Assessment.Score < 80 ||
		analysis.Assessment.Visibility == domain.VisibilityRejected || analysis.Assessment.Visibility == domain.VisibilityReview {
		return false
	}
	if !matching.NotificationOpportunity(analysis.OpportunityType) {
		return false
	}
	return true
}

// applyTrustedNotificationLayer promotes a high-scoring analysis from a trusted
// source into the notification layer. This store-level promotion once bypassed
// the matching layer's opportunity-type rule, which let senior full-time roles
// from trusted sources become push notifications; both gates must now agree.
func (r *SQLiteRepository) applyTrustedNotificationLayer(ctx context.Context, tx *sql.Tx, listingID string, analysis *domain.ListingAnalysis) error {
	if !promotableToNotification(analysis) {
		return nil
	}

	var trust string
	if err := tx.QueryRowContext(ctx, `
		SELECT company_sources.trust_level FROM listings
		JOIN company_sources ON company_sources.id = listings.source_id WHERE listings.id = ?
	`, listingID).Scan(&trust); err != nil {
		return fmt.Errorf("load assessment source trust: %w", err)
	}

	switch trust {
	case "official_company", "official_ats", "verified_newsletter":
		analysis.Assessment.Visibility = domain.VisibilityNotification
		analysis.Assessment.PushEligible = true
		analysis.Assessment.Reason = "strong_match"
	}

	return nil
}
