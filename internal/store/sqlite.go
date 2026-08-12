package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/opportunity"
)

type SQLiteRepository struct {
	db  *sql.DB
	now func() time.Time
}

var ErrListingNotFound = errors.New("listing not found")
var ErrSourceNotFound = errors.New("source not found")
var ErrOpportunityNotFound = errors.New("opportunity not found")

func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &SQLiteRepository{db: db, now: time.Now}, nil
}

// ReconcileOpportunities applies the current deterministic matcher to analyzed
// rows that may have been backfilled one-listing-per-opportunity by a migration.
// It is safe to run on every startup: established compatible memberships stay
// unchanged and match events are idempotent.
func (r *SQLiteRepository) ReconcileOpportunities(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT listings.id, COALESCE(listing_analyses.location, '')
		FROM listings
		JOIN listing_analyses ON listing_analyses.listing_id = listings.id
		WHERE listing_analyses.processing_status = 'processed'
		ORDER BY listings.first_seen_at, listings.id
	`)
	if err != nil {
		return fmt.Errorf("query analyzed opportunities for reconciliation: %w", err)
	}
	type analyzedListing struct {
		id       string
		location string
	}
	items := make([]analyzedListing, 0)
	for rows.Next() {
		var item analyzedListing
		if err := rows.Scan(&item.id, &item.location); err != nil {
			rows.Close()
			return fmt.Errorf("scan opportunity reconciliation listing: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read opportunity reconciliation listings: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close opportunity reconciliation listings: %w", err)
	}

	for _, item := range items {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin opportunity reconciliation: %w", err)
		}
		if _, err := r.resolveOpportunity(ctx, tx, item.id, domain.ListingAnalysis{Location: item.location}); err != nil {
			tx.Rollback()
			return fmt.Errorf("reconcile listing %q: %w", item.id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit opportunity reconciliation: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) RegisterSource(ctx context.Context, source domain.SourceRegistration) error {
	if err := validateSourceRegistration(source); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin source registration: %w", err)
	}
	defer tx.Rollback()

	trackingStatus := strings.TrimSpace(source.TrackingStatus)
	if trackingStatus == "" {
		trackingStatus = "active"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO companies(name, priority_group, tracking_status, tracking_phase)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			priority_group = excluded.priority_group,
			tracking_status = excluded.tracking_status,
			tracking_phase = excluded.tracking_phase,
			updated_at = CURRENT_TIMESTAMP
	`, source.Company, source.PriorityGroup, trackingStatus, strings.TrimSpace(source.TrackingPhase)); err != nil {
		return fmt.Errorf("upsert company %q: %w", source.Company, err)
	}

	var companyID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM companies WHERE name = ?", source.Company).Scan(&companyID); err != nil {
		return fmt.Errorf("find company %q: %w", source.Company, err)
	}

	strategy := strings.TrimSpace(source.Strategy)
	if strategy == "" {
		strategy = "legacy_html"
	}
	accessMode := strings.TrimSpace(source.AccessMode)
	if accessMode == "" {
		accessMode = "legacy"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_sources(
			company_id, source_key, source_type, url, adapter_type, strategy, enabled,
			access_mode, access_scope, minimum_interval_seconds,
			base_cooldown_seconds, maximum_cooldown_seconds,
			coverage_status, coverage_reason, coverage_reason_code, last_verified_at, trust_level
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_key) DO UPDATE SET
			company_id = excluded.company_id,
			source_type = excluded.source_type,
			url = excluded.url,
			adapter_type = excluded.adapter_type,
			strategy = excluded.strategy,
			enabled = excluded.enabled,
			access_mode = excluded.access_mode,
			access_scope = excluded.access_scope,
			minimum_interval_seconds = excluded.minimum_interval_seconds,
			base_cooldown_seconds = excluded.base_cooldown_seconds,
			maximum_cooldown_seconds = excluded.maximum_cooldown_seconds,
			coverage_status = excluded.coverage_status,
			coverage_reason = excluded.coverage_reason,
			coverage_reason_code = excluded.coverage_reason_code,
			last_verified_at = excluded.last_verified_at,
			trust_level = excluded.trust_level,
			updated_at = CURRENT_TIMESTAMP
	`, companyID, source.Key, source.Type, source.URL, source.Adapter, strategy, boolInt(source.Enabled),
		accessMode, strings.ToLower(strings.TrimSpace(source.AccessScope)), durationSeconds(source.MinimumInterval),
		durationSeconds(source.BaseCooldown), durationSeconds(source.MaximumCooldown),
		effectiveCoverageStatus(source), strings.TrimSpace(source.CoverageReason), strings.TrimSpace(source.CoverageReasonCode), nullableTime(source.LastVerifiedAt), effectiveTrustLevel(source)); err != nil {
		return fmt.Errorf("upsert source %q: %w", source.Key, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source registration: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) RegisterProgramWindow(ctx context.Context, program domain.ProgramWindow) error {
	if strings.TrimSpace(program.Key) == "" || strings.TrimSpace(program.Company) == "" ||
		strings.TrimSpace(program.Name) == "" || strings.TrimSpace(program.Type) == "" {
		return errors.New("program key, company, name and type are required")
	}
	parsedURL, err := url.ParseRequestURI(program.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return errors.New("program URL must be an absolute HTTP(S) URL")
	}
	if program.Status != "open" && program.Status != "closed" && program.Status != "unknown" {
		return fmt.Errorf("invalid program status %q", program.Status)
	}
	if program.OpensAt != nil && program.ClosesAt != nil && program.OpensAt.After(*program.ClosesAt) {
		return errors.New("program opens_at must not be after closes_at")
	}
	var companyID int64
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM companies WHERE name = ?", program.Company).Scan(&companyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("program company %q is not registered", program.Company)
		}
		return fmt.Errorf("find program company %q: %w", program.Company, err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO program_windows(company_id, program_key, name, program_type, url, status, opens_at, closes_at, last_verified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(program_key) DO UPDATE SET company_id = excluded.company_id, name = excluded.name,
			program_type = excluded.program_type, url = excluded.url, status = excluded.status,
			opens_at = excluded.opens_at, closes_at = excluded.closes_at,
			last_verified_at = excluded.last_verified_at, updated_at = CURRENT_TIMESTAMP
	`, companyID, strings.TrimSpace(program.Key), strings.TrimSpace(program.Name), strings.TrimSpace(program.Type),
		program.URL, program.Status, nullableTime(program.OpensAt), nullableTime(program.ClosesAt), nullableTime(program.LastVerifiedAt))
	if err != nil {
		return fmt.Errorf("upsert program window %q: %w", program.Key, err)
	}
	if err := r.projectProgramWindow(ctx, strings.TrimSpace(program.Key)); err != nil {
		return err
	}
	return nil
}

// projectProgramWindow keeps a period-based program distinct from listings
// while exposing its current official evidence through the common opportunity
// model. Unknown/open program facts remain reviewable rather than fabricated
// as a job listing.
func (r *SQLiteRepository) projectProgramWindow(ctx context.Context, key string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin program opportunity projection: %w", err)
	}
	defer tx.Rollback()
	var programID, companyID int64
	var name, programType, sourceURL, status, observedAt string
	if err := tx.QueryRowContext(ctx, `
		SELECT id, company_id, name, program_type, url, status,
			COALESCE(last_verified_at, created_at)
		FROM program_windows WHERE program_key = ?
	`, key).Scan(&programID, &companyID, &name, &programType, &sourceURL, &status, &observedAt); err != nil {
		return fmt.Errorf("load program window %q: %w", key, err)
	}
	kind := programOpportunityType(programType)
	layer, reason := domain.VisibilityReview, "program_window_requires_review"
	if status == "closed" {
		layer, reason = domain.VisibilityRejected, "application_closed"
	}
	opportunityID := stableProgramOpportunityID(key)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO opportunities(
			id, company_id, normalized_title, normalized_location, opportunity_type,
			visibility_layer, assessment_reason, assessed_at
		) VALUES (?, ?, ?, '', ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET company_id = excluded.company_id,
			normalized_title = excluded.normalized_title, opportunity_type = excluded.opportunity_type,
			visibility_layer = excluded.visibility_layer, assessment_reason = excluded.assessment_reason,
			assessed_at = excluded.assessed_at, status = 'active', updated_at = CURRENT_TIMESTAMP
	`, opportunityID, companyID, opportunity.NormalizeTitle(name), kind, layer, reason, observedAt); err != nil {
		return fmt.Errorf("upsert program opportunity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO opportunity_evidence(
			opportunity_id, program_window_id, source_type, source_url,
			first_observed_at, last_observed_at, freshness_at
		) VALUES (?, ?, 'program_window', ?, ?, ?, ?)
		ON CONFLICT(program_window_id) DO UPDATE SET opportunity_id = excluded.opportunity_id,
			source_url = excluded.source_url, last_observed_at = excluded.last_observed_at,
			freshness_at = excluded.freshness_at
	`, opportunityID, programID, sourceURL, observedAt, observedAt, observedAt); err != nil {
		return fmt.Errorf("persist program evidence: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit program opportunity projection: %w", err)
	}
	return nil
}

func programOpportunityType(value string) domain.OpportunityType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "internship", "staj":
		return domain.OpportunityInternship
	case "long_term_internship", "uzun_donem_staj":
		return domain.OpportunityLongTermInternship
	case "bootcamp":
		return domain.OpportunityBootcamp
	case "hackathon":
		return domain.OpportunityHackathon
	case "competition", "yarisma":
		return domain.OpportunityCompetition
	case "scholarship", "burs":
		return domain.OpportunityScholarship
	case "training", "egitim":
		return domain.OpportunityTraining
	default:
		return domain.OpportunityUniversityCompanyProgram
	}
}

// LoadSourceRecipe returns the one active learned recipe for a source.
func (r *SQLiteRepository) LoadSourceRecipe(ctx context.Context, sourceKey string) (domain.SourceRecipe, bool, error) {
	var recipe domain.SourceRecipe
	err := r.db.QueryRowContext(ctx, `
		SELECT cs.source_key, r.version, r.identity_selector, r.identity_text,
		       r.listing_selector, r.title_selector, r.link_selector,
		       r.golden_listing_count, r.golden_fingerprint
		FROM source_extraction_recipes r
		JOIN company_sources cs ON cs.id = r.source_id
		WHERE cs.source_key = ? AND r.active = 1
	`, strings.TrimSpace(sourceKey)).Scan(
		&recipe.SourceKey, &recipe.Version, &recipe.IdentitySelector, &recipe.IdentityText,
		&recipe.ListingSelector, &recipe.TitleSelector, &recipe.LinkSelector,
		&recipe.GoldenListingCount, &recipe.GoldenFingerprint,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SourceRecipe{}, false, nil
	}
	if err != nil {
		return domain.SourceRecipe{}, false, fmt.Errorf("load source recipe %q: %w", sourceKey, err)
	}
	return recipe, true, nil
}

// SaveSourceRecipe atomically retires the active recipe and inserts its next
// immutable version, retaining history for diagnosis and rollback decisions.
func (r *SQLiteRepository) SaveSourceRecipe(ctx context.Context, recipe domain.SourceRecipe) (domain.SourceRecipe, error) {
	if strings.TrimSpace(recipe.SourceKey) == "" {
		return domain.SourceRecipe{}, errors.New("recipe source key is required")
	}
	if recipe.GoldenListingCount < 0 {
		return domain.SourceRecipe{}, errors.New("recipe golden listing count cannot be negative")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("begin source recipe save: %w", err)
	}
	defer tx.Rollback()

	var sourceID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM company_sources WHERE source_key = ?", strings.TrimSpace(recipe.SourceKey)).Scan(&sourceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.SourceRecipe{}, ErrSourceNotFound
		}
		return domain.SourceRecipe{}, fmt.Errorf("find recipe source %q: %w", recipe.SourceKey, err)
	}
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) + 1 FROM source_extraction_recipes WHERE source_id = ?", sourceID).Scan(&version); err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("select next recipe version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE source_extraction_recipes SET active = 0, updated_at = CURRENT_TIMESTAMP WHERE source_id = ? AND active = 1", sourceID); err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("retire active source recipe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO source_extraction_recipes(
			source_id, version, active, identity_selector, identity_text,
			listing_selector, title_selector, link_selector,
			golden_listing_count, golden_fingerprint
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?)
	`, sourceID, version, strings.TrimSpace(recipe.IdentitySelector), strings.TrimSpace(recipe.IdentityText),
		strings.TrimSpace(recipe.ListingSelector), strings.TrimSpace(recipe.TitleSelector), strings.TrimSpace(recipe.LinkSelector),
		recipe.GoldenListingCount, strings.TrimSpace(recipe.GoldenFingerprint)); err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("insert source recipe version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE company_sources SET strategy_version = ?, last_listing_count = ?,
			last_listing_fingerprint = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, version, recipe.GoldenListingCount, strings.TrimSpace(recipe.GoldenFingerprint), sourceID); err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("update source recipe health: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("commit source recipe save: %w", err)
	}
	recipe.SourceKey = strings.TrimSpace(recipe.SourceKey)
	recipe.Version = version
	return recipe, nil
}

func (r *SQLiteRepository) UpdateSourceRecipeSnapshot(ctx context.Context, sourceKey string, version, count int, fingerprint string) error {
	if version < 1 || count < 0 {
		return errors.New("recipe version must be positive and listing count cannot be negative")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin source recipe snapshot update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE source_extraction_recipes
		SET golden_listing_count = ?, golden_fingerprint = ?, updated_at = CURRENT_TIMESTAMP
		WHERE source_id = (SELECT id FROM company_sources WHERE source_key = ?)
		  AND version = ? AND active = 1
	`, count, strings.TrimSpace(fingerprint), strings.TrimSpace(sourceKey), version)
	if err != nil {
		return fmt.Errorf("update source recipe snapshot: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read source recipe snapshot result: %w", err)
	}
	if changed != 1 {
		return ErrSourceNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE company_sources SET strategy_version = ?, last_listing_count = ?,
			last_listing_fingerprint = ?, updated_at = CURRENT_TIMESTAMP
		WHERE source_key = ?
	`, version, count, strings.TrimSpace(fingerprint), strings.TrimSpace(sourceKey)); err != nil {
		return fmt.Errorf("update source health snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source recipe snapshot update: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) LoadExtractionBlocks(ctx context.Context, sourceKey string, hashes []string) (map[string][]domain.RawListing, error) {
	result := make(map[string][]domain.RawListing)
	for _, hash := range hashes {
		var encoded string
		err := r.db.QueryRowContext(ctx, `
			SELECT c.listings_json FROM source_extraction_block_cache c
			JOIN company_sources s ON s.id = c.source_id
			WHERE s.source_key = ? AND c.block_hash = ?
		`, strings.TrimSpace(sourceKey), strings.TrimSpace(hash)).Scan(&encoded)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load extraction block %q: %w", hash, err)
		}
		var listings []domain.RawListing
		if err := json.Unmarshal([]byte(encoded), &listings); err != nil {
			return nil, fmt.Errorf("decode extraction block %q: %w", hash, err)
		}
		result[hash] = listings
	}
	return result, nil
}

func (r *SQLiteRepository) SaveExtractionBlocks(ctx context.Context, sourceKey string, entries map[string][]domain.RawListing) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin extraction block save: %w", err)
	}
	defer tx.Rollback()
	var sourceID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM company_sources WHERE source_key = ?", strings.TrimSpace(sourceKey)).Scan(&sourceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSourceNotFound
		}
		return fmt.Errorf("find extraction cache source: %w", err)
	}
	for hash, listings := range entries {
		encoded, err := json.Marshal(listings)
		if err != nil {
			return fmt.Errorf("encode extraction block %q: %w", hash, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_extraction_block_cache(source_id, block_hash, listings_json)
			VALUES (?, ?, ?)
			ON CONFLICT(source_id, block_hash) DO UPDATE SET
				listings_json = excluded.listings_json, updated_at = CURRENT_TIMESTAMP
		`, sourceID, strings.TrimSpace(hash), string(encoded)); err != nil {
			return fmt.Errorf("upsert extraction block %q: %w", hash, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit extraction block save: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) UpsertRawListing(ctx context.Context, listing domain.RawListing) (string, bool, error) {
	canonicalURL, err := CanonicalURL(listing.URL)
	if err != nil {
		return "", false, fmt.Errorf("canonicalize listing URL: %w", err)
	}
	if strings.TrimSpace(listing.Company) == "" || strings.TrimSpace(listing.SourceID) == "" {
		return "", false, errors.New("listing company and source ID are required")
	}
	if strings.TrimSpace(listing.Title) == "" {
		return "", false, errors.New("listing title is required")
	}
	if listing.FetchedAt.IsZero() {
		listing.FetchedAt = r.now().UTC()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin listing upsert: %w", err)
	}
	defer tx.Rollback()

	var companyID, sourceID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT companies.id, company_sources.id
		FROM company_sources
		JOIN companies ON companies.id = company_sources.company_id
		WHERE company_sources.source_key = ? AND companies.name = ?
	`, listing.SourceID, listing.Company).Scan(&companyID, &sourceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("source %q is not registered for company %q", listing.SourceID, listing.Company)
		}
		return "", false, fmt.Errorf("resolve listing source: %w", err)
	}

	listingID := stableListingID(listing.Company, canonicalURL)
	contentHash := hashText(listing.Title + "\n" + listing.RawText)
	timestamp := listing.FetchedAt.UTC().Format(time.RFC3339Nano)

	var existingID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM listings WHERE company_id = ? AND canonical_url = ?
	`, companyID, canonicalURL).Scan(&existingID)
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return "", false, fmt.Errorf("find existing listing: %w", err)
	}

	if isNew {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO listings(
				id, company_id, source_id, title, canonical_url, raw_text,
				content_hash, first_seen_at, last_seen_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, listingID, companyID, sourceID, listing.Title, canonicalURL, listing.RawText,
			contentHash, timestamp, timestamp); err != nil {
			return "", false, fmt.Errorf("insert listing: %w", err)
		}
		opportunityID := stableOpportunityID(listingID)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO opportunities(id, company_id, normalized_title, normalized_location)
			VALUES (?, ?, ?, '')
		`, opportunityID, companyID, opportunity.NormalizeTitle(listing.Title)); err != nil {
			return "", false, fmt.Errorf("create listing opportunity: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO listing_opportunities(
				listing_id, opportunity_id, match_method, title_score, match_reason
			) VALUES (?, ?, 'new', 1, 'new_listing')
		`, listingID, opportunityID); err != nil {
			return "", false, fmt.Errorf("link new listing opportunity: %w", err)
		}
	} else {
		listingID = existingID
		if _, err := tx.ExecContext(ctx, `
			UPDATE listings SET
				source_id = ?, title = ?, raw_text = ?, content_hash = ?,
				last_seen_at = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, sourceID, listing.Title, listing.RawText, contentHash, timestamp, listingID); err != nil {
			return "", false, fmt.Errorf("update listing: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("commit listing upsert: %w", err)
	}
	return listingID, isNew, nil
}

func (r *SQLiteRepository) SaveAnalysis(ctx context.Context, listingID string, analysis domain.ListingAnalysis) error {
	matchingAreas, err := json.Marshal(analysis.MatchingAreas)
	if err != nil {
		return fmt.Errorf("encode matching areas: %w", err)
	}

	provider := strings.TrimSpace(analysis.Provider)
	if provider == "" {
		provider = "deterministic"
	}
	if analysis.PromptTokens < 0 || analysis.CompletionTokens < 0 || analysis.TotalTokens < 0 || analysis.EstimatedCostUSD < 0 {
		return errors.New("analysis usage values cannot be negative")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin listing analysis save: %w", err)
	}
	defer tx.Rollback()

	var firstProcessedAt sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT first_processed_at FROM listing_analyses WHERE listing_id = ?
	`, listingID).Scan(&firstProcessedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read first analysis state: %w", err)
	}
	firstSuccessfulAnalysis := errors.Is(err, sql.ErrNoRows) || !firstProcessedAt.Valid
	analyzedAt := r.now().UTC()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO listing_analyses(
			listing_id, opportunity_type, is_application_open, is_relevant,
			matching_areas_json, class_year_requirement, gpa_requirement,
			location, work_model, eligibility_status, application_deadline,
			summary, confidence, needs_user_decision, decision_question,
			provider, model, analyzed_at, first_processed_at, processing_status,
			prompt_tokens, completion_tokens, total_tokens, estimated_cost_usd
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, 'processed', ?, ?, ?, ?)
		ON CONFLICT(listing_id) DO UPDATE SET
			opportunity_type = excluded.opportunity_type,
			is_application_open = excluded.is_application_open,
			is_relevant = excluded.is_relevant,
			matching_areas_json = excluded.matching_areas_json,
			class_year_requirement = excluded.class_year_requirement,
			gpa_requirement = excluded.gpa_requirement,
			location = excluded.location,
			work_model = excluded.work_model,
			eligibility_status = excluded.eligibility_status,
			application_deadline = excluded.application_deadline,
			summary = excluded.summary,
			confidence = excluded.confidence,
			needs_user_decision = excluded.needs_user_decision,
			decision_question = excluded.decision_question,
			provider = excluded.provider,
			model = excluded.model,
			analyzed_at = excluded.analyzed_at,
			first_processed_at = COALESCE(listing_analyses.first_processed_at, excluded.first_processed_at),
			processing_status = excluded.processing_status,
			prompt_tokens = excluded.prompt_tokens,
			completion_tokens = excluded.completion_tokens,
			total_tokens = excluded.total_tokens,
			estimated_cost_usd = excluded.estimated_cost_usd,
			last_error = NULL
	`, listingID, analysis.OpportunityType, boolInt(analysis.ApplicationOpen), boolInt(analysis.Relevant),
		string(matchingAreas), analysis.ClassRequirement, analysis.GPARequirement, analysis.Location,
		analysis.WorkModel, analysis.Eligibility, nullableTime(analysis.ApplicationDueAt),
		analysis.Summary, analysis.Confidence, boolInt(analysis.NeedsUserDecision),
		analysis.DecisionQuestion, provider, analysis.Model, analyzedAt.Format(time.RFC3339Nano), analyzedAt.Format(time.RFC3339Nano),
		analysis.PromptTokens, analysis.CompletionTokens, analysis.TotalTokens, analysis.EstimatedCostUSD)
	if err != nil {
		return fmt.Errorf("save listing analysis: %w", err)
	}
	opportunityID, err := r.resolveOpportunity(ctx, tx, listingID, analysis)
	if err != nil {
		return err
	}
	if err := r.persistOpportunityAssessment(ctx, tx, listingID, opportunityID, analysis); err != nil {
		return err
	}
	if err := r.enqueueListingNotification(ctx, tx, listingID, analysis, firstSuccessfulAnalysis); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit listing analysis and notification: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) persistOpportunityAssessment(
	ctx context.Context,
	tx *sql.Tx,
	listingID, opportunityID string,
	analysis domain.ListingAnalysis,
) error {
	assessment, err := normalizedAssessment(analysis.Assessment)
	if err != nil {
		return err
	}
	if analysis.OpportunityType == "" {
		analysis.OpportunityType = domain.OpportunityOther
	}
	if !analysis.OpportunityType.Valid() {
		return fmt.Errorf("invalid opportunity type %q", analysis.OpportunityType)
	}
	assessedAt := r.now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `
		UPDATE opportunities SET opportunity_type = ?, visibility_layer = ?, match_score = ?,
			focus_score = ?, type_score = ?, location_score = ?, eligibility_score = ?,
			requirement_score = ?, assessment_reason = ?, assessed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, analysis.OpportunityType, assessment.Visibility, assessment.Score, assessment.FocusScore,
		assessment.TypeScore, assessment.LocationScore, assessment.EligibilityScore,
		assessment.RequirementScore, assessment.Reason, assessedAt, opportunityID); err != nil {
		return fmt.Errorf("persist opportunity assessment: %w", err)
	}
	var sourceURL, firstSeen, lastSeen string
	if err := tx.QueryRowContext(ctx, `
		SELECT canonical_url, first_seen_at, last_seen_at FROM listings WHERE id = ?
	`, listingID).Scan(&sourceURL, &firstSeen, &lastSeen); err != nil {
		return fmt.Errorf("load listing evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO opportunity_evidence(
			opportunity_id, listing_id, source_type, source_url, first_observed_at, last_observed_at, freshness_at
		) VALUES (?, ?, 'web', ?, ?, ?, ?)
		ON CONFLICT(listing_id) DO UPDATE SET opportunity_id = excluded.opportunity_id,
			source_url = excluded.source_url, last_observed_at = excluded.last_observed_at,
			freshness_at = excluded.freshness_at
	`, opportunityID, listingID, sourceURL, firstSeen, lastSeen, lastSeen); err != nil {
		return fmt.Errorf("persist listing evidence: %w", err)
	}
	return nil
}

func normalizedAssessment(value domain.MatchAssessment) (domain.MatchAssessment, error) {
	if value.Visibility == "" {
		value.Visibility = domain.VisibilityReview
	}
	if value.Reason == "" {
		value.Reason = "not_assessed"
	}
	if !value.Visibility.Valid() || value.Score < 0 || value.Score > 100 ||
		value.FocusScore < 0 || value.FocusScore > 40 || value.TypeScore < 0 || value.TypeScore > 25 ||
		value.LocationScore < 0 || value.LocationScore > 20 || value.EligibilityScore < 0 || value.EligibilityScore > 10 ||
		value.RequirementScore < 0 || value.RequirementScore > 5 {
		return domain.MatchAssessment{}, errors.New("invalid opportunity assessment")
	}
	if value.Score != value.FocusScore+value.TypeScore+value.LocationScore+value.EligibilityScore+value.RequirementScore {
		return domain.MatchAssessment{}, errors.New("opportunity assessment score must equal its components")
	}
	return value, nil
}

type opportunityCandidate struct {
	id       string
	decision opportunity.Decision
}

func (r *SQLiteRepository) resolveOpportunity(
	ctx context.Context,
	tx *sql.Tx,
	listingID string,
	analysis domain.ListingAnalysis,
) (string, error) {
	var currentID, title string
	var companyID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT listing_opportunities.opportunity_id, listings.company_id, listings.title
		FROM listing_opportunities
		JOIN listings ON listings.id = listing_opportunities.listing_id
		WHERE listing_opportunities.listing_id = ?
	`, listingID).Scan(&currentID, &companyID, &title); err != nil {
		return "", fmt.Errorf("load listing opportunity: %w", err)
	}

	var memberCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM listing_opportunities WHERE opportunity_id = ?
	`, currentID).Scan(&memberCount); err != nil {
		return "", fmt.Errorf("count opportunity members: %w", err)
	}

	identity := opportunity.Identity{Title: title, Location: analysis.Location}
	if memberCount > 1 {
		var otherTitle, otherLocation string
		err := tx.QueryRowContext(ctx, `
			SELECT listings.title, COALESCE(listing_analyses.location, '')
			FROM listing_opportunities
			JOIN listings ON listings.id = listing_opportunities.listing_id
			LEFT JOIN listing_analyses ON listing_analyses.listing_id = listings.id
			WHERE listing_opportunities.opportunity_id = ? AND listings.id != ?
			ORDER BY listings.first_seen_at, listings.id
			LIMIT 1
		`, currentID, listingID).Scan(&otherTitle, &otherLocation)
		if err != nil {
			return "", fmt.Errorf("load opportunity comparison member: %w", err)
		}
		decision := opportunity.Evaluate(identity, opportunity.Identity{Title: otherTitle, Location: otherLocation})
		if decision.Outcome == opportunity.Separate {
			return r.splitOpportunity(ctx, tx, listingID, currentID, companyID, identity, decision)
		}
		return currentID, nil
	}

	normalizedTitle := opportunity.NormalizeTitle(title)
	normalizedLocation := opportunity.NormalizeLocation(analysis.Location)
	if _, err := tx.ExecContext(ctx, `
		UPDATE opportunities
		SET normalized_title = ?, normalized_location = ?, status = 'active', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, normalizedTitle, normalizedLocation, currentID); err != nil {
		return "", fmt.Errorf("update opportunity identity: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, normalized_title, normalized_location
		FROM opportunities
		WHERE company_id = ? AND status = 'active' AND id != ?
		ORDER BY created_at, id
	`, companyID, currentID)
	if err != nil {
		return "", fmt.Errorf("query opportunity candidates: %w", err)
	}
	defer rows.Close()

	autoCandidates := make([]opportunityCandidate, 0, 1)
	var ambiguousCandidate *opportunityCandidate
	for rows.Next() {
		var candidateID, candidateTitle, candidateLocation string
		if err := rows.Scan(&candidateID, &candidateTitle, &candidateLocation); err != nil {
			return "", fmt.Errorf("scan opportunity candidate: %w", err)
		}
		decision := opportunity.Evaluate(identity, opportunity.Identity{Title: candidateTitle, Location: candidateLocation})
		candidate := opportunityCandidate{id: candidateID, decision: decision}
		switch decision.Outcome {
		case opportunity.AutoMerge:
			autoCandidates = append(autoCandidates, candidate)
		case opportunity.Ambiguous:
			if ambiguousCandidate == nil || decision.Score > ambiguousCandidate.decision.Score {
				copy := candidate
				ambiguousCandidate = &copy
			}
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("read opportunity candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return "", fmt.Errorf("close opportunity candidates: %w", err)
	}

	if len(autoCandidates) == 1 {
		candidate := autoCandidates[0]
		if err := r.recordOpportunityEvent(ctx, tx, listingID, currentID, candidate.id, opportunity.AutoMerge, candidate.decision.Score, candidate.decision.Reason); err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE listing_opportunities
			SET opportunity_id = ?, match_method = 'auto', title_score = ?, match_reason = ?, updated_at = CURRENT_TIMESTAMP
			WHERE listing_id = ?
		`, candidate.id, candidate.decision.Score, candidate.decision.Reason, listingID); err != nil {
			return "", fmt.Errorf("merge listing opportunity: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE notifications SET opportunity_id = ? WHERE listing_id = ?`, candidate.id, listingID); err != nil {
			return "", fmt.Errorf("move listing notification opportunity: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE opportunities SET status = 'merged', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, currentID); err != nil {
			return "", fmt.Errorf("retire merged opportunity: %w", err)
		}
		return candidate.id, nil
	}

	if len(autoCandidates) > 1 {
		if err := r.recordOpportunityEvent(ctx, tx, listingID, currentID, "", opportunity.Ambiguous, autoCandidates[0].decision.Score, "multiple_auto_candidates"); err != nil {
			return "", err
		}
	} else if ambiguousCandidate != nil {
		if err := r.recordOpportunityEvent(ctx, tx, listingID, currentID, ambiguousCandidate.id, opportunity.Ambiguous, ambiguousCandidate.decision.Score, ambiguousCandidate.decision.Reason); err != nil {
			return "", err
		}
	}
	return currentID, nil
}

func (r *SQLiteRepository) splitOpportunity(
	ctx context.Context,
	tx *sql.Tx,
	listingID, currentID string,
	companyID int64,
	identity opportunity.Identity,
	decision opportunity.Decision,
) (string, error) {
	targetID := stableOpportunityID(listingID)
	if targetID == currentID {
		targetID = stableSplitOpportunityID(listingID, opportunity.NormalizeTitle(identity.Title), opportunity.NormalizeLocation(identity.Location))
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO opportunities(id, company_id, normalized_title, normalized_location, status)
		VALUES (?, ?, ?, ?, 'active')
		ON CONFLICT(id) DO UPDATE SET
			normalized_title = excluded.normalized_title,
			normalized_location = excluded.normalized_location,
			status = 'active', updated_at = CURRENT_TIMESTAMP
	`, targetID, companyID, opportunity.NormalizeTitle(identity.Title), opportunity.NormalizeLocation(identity.Location)); err != nil {
		return "", fmt.Errorf("create split opportunity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE listing_opportunities
		SET opportunity_id = ?, match_method = 'split', title_score = ?, match_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE listing_id = ?
	`, targetID, decision.Score, decision.Reason, listingID); err != nil {
		return "", fmt.Errorf("split listing opportunity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE notifications SET opportunity_id = ? WHERE listing_id = ?`, targetID, listingID); err != nil {
		return "", fmt.Errorf("split listing notifications: %w", err)
	}
	if err := r.recordOpportunityEvent(ctx, tx, listingID, currentID, targetID, opportunity.Split, decision.Score, decision.Reason); err != nil {
		return "", err
	}
	return targetID, nil
}

func (r *SQLiteRepository) recordOpportunityEvent(
	ctx context.Context,
	tx *sql.Tx,
	listingID, fromID, candidateID string,
	outcome opportunity.Outcome,
	score float64,
	reason string,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO opportunity_match_events(
			listing_id, from_opportunity_id, candidate_opportunity_id, outcome, title_score, reason
		) VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?)
	`, listingID, fromID, candidateID, outcome, score, reason)
	if err != nil {
		return fmt.Errorf("record opportunity match event: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) enqueueListingNotification(
	ctx context.Context,
	tx *sql.Tx,
	listingID string,
	analysis domain.ListingAnalysis,
	firstSuccessfulAnalysis bool,
) error {
	var opportunityID, company, title, priorityGroup, trustLevel string
	if err := tx.QueryRowContext(ctx, `
		SELECT listing_opportunities.opportunity_id, companies.name, listings.title,
			companies.priority_group, company_sources.trust_level
		FROM listings
		JOIN companies ON companies.id = listings.company_id
		JOIN company_sources ON company_sources.id = listings.source_id
		JOIN listing_opportunities ON listing_opportunities.listing_id = listings.id
		WHERE listings.id = ?
	`, listingID).Scan(&opportunityID, &company, &title, &priorityGroup, &trustLevel); err != nil {
		return fmt.Errorf("load notification listing: %w", err)
	}
	if trustLevel != "official_company" && trustLevel != "official_ats" && trustLevel != "verified_newsletter" {
		return nil
	}
	notification, eligible := domain.NewListingNotification(
		opportunityID, listingID, company, title, priorityGroup, analysis, firstSuccessfulAnalysis,
	)
	if !eligible {
		return nil
	}
	var alreadyNotified bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM notifications
			WHERE opportunity_id = ? AND event_type = ?
		)
	`, opportunityID, notification.EventType).Scan(&alreadyNotified); err != nil {
		return fmt.Errorf("check opportunity notification history: %w", err)
	}
	if alreadyNotified {
		return nil
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO notifications(listing_id, opportunity_id, event_type, channel, status, dedup_key)
		VALUES (?, ?, ?, 'web_push', 'pending', ?)
		ON CONFLICT(dedup_key) DO NOTHING
	`, listingID, opportunityID, notification.EventType, notification.DedupKey)
	if err != nil {
		return fmt.Errorf("enqueue notification event: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read notification enqueue result: %w", err)
	}
	if inserted == 0 {
		return nil
	}
	notificationID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read notification ID: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notification_payloads(notification_id, title, body, target_url, topic)
		VALUES (?, ?, ?, ?, ?)
	`, notificationID, notification.Title, notification.Body, notification.TargetURL, notification.Topic); err != nil {
		return fmt.Errorf("save notification payload: %w", err)
	}
	deliveries, err := tx.ExecContext(ctx, `
		INSERT INTO notification_deliveries(
			notification_id, subscription_id, subscription_endpoint_hash, status
		)
		SELECT ?, id, endpoint_hash, 'pending'
		FROM push_subscriptions
		WHERE expiration_at IS NULL OR expiration_at > ?
	`, notificationID, r.now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("fan out notification deliveries: %w", err)
	}
	deliveryCount, err := deliveries.RowsAffected()
	if err != nil {
		return fmt.Errorf("read notification delivery count: %w", err)
	}
	if deliveryCount == 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE notifications SET status = 'cancelled' WHERE id = ?
		`, notificationID); err != nil {
			return fmt.Errorf("cancel notification without subscriptions: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) AnalysisRequired(ctx context.Context, listingID string) (bool, error) {
	if strings.TrimSpace(listingID) == "" {
		return false, errors.New("listing ID is required")
	}
	var status string
	err := r.db.QueryRowContext(ctx, `
		SELECT processing_status FROM listing_analyses WHERE listing_id = ?
	`, listingID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read listing analysis state: %w", err)
	}
	return status != "processed", nil
}

func (r *SQLiteRepository) SaveAnalysisFailure(
	ctx context.Context,
	listingID string,
	provider string,
	model string,
	reason string,
) error {
	listingID = strings.TrimSpace(listingID)
	provider = strings.TrimSpace(provider)
	reason = strings.TrimSpace(reason)
	if listingID == "" || provider == "" || reason == "" {
		return errors.New("listing ID, provider and analysis failure reason are required")
	}
	const maxReasonBytes = 500
	if len(reason) > maxReasonBytes {
		reason = reason[:maxReasonBytes]
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO listing_analyses(
			listing_id, matching_areas_json, eligibility_status, needs_user_decision,
			provider, model, processing_status, retry_count, last_error
		) VALUES (?, '[]', 'karar_bekliyor', 1, ?, NULLIF(?, ''), 'pending', 1, ?)
		ON CONFLICT(listing_id) DO UPDATE SET
			eligibility_status = 'karar_bekliyor',
			needs_user_decision = 1,
			provider = excluded.provider,
			model = excluded.model,
			processing_status = 'pending',
			retry_count = listing_analyses.retry_count + 1,
			last_error = excluded.last_error
	`, listingID, provider, model, reason)
	if err != nil {
		return fmt.Errorf("save listing analysis failure: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) PendingAnalyses(ctx context.Context, limit int) ([]PendingAnalysis, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("pending analysis limit must be between 1 and 100")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT listings.id, companies.name, company_sources.source_key, listings.title,
			listings.canonical_url, listings.raw_text, listings.last_seen_at
		FROM listing_analyses
		JOIN listings ON listings.id = listing_analyses.listing_id
		JOIN companies ON companies.id = listings.company_id
		JOIN company_sources ON company_sources.id = listings.source_id
		WHERE listing_analyses.processing_status = 'pending'
		ORDER BY listing_analyses.retry_count, listings.first_seen_at, listings.id
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending analyses: %w", err)
	}
	defer rows.Close()

	result := make([]PendingAnalysis, 0)
	for rows.Next() {
		var pending PendingAnalysis
		var fetchedAt string
		if err := rows.Scan(
			&pending.ListingID, &pending.Listing.Company, &pending.Listing.SourceID,
			&pending.Listing.Title, &pending.Listing.URL, &pending.Listing.RawText, &fetchedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending analysis: %w", err)
		}
		pending.Listing.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
		if err != nil {
			return nil, fmt.Errorf("parse pending listing fetch time: %w", err)
		}
		result = append(result, pending)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending analyses: %w", err)
	}
	return result, nil
}

func (r *SQLiteRepository) StartScanRun(ctx context.Context, trigger string, startedAt time.Time) (int64, error) {
	if trigger != "manual" && trigger != "scheduled" {
		return 0, fmt.Errorf("invalid scan trigger %q", trigger)
	}
	if startedAt.IsZero() {
		startedAt = r.now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO scan_runs(trigger_type, started_at, status)
		VALUES (?, ?, 'running')
	`, trigger, startedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("start scan run: %w", err)
	}
	runID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read scan run ID: %w", err)
	}
	return runID, nil
}

func (r *SQLiteRepository) FinishScanRun(ctx context.Context, runID int64, completion ScanCompletion) error {
	if runID < 1 {
		return errors.New("scan run ID must be positive")
	}
	switch completion.Status {
	case "completed", "partial", "failed":
	default:
		return fmt.Errorf("invalid finished scan status %q", completion.Status)
	}
	if completion.FinishedAt.IsZero() {
		completion.FinishedAt = r.now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE scan_runs SET
			finished_at = ?, status = ?, sources_succeeded = ?, sources_failed = ?,
			new_listings_count = ?, error_summary = NULLIF(?, '')
		WHERE id = ? AND status = 'running'
	`, completion.FinishedAt.UTC().Format(time.RFC3339Nano), completion.Status,
		completion.SourcesSucceeded, completion.SourcesFailed, completion.NewListings,
		completion.ErrorSummary, runID)
	if err != nil {
		return fmt.Errorf("finish scan run: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read finished scan rows: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("running scan %d was not found", runID)
	}
	return nil
}

func (r *SQLiteRepository) RecordSourceSuccess(ctx context.Context, sourceKey string, finishedAt time.Time) error {
	return r.recordSourceResult(ctx, sourceKey, finishedAt, "")
}

func (r *SQLiteRepository) RecordSourceFailure(
	ctx context.Context,
	sourceKey string,
	finishedAt time.Time,
	reason string,
) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("source failure reason is required")
	}
	return r.recordSourceResult(ctx, sourceKey, finishedAt, reason)
}

func (r *SQLiteRepository) recordSourceResult(
	ctx context.Context,
	sourceKey string,
	finishedAt time.Time,
	reason string,
) error {
	if strings.TrimSpace(sourceKey) == "" {
		return errors.New("source key is required")
	}
	if finishedAt.IsZero() {
		finishedAt = r.now().UTC()
	}
	var result sql.Result
	var err error
	if reason == "" {
		result, err = r.db.ExecContext(ctx, `
			UPDATE company_sources
			SET last_success_at = ?, last_error = NULL, updated_at = CURRENT_TIMESTAMP
			WHERE source_key = ?
		`, finishedAt.UTC().Format(time.RFC3339Nano), sourceKey)
	} else {
		result, err = r.db.ExecContext(ctx, `
			UPDATE company_sources
			SET last_error = ?, updated_at = CURRENT_TIMESTAMP
			WHERE source_key = ?
		`, finishedAt.UTC().Format(time.RFC3339Nano)+": "+reason, sourceKey)
	}
	if err != nil {
		return fmt.Errorf("record source %q result: %w", sourceKey, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read source %q result rows: %w", sourceKey, err)
	}
	if updated != 1 {
		return fmt.Errorf("source %q was not found", sourceKey)
	}
	return nil
}

func (r *SQLiteRepository) ReserveSourceAccess(
	ctx context.Context,
	scope string,
	attemptedAt time.Time,
	minimumInterval time.Duration,
) (AccessDecision, error) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		return AccessDecision{}, errors.New("access scope is required")
	}
	if minimumInterval <= 0 {
		return AccessDecision{}, errors.New("minimum access interval must be positive")
	}
	if attemptedAt.IsZero() {
		attemptedAt = r.now().UTC()
	}
	attemptedAt = attemptedAt.UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AccessDecision{}, fmt.Errorf("begin access reservation: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO source_access_states(scope) VALUES (?)
		ON CONFLICT(scope) DO NOTHING
	`, scope); err != nil {
		return AccessDecision{}, fmt.Errorf("initialize access scope %q: %w", scope, err)
	}

	var failureCount int
	var nextAllowedText, blockedUntilText sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT failure_count, next_allowed_at, blocked_until
		FROM source_access_states WHERE scope = ?
	`, scope).Scan(&failureCount, &nextAllowedText, &blockedUntilText); err != nil {
		return AccessDecision{}, fmt.Errorf("load access scope %q: %w", scope, err)
	}

	nextAllowed, err := latestStoredTime(nextAllowedText, blockedUntilText)
	if err != nil {
		return AccessDecision{}, fmt.Errorf("parse access scope %q timing: %w", scope, err)
	}
	if nextAllowed != nil && attemptedAt.Before(*nextAllowed) {
		if err := tx.Commit(); err != nil {
			return AccessDecision{}, fmt.Errorf("commit denied access reservation: %w", err)
		}
		return AccessDecision{
			Allowed: false, RetryAt: nextAllowed, Reason: "domain access cooldown is active",
			FailCount: failureCount,
		}, nil
	}

	nextAttempt := attemptedAt.Add(minimumInterval)
	if _, err := tx.ExecContext(ctx, `
		UPDATE source_access_states
		SET last_attempt_at = ?, next_allowed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE scope = ?
	`, attemptedAt.Format(time.RFC3339Nano), nextAttempt.Format(time.RFC3339Nano), scope); err != nil {
		return AccessDecision{}, fmt.Errorf("reserve access scope %q: %w", scope, err)
	}
	if err := tx.Commit(); err != nil {
		return AccessDecision{}, fmt.Errorf("commit access reservation: %w", err)
	}
	return AccessDecision{Allowed: true, FailCount: failureCount}, nil
}

func (r *SQLiteRepository) RecordSourceAccessFailure(
	ctx context.Context,
	scope string,
	failedAt time.Time,
	failure AccessFailure,
	baseCooldown time.Duration,
	maximumCooldown time.Duration,
) (AccessDecision, error) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" || strings.TrimSpace(failure.Reason) == "" {
		return AccessDecision{}, errors.New("access scope and failure reason are required")
	}
	if baseCooldown <= 0 || maximumCooldown < baseCooldown {
		return AccessDecision{}, errors.New("access cooldown durations are invalid")
	}
	if failedAt.IsZero() {
		failedAt = r.now().UTC()
	}
	failedAt = failedAt.UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AccessDecision{}, fmt.Errorf("begin access failure: %w", err)
	}
	defer tx.Rollback()

	var failureCount int
	var nextAllowedText sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT failure_count, next_allowed_at
		FROM source_access_states WHERE scope = ?
	`, scope).Scan(&failureCount, &nextAllowedText); err != nil {
		return AccessDecision{}, fmt.Errorf("load failed access scope %q: %w", scope, err)
	}
	failureCount++
	cooldown := exponentialCooldown(baseCooldown, maximumCooldown, failureCount)
	blockedUntil := failedAt.Add(cooldown)
	if failure.RetryAfter != nil && failure.RetryAfter.After(blockedUntil) {
		blockedUntil = failure.RetryAfter.UTC()
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE source_access_states SET
			failure_count = ?, blocked_until = ?, last_status_code = NULLIF(?, 0),
			last_server = NULLIF(?, ''), last_cf_ray = NULLIF(?, ''),
			last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE scope = ?
	`, failureCount, blockedUntil.Format(time.RFC3339Nano), failure.StatusCode,
		failure.Server, failure.CFRay, failure.Reason, scope); err != nil {
		return AccessDecision{}, fmt.Errorf("record access failure for %q: %w", scope, err)
	}
	if err := tx.Commit(); err != nil {
		return AccessDecision{}, fmt.Errorf("commit access failure: %w", err)
	}

	retryAt := blockedUntil
	if nextAllowed, err := parseStoredTime(nextAllowedText); err != nil {
		return AccessDecision{}, fmt.Errorf("parse next access for %q: %w", scope, err)
	} else if nextAllowed != nil && nextAllowed.After(retryAt) {
		retryAt = *nextAllowed
	}
	return AccessDecision{
		Allowed: false, RetryAt: &retryAt, Reason: "domain access protection was triggered",
		FailCount: failureCount,
	}, nil
}

func (r *SQLiteRepository) RecordSourceAccessSuccess(
	ctx context.Context,
	scope string,
	succeededAt time.Time,
) error {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		return errors.New("access scope is required")
	}
	if succeededAt.IsZero() {
		succeededAt = r.now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE source_access_states SET
			failure_count = 0, blocked_until = NULL, last_success_at = ?,
			last_status_code = NULL, last_server = NULL, last_cf_ray = NULL,
			last_error = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE scope = ?
	`, succeededAt.UTC().Format(time.RFC3339Nano), scope)
	if err != nil {
		return fmt.Errorf("record access success for %q: %w", scope, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read access success rows for %q: %w", scope, err)
	}
	if updated != 1 {
		return fmt.Errorf("access scope %q was not found", scope)
	}
	return nil
}

func exponentialCooldown(base, maximum time.Duration, failureCount int) time.Duration {
	cooldown := base
	for attempt := 1; attempt < failureCount && cooldown < maximum; attempt++ {
		if cooldown > maximum/2 {
			return maximum
		}
		cooldown *= 2
	}
	if cooldown > maximum {
		return maximum
	}
	return cooldown
}

func latestStoredTime(values ...sql.NullString) (*time.Time, error) {
	var latest *time.Time
	for _, value := range values {
		parsed, err := parseStoredTime(value)
		if err != nil {
			return nil, err
		}
		if parsed != nil && (latest == nil || parsed.After(*latest)) {
			copy := *parsed
			latest = &copy
		}
	}
	return latest, nil
}

func parseStoredTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (r *SQLiteRepository) Dashboard(ctx context.Context) (DashboardSnapshot, error) {
	newListings, err := r.dashboardListings(ctx, `
		WHERE listing_analyses.is_relevant = 1
			AND listing_analyses.is_application_open = 1
			AND listing_analyses.eligibility_status IN ('uygun', 'kismen_uygun')
	`)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load new listings: %w", err)
	}
	needsDecision, err := r.dashboardListings(ctx, `
		WHERE listing_analyses.eligibility_status = 'karar_bekliyor'
	`)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load decision listings: %w", err)
	}
	activeApplications, err := r.dashboardListings(ctx, `
		WHERE application_tracking.status IN ('incelenecek', 'basvuruldu', 'sinav_mulakat')
	`)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load active applications: %w", err)
	}
	manualChecks, err := r.manualChecks(ctx)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load manual checks: %w", err)
	}
	watchlist, err := r.watchlist(ctx)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load watchlist: %w", err)
	}

	lastScan, err := r.lastScan(ctx)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load last scan: %w", err)
	}

	return DashboardSnapshot{
		NewListings:        newListings,
		NeedsDecision:      needsDecision,
		ActiveApplications: activeApplications,
		ManualChecks:       manualChecks,
		Watchlist:          watchlist,
		LastScan:           lastScan,
	}, nil
}

func (r *SQLiteRepository) Coverage(ctx context.Context) (CoverageReport, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT companies.name, companies.priority_group, companies.tracking_status, companies.tracking_phase,
			company_sources.source_key, company_sources.source_type, company_sources.url,
			company_sources.adapter_type, company_sources.strategy, company_sources.coverage_status,
			company_sources.coverage_reason, company_sources.coverage_reason_code, company_sources.last_verified_at,
			company_sources.trust_level, company_sources.enabled,
			company_sources.last_success_at, COALESCE(company_sources.last_error, '')
		FROM companies JOIN company_sources ON company_sources.company_id = companies.id
		WHERE companies.priority_group IN ('primary', 'secondary')
		ORDER BY companies.priority_group = 'primary' DESC, companies.name, company_sources.source_key
	`)
	if err != nil {
		return CoverageReport{}, fmt.Errorf("query source coverage: %w", err)
	}
	defer rows.Close()
	report := CoverageReport{
		PrioritySummaries: map[string]CoverageSummary{"primary": {}, "secondary": {}},
		SectionSummaries:  map[string]CoverageSummary{"primary": {}, "secondary": {}, "phase_16_5": {}},
		Companies:         make([]CompanyCoverage, 0), Programs: make([]ProgramCoverage, 0),
	}
	companyIndex := make(map[string]int)
	for rows.Next() {
		var company CompanyCoverage
		var source CoverageSource
		var enabled int
		var lastSuccess, lastVerified sql.NullString
		if err := rows.Scan(&company.Name, &company.Priority, &company.TrackingStatus, &company.TrackingPhase,
			&source.SourceID, &source.Type, &source.URL, &source.Adapter, &source.Strategy,
			&source.Status, &source.Reason, &source.ReasonCode, &lastVerified, &source.TrustLevel, &enabled, &lastSuccess, &source.LastError); err != nil {
			return CoverageReport{}, fmt.Errorf("scan source coverage: %w", err)
		}
		source.Enabled = enabled == 1
		source.LastSuccessAt, err = parseStoredTime(lastSuccess)
		if err != nil {
			return CoverageReport{}, fmt.Errorf("parse coverage success time: %w", err)
		}
		source.LastVerifiedAt, err = parseStoredTime(lastVerified)
		if err != nil {
			return CoverageReport{}, fmt.Errorf("parse coverage verification time: %w", err)
		}
		section := coverageSection(company)
		index, found := companyIndex[company.Name]
		if !found {
			index = len(report.Companies)
			companyIndex[company.Name] = index
			company.Sources = make([]CoverageSource, 0)
			report.Companies = append(report.Companies, company)
			report.Summary.TotalCompanies++
			prioritySummary := report.PrioritySummaries[company.Priority]
			prioritySummary.TotalCompanies++
			report.PrioritySummaries[company.Priority] = prioritySummary
			sectionSummary := report.SectionSummaries[section]
			sectionSummary.TotalCompanies++
			report.SectionSummaries[section] = sectionSummary
		}
		report.Companies[index].Sources = append(report.Companies[index].Sources, source)
		addCoverageSource(&report.Summary, source.Status)
		prioritySummary := report.PrioritySummaries[company.Priority]
		addCoverageSource(&prioritySummary, source.Status)
		report.PrioritySummaries[company.Priority] = prioritySummary
		sectionSummary := report.SectionSummaries[section]
		addCoverageSource(&sectionSummary, source.Status)
		report.SectionSummaries[section] = sectionSummary
	}
	if err := rows.Err(); err != nil {
		return CoverageReport{}, fmt.Errorf("read source coverage: %w", err)
	}
	finalizeCoverageSummary(&report.Summary)
	for priority, summary := range report.PrioritySummaries {
		finalizeCoverageSummary(&summary)
		report.PrioritySummaries[priority] = summary
	}
	for section, summary := range report.SectionSummaries {
		finalizeCoverageSummary(&summary)
		report.SectionSummaries[section] = summary
	}

	programRows, err := r.db.QueryContext(ctx, `
		SELECT program_windows.program_key, companies.name, program_windows.name,
			program_windows.program_type, program_windows.url, program_windows.status,
			program_windows.opens_at, program_windows.closes_at, program_windows.last_verified_at
		FROM program_windows JOIN companies ON companies.id = program_windows.company_id
		WHERE companies.priority_group IN ('primary', 'secondary')
		ORDER BY companies.priority_group = 'primary' DESC, companies.name, program_windows.program_key
	`)
	if err != nil {
		return CoverageReport{}, fmt.Errorf("query program coverage: %w", err)
	}
	defer programRows.Close()
	for programRows.Next() {
		var program ProgramCoverage
		var opensAt, closesAt, verifiedAt sql.NullString
		if err := programRows.Scan(&program.ProgramID, &program.Company, &program.Name, &program.Type,
			&program.URL, &program.Status, &opensAt, &closesAt, &verifiedAt); err != nil {
			return CoverageReport{}, fmt.Errorf("scan program coverage: %w", err)
		}
		if program.OpensAt, err = parseStoredTime(opensAt); err != nil {
			return CoverageReport{}, err
		}
		if program.ClosesAt, err = parseStoredTime(closesAt); err != nil {
			return CoverageReport{}, err
		}
		if program.LastVerifiedAt, err = parseStoredTime(verifiedAt); err != nil {
			return CoverageReport{}, err
		}
		report.Programs = append(report.Programs, program)
	}
	if err := programRows.Err(); err != nil {
		return CoverageReport{}, fmt.Errorf("read program coverage: %w", err)
	}
	return report, nil
}

func addCoverageSource(summary *CoverageSummary, status string) {
	summary.TotalSources++
	switch status {
	case "automatic":
		summary.AutomaticSources++
	case "feed":
		summary.FeedSources++
	case "manual":
		summary.ManualSources++
	case "researching":
		summary.ResearchingSources++
	case "broken":
		summary.BrokenSources++
	}
}

func finalizeCoverageSummary(summary *CoverageSummary) {
	summary.AutomaticEligibleSources = summary.AutomaticSources + summary.FeedSources + summary.ResearchingSources + summary.BrokenSources
	if summary.AutomaticEligibleSources > 0 {
		summary.AutomaticCoveragePercent = float64(summary.AutomaticSources+summary.FeedSources) * 100 / float64(summary.AutomaticEligibleSources)
	}
}

func coverageSection(company CompanyCoverage) string {
	if company.TrackingPhase == "16.5" {
		return "phase_16_5"
	}
	return company.Priority
}

// manualChecks surfaces sources the scraper attempted and failed on. It
// deliberately excludes tracking_status = 'manual' companies (see
// watchlist), so a source never appears in both: it either was never
// automated by design, or it broke while being automated.
func (r *SQLiteRepository) manualChecks(ctx context.Context) ([]ManualCheck, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT company_sources.source_key, companies.name, company_sources.url,
			COALESCE(company_sources.last_error, 'Bu kaynak manuel takip ediliyor.'),
			company_sources.last_success_at
		FROM company_sources
		JOIN companies ON companies.id = company_sources.company_id
		WHERE company_sources.last_error IS NOT NULL AND companies.tracking_status != 'manual'
		ORDER BY companies.priority_group = 'primary' DESC, companies.name, company_sources.source_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	checks := make([]ManualCheck, 0)
	for rows.Next() {
		var check ManualCheck
		var lastSuccess sql.NullString
		if err := rows.Scan(&check.SourceID, &check.Company, &check.URL, &check.Reason, &lastSuccess); err != nil {
			return nil, err
		}
		check.LastSuccessAt, err = parseStoredTime(lastSuccess)
		if err != nil {
			return nil, fmt.Errorf("parse manual check success time: %w", err)
		}
		checks = append(checks, check)
	}
	return checks, rows.Err()
}

// watchlist returns companies the user has deliberately chosen to track by
// hand, regardless of scrape/error state (see manualChecks for the opposite).
func (r *SQLiteRepository) watchlist(ctx context.Context) ([]WatchlistEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT company_sources.source_key, companies.name, company_sources.url,
			company_sources.access_mode, company_sources.last_manual_check_at
		FROM company_sources
		JOIN companies ON companies.id = company_sources.company_id
		WHERE companies.tracking_status = 'manual'
		ORDER BY companies.priority_group = 'primary' DESC, companies.name, company_sources.source_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]WatchlistEntry, 0)
	for rows.Next() {
		var entry WatchlistEntry
		var lastChecked sql.NullString
		if err := rows.Scan(&entry.SourceID, &entry.Company, &entry.URL, &entry.AccessMode, &lastChecked); err != nil {
			return nil, err
		}
		switch entry.AccessMode {
		case "manual_only":
			entry.Reason = "Uyum politikası nedeniyle otomatik erişim kapalı."
		default:
			entry.Reason = "Bu kaynak elle takip ediliyor."
		}
		entry.LastCheckedAt, err = parseStoredTime(lastChecked)
		if err != nil {
			return nil, fmt.Errorf("parse watchlist last checked time: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// MarkSourceChecked records that the user manually checked a watchlist
// source right now. It is not restricted to tracking_status = 'manual'
// sources at the storage layer; the dashboard only surfaces the timestamp
// for watchlist entries.
func (r *SQLiteRepository) MarkSourceChecked(ctx context.Context, sourceKey string, checkedAt time.Time) error {
	sourceKey = strings.TrimSpace(sourceKey)
	if sourceKey == "" {
		return ErrSourceNotFound
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE company_sources SET last_manual_check_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE source_key = ?
	`, checkedAt.UTC().Format(time.RFC3339Nano), sourceKey)
	if err != nil {
		return fmt.Errorf("mark source checked: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark source checked: %w", err)
	}
	if affected == 0 {
		return ErrSourceNotFound
	}
	return nil
}

func (r *SQLiteRepository) lastScan(ctx context.Context) (*ScanSummary, error) {
	var summary ScanSummary
	var finishedAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, finished_at, status, sources_succeeded, sources_failed,
			new_listings_count, COALESCE(error_summary, '')
		FROM scan_runs
		WHERE finished_at IS NOT NULL
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&summary.ID, &finishedAt, &summary.Status, &summary.SourcesSucceeded,
		&summary.SourcesFailed, &summary.NewListings, &summary.ErrorSummary)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	summary.FinishedAt, err = time.Parse(time.RFC3339Nano, finishedAt)
	if err != nil {
		return nil, fmt.Errorf("parse finished_at: %w", err)
	}
	return &summary, nil
}

func (r *SQLiteRepository) dashboardListings(ctx context.Context, clause string) ([]DashboardListing, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, opportunity_id, company, title, canonical_url, priority_group,
			eligibility_status, summary, application_deadline, application_status,
			tracking_deadline, interview_at, lifecycle_status, visibility_layer, match_score, assessment_reason
		FROM (
			SELECT listings.id AS id,
				listing_opportunities.opportunity_id AS opportunity_id,
				companies.name AS company, listings.title AS title,
				listings.canonical_url AS canonical_url,
				companies.priority_group AS priority_group,
				COALESCE(listing_analyses.eligibility_status, '') AS eligibility_status,
				COALESCE(listing_analyses.summary, '') AS summary,
				listing_analyses.application_deadline AS application_deadline,
				COALESCE(application_tracking.status, '') AS application_status,
				application_tracking.deadline AS tracking_deadline,
				application_tracking.interview_at AS interview_at,
				opportunities.lifecycle_status AS lifecycle_status,
				opportunities.visibility_layer AS visibility_layer,
				opportunities.match_score AS match_score,
				opportunities.assessment_reason AS assessment_reason,
				listings.first_seen_at AS first_seen_at,
				ROW_NUMBER() OVER (
					PARTITION BY listing_opportunities.opportunity_id
					ORDER BY listings.first_seen_at, listings.id
				) AS opportunity_rank
			FROM listings
			JOIN companies ON companies.id = listings.company_id
			JOIN listing_opportunities ON listing_opportunities.listing_id = listings.id
			JOIN opportunities ON opportunities.id = listing_opportunities.opportunity_id
			LEFT JOIN listing_analyses ON listing_analyses.listing_id = listings.id
			LEFT JOIN application_tracking ON application_tracking.listing_id = listings.id
	`+clause+`
		)
		WHERE opportunity_rank = 1
		ORDER BY first_seen_at DESC, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]DashboardListing, 0)
	for rows.Next() {
		var listing DashboardListing
		var applicationDue, trackingDeadline, interviewAt sql.NullString
		if err := rows.Scan(
			&listing.ID,
			&listing.OpportunityID,
			&listing.Company,
			&listing.Title,
			&listing.URL,
			&listing.Priority,
			&listing.Eligibility,
			&listing.Summary,
			&applicationDue,
			&listing.ApplicationStatus,
			&trackingDeadline,
			&interviewAt,
			&listing.Lifecycle,
			&listing.Visibility,
			&listing.MatchScore,
			&listing.AssessmentReason,
		); err != nil {
			return nil, err
		}
		listing.ApplicationDueAt, err = parseStoredTime(applicationDue)
		if err != nil {
			return nil, fmt.Errorf("parse listing application deadline: %w", err)
		}
		listing.TrackingDeadline, err = parseStoredTime(trackingDeadline)
		if err != nil {
			return nil, fmt.Errorf("parse tracking deadline: %w", err)
		}
		listing.InterviewAt, err = parseStoredTime(interviewAt)
		if err != nil {
			return nil, fmt.Errorf("parse interview time: %w", err)
		}
		listings = append(listings, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return listings, nil
}

func (r *SQLiteRepository) OpportunityHistory(ctx context.Context, query OpportunityHistoryQuery) (OpportunityHistoryPage, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	if query.Lifecycle != "" && !query.Lifecycle.Valid() {
		return OpportunityHistoryPage{}, fmt.Errorf("invalid opportunity lifecycle %q", query.Lifecycle)
	}
	if query.Visibility != "" && !query.Visibility.Valid() {
		return OpportunityHistoryPage{}, fmt.Errorf("invalid opportunity visibility %q", query.Visibility)
	}
	company := strings.ToLower(strings.TrimSpace(query.Company))
	search := strings.ToLower(strings.TrimSpace(query.Query))
	lifecycle := string(query.Lifecycle)
	visibility := string(query.Visibility)
	where := `WHERE opportunities.status = 'active'
		AND (? = '' OR opportunities.lifecycle_status = ?)
		AND (? = '' OR opportunities.visibility_layer = ?)
		AND (? = '' OR INSTR(LOWER(companies.name), ?) > 0)
		AND (? = '' OR INSTR(LOWER(listings.title), ?) > 0 OR INSTR(LOWER(COALESCE(listing_analyses.summary, '')), ?) > 0)`
	args := []any{lifecycle, lifecycle, visibility, visibility, company, company, search, search, search}

	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT opportunities.id)
		FROM opportunities
		JOIN listing_opportunities ON listing_opportunities.opportunity_id = opportunities.id
		JOIN listings ON listings.id = listing_opportunities.listing_id
		JOIN companies ON companies.id = listings.company_id
		LEFT JOIN listing_analyses ON listing_analyses.listing_id = listings.id
		`+where, args...).Scan(&total); err != nil {
		return OpportunityHistoryPage{}, fmt.Errorf("count opportunity history: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, opportunity_id, company, title, canonical_url, priority_group,
			eligibility_status, summary, application_deadline, application_status,
			tracking_deadline, interview_at, lifecycle_status, visibility_layer, match_score, assessment_reason
		FROM (
			SELECT listings.id, opportunities.id AS opportunity_id, companies.name AS company,
				listings.title, listings.canonical_url, companies.priority_group,
				COALESCE(listing_analyses.eligibility_status, '') AS eligibility_status,
				COALESCE(listing_analyses.summary, '') AS summary,
				listing_analyses.application_deadline,
				COALESCE(application_tracking.status, '') AS application_status,
				application_tracking.deadline AS tracking_deadline,
				application_tracking.interview_at,
				opportunities.lifecycle_status,
				opportunities.visibility_layer,
				opportunities.match_score,
				opportunities.assessment_reason,
				listings.last_seen_at,
				ROW_NUMBER() OVER (PARTITION BY opportunities.id ORDER BY listings.last_seen_at DESC, listings.id) AS opportunity_rank
			FROM opportunities
			JOIN listing_opportunities ON listing_opportunities.opportunity_id = opportunities.id
			JOIN listings ON listings.id = listing_opportunities.listing_id
			JOIN companies ON companies.id = listings.company_id
			LEFT JOIN listing_analyses ON listing_analyses.listing_id = listings.id
			LEFT JOIN application_tracking ON application_tracking.listing_id = listings.id
			`+where+`
		)
		WHERE opportunity_rank = 1
		ORDER BY last_seen_at DESC, opportunity_id
		LIMIT ? OFFSET ?
	`, append(args, query.PageSize, (query.Page-1)*query.PageSize)...)
	if err != nil {
		return OpportunityHistoryPage{}, fmt.Errorf("query opportunity history: %w", err)
	}
	defer rows.Close()
	items := make([]DashboardListing, 0)
	for rows.Next() {
		var item DashboardListing
		var applicationDue, trackingDeadline, interviewAt sql.NullString
		if err := rows.Scan(&item.ID, &item.OpportunityID, &item.Company, &item.Title, &item.URL,
			&item.Priority, &item.Eligibility, &item.Summary, &applicationDue, &item.ApplicationStatus,
			&trackingDeadline, &interviewAt, &item.Lifecycle, &item.Visibility, &item.MatchScore, &item.AssessmentReason); err != nil {
			return OpportunityHistoryPage{}, err
		}
		if item.ApplicationDueAt, err = parseStoredTime(applicationDue); err != nil {
			return OpportunityHistoryPage{}, err
		}
		if item.TrackingDeadline, err = parseStoredTime(trackingDeadline); err != nil {
			return OpportunityHistoryPage{}, err
		}
		if item.InterviewAt, err = parseStoredTime(interviewAt); err != nil {
			return OpportunityHistoryPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return OpportunityHistoryPage{}, err
	}
	return OpportunityHistoryPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *SQLiteRepository) UpdateOpportunityLifecycle(ctx context.Context, opportunityID string, lifecycle domain.OpportunityLifecycle) error {
	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" {
		return errors.New("opportunity ID is required")
	}
	if !lifecycle.Valid() {
		return fmt.Errorf("invalid opportunity lifecycle %q", lifecycle)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE opportunities SET lifecycle_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'active'
	`, lifecycle, opportunityID)
	if err != nil {
		return fmt.Errorf("update opportunity lifecycle: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read opportunity lifecycle update: %w", err)
	}
	if updated == 0 {
		return ErrOpportunityNotFound
	}
	return nil
}

func (r *SQLiteRepository) ListingDetail(ctx context.Context, listingID string) (ListingDetail, error) {
	listingID = strings.TrimSpace(listingID)
	if listingID == "" {
		return ListingDetail{}, errors.New("listing ID is required")
	}

	var detail ListingDetail
	var matchingAreas string
	var applicationDue, firstSeen, lastSeen sql.NullString
	var trackingStatus, trackingDeadline, interviewAt, notes sql.NullString
	var applicationOpen, relevant, needsDecision int
	err := r.db.QueryRowContext(ctx, `
		SELECT listings.id, listing_opportunities.opportunity_id,
			companies.name, listings.title, listings.canonical_url,
			companies.priority_group, COALESCE(listing_analyses.eligibility_status, ''),
			COALESCE(listing_analyses.summary, ''), listing_analyses.application_deadline,
			COALESCE(listing_analyses.opportunity_type, ''),
			COALESCE(listing_analyses.is_application_open, 0),
			COALESCE(listing_analyses.is_relevant, 0),
			COALESCE(listing_analyses.matching_areas_json, '[]'),
			listing_analyses.class_year_requirement, listing_analyses.gpa_requirement,
			COALESCE(listing_analyses.location, ''), COALESCE(listing_analyses.work_model, ''),
			COALESCE(listing_analyses.confidence, 0),
			COALESCE(listing_analyses.needs_user_decision, 0),
			COALESCE(listing_analyses.decision_question, ''),
			listings.first_seen_at, listings.last_seen_at,
			application_tracking.status, application_tracking.deadline,
			application_tracking.interview_at, application_tracking.notes,
			opportunities.lifecycle_status
		FROM listings
		JOIN companies ON companies.id = listings.company_id
		JOIN listing_opportunities ON listing_opportunities.listing_id = listings.id
		JOIN opportunities ON opportunities.id = listing_opportunities.opportunity_id
		LEFT JOIN listing_analyses ON listing_analyses.listing_id = listings.id
		LEFT JOIN application_tracking ON application_tracking.listing_id = listings.id
		WHERE listings.id = ?
	`, listingID).Scan(
		&detail.ID, &detail.OpportunityID, &detail.Company, &detail.Title, &detail.URL, &detail.Priority,
		&detail.Eligibility, &detail.Summary, &applicationDue, &detail.OpportunityType,
		&applicationOpen, &relevant, &matchingAreas, &detail.ClassRequirement,
		&detail.GPARequirement, &detail.Location, &detail.WorkModel, &detail.Confidence,
		&needsDecision, &detail.DecisionQuestion, &firstSeen, &lastSeen, &trackingStatus,
		&trackingDeadline, &interviewAt, &notes, &detail.Lifecycle,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ListingDetail{}, ErrListingNotFound
	}
	if err != nil {
		return ListingDetail{}, fmt.Errorf("load listing detail: %w", err)
	}
	if err := json.Unmarshal([]byte(matchingAreas), &detail.MatchingAreas); err != nil {
		return ListingDetail{}, fmt.Errorf("decode listing matching areas: %w", err)
	}
	detail.ApplicationOpen = applicationOpen == 1
	detail.Relevant = relevant == 1
	detail.NeedsUserDecision = needsDecision == 1
	if detail.ApplicationDueAt, err = parseStoredTime(applicationDue); err != nil {
		return ListingDetail{}, fmt.Errorf("parse application deadline: %w", err)
	}
	if detail.FirstSeenAt, err = requiredStoredTime(firstSeen); err != nil {
		return ListingDetail{}, fmt.Errorf("parse first seen time: %w", err)
	}
	if detail.LastSeenAt, err = requiredStoredTime(lastSeen); err != nil {
		return ListingDetail{}, fmt.Errorf("parse last seen time: %w", err)
	}
	if trackingStatus.Valid {
		tracking := &ApplicationTracking{Status: domain.ApplicationStatus(trackingStatus.String), Notes: notes.String}
		if tracking.Deadline, err = parseStoredTime(trackingDeadline); err != nil {
			return ListingDetail{}, fmt.Errorf("parse tracking deadline: %w", err)
		}
		if tracking.InterviewAt, err = parseStoredTime(interviewAt); err != nil {
			return ListingDetail{}, fmt.Errorf("parse interview time: %w", err)
		}
		detail.Application = tracking
		detail.ApplicationStatus = tracking.Status
		detail.TrackingDeadline = tracking.Deadline
		detail.InterviewAt = tracking.InterviewAt
	}
	return detail, nil
}

func (r *SQLiteRepository) SaveApplication(ctx context.Context, listingID string, tracking ApplicationTracking) error {
	listingID = strings.TrimSpace(listingID)
	if listingID == "" {
		return errors.New("listing ID is required")
	}
	switch tracking.Status {
	case domain.ApplicationToReview, domain.ApplicationSubmitted, domain.ApplicationInterview,
		domain.ApplicationPositive, domain.ApplicationNegative:
	default:
		return fmt.Errorf("invalid application status %q", tracking.Status)
	}
	tracking.Notes = strings.TrimSpace(tracking.Notes)
	if len(tracking.Notes) > 2000 {
		return errors.New("application notes cannot exceed 2000 characters")
	}
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT 1 FROM listings WHERE id = ?", listingID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return ErrListingNotFound
	} else if err != nil {
		return fmt.Errorf("find tracked listing: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO application_tracking(listing_id, status, deadline, interview_at, notes)
		VALUES (?, ?, ?, ?, NULLIF(?, ''))
		ON CONFLICT(listing_id) DO UPDATE SET
			status = excluded.status, deadline = excluded.deadline,
			interview_at = excluded.interview_at, notes = excluded.notes,
			updated_at = CURRENT_TIMESTAMP
	`, listingID, tracking.Status, nullableTime(tracking.Deadline), nullableTime(tracking.InterviewAt), tracking.Notes)
	if err != nil {
		return fmt.Errorf("save application tracking: %w", err)
	}
	if updated, err := result.RowsAffected(); err != nil || updated != 1 {
		return fmt.Errorf("save application tracking affected %d rows: %w", updated, err)
	}
	return nil
}

func (r *SQLiteRepository) UpsertPushSubscription(
	ctx context.Context,
	subscription PushSubscriptionInput,
) (bool, error) {
	subscription.Endpoint = strings.TrimSpace(subscription.Endpoint)
	subscription.P256DH = strings.TrimSpace(subscription.P256DH)
	subscription.Auth = strings.TrimSpace(subscription.Auth)
	if subscription.Endpoint == "" || subscription.P256DH == "" || subscription.Auth == "" {
		return false, errors.New("push endpoint and keys are required")
	}
	hash := endpointHash(subscription.Endpoint)
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM push_subscriptions WHERE endpoint_hash = ?)
	`, hash).Scan(&exists); err != nil {
		return false, fmt.Errorf("find push subscription: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions(endpoint, endpoint_hash, p256dh, auth, expiration_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(endpoint_hash) DO UPDATE SET
			endpoint = excluded.endpoint,
			p256dh = excluded.p256dh,
			auth = excluded.auth,
			expiration_at = excluded.expiration_at,
			failure_count = 0,
			last_failure_at = NULL,
			last_status_code = NULL,
			updated_at = CURRENT_TIMESTAMP
	`, subscription.Endpoint, hash, subscription.P256DH, subscription.Auth, nullableTime(subscription.ExpirationAt)); err != nil {
		return false, fmt.Errorf("save push subscription: %w", err)
	}
	return !exists, nil
}

func (r *SQLiteRepository) DeletePushSubscription(ctx context.Context, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return errors.New("push endpoint is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin push subscription deletion: %w", err)
	}
	defer tx.Rollback()
	var subscriptionID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM push_subscriptions WHERE endpoint_hash = ?
	`, endpointHash(endpoint)).Scan(&subscriptionID)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("find deleted push subscription: %w", err)
	}
	if err := r.disablePushSubscriptionTx(ctx, tx, subscriptionID, r.now().UTC(), 0); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit push subscription deletion: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ClaimPushDeliveries(
	ctx context.Context,
	limit int,
	now time.Time,
	lease time.Duration,
) ([]PushDelivery, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("push delivery limit must be between 1 and 100")
	}
	if lease <= 0 {
		return nil, errors.New("push delivery lease must be positive")
	}
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin push delivery claim: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE notification_deliveries
		SET status = 'pending', lease_until = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE status = 'sending' AND lease_until <= ?
	`, now.Format(time.RFC3339Nano)); err != nil {
		return nil, fmt.Errorf("release expired push delivery leases: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT notification_deliveries.id, notification_deliveries.notification_id,
			notification_deliveries.attempt_count,
			push_subscriptions.id, push_subscriptions.endpoint,
			push_subscriptions.endpoint_hash, push_subscriptions.p256dh,
			push_subscriptions.auth, push_subscriptions.expiration_at,
			notification_payloads.title, notification_payloads.body,
			notification_payloads.target_url, notification_payloads.topic
		FROM notification_deliveries
		JOIN notifications ON notifications.id = notification_deliveries.notification_id
		JOIN notification_payloads ON notification_payloads.notification_id = notifications.id
		JOIN push_subscriptions ON push_subscriptions.id = notification_deliveries.subscription_id
		WHERE notification_deliveries.status = 'pending'
			AND (notification_deliveries.next_attempt_at IS NULL OR notification_deliveries.next_attempt_at <= ?)
		ORDER BY notification_deliveries.id
		LIMIT ?
	`, now.Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("query due push deliveries: %w", err)
	}
	deliveries := make([]PushDelivery, 0)
	for rows.Next() {
		var delivery PushDelivery
		var expiration sql.NullString
		if err := rows.Scan(
			&delivery.ID, &delivery.NotificationID, &delivery.AttemptCount,
			&delivery.Subscription.ID, &delivery.Subscription.Endpoint,
			&delivery.Subscription.EndpointHash, &delivery.Subscription.P256DH,
			&delivery.Subscription.Auth, &expiration,
			&delivery.Title, &delivery.Body, &delivery.TargetURL, &delivery.Topic,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan due push delivery: %w", err)
		}
		delivery.Subscription.ExpirationAt, err = parseStoredTime(expiration)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("parse push subscription expiration: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close push delivery rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read due push deliveries: %w", err)
	}

	claimed := deliveries[:0]
	leaseUntil := now.Add(lease).Format(time.RFC3339Nano)
	for _, delivery := range deliveries {
		result, err := tx.ExecContext(ctx, `
			UPDATE notification_deliveries
			SET status = 'sending', attempt_count = attempt_count + 1,
				lease_until = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND status = 'pending'
		`, leaseUntil, delivery.ID)
		if err != nil {
			return nil, fmt.Errorf("claim push delivery: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("read push delivery claim result: %w", err)
		}
		if count == 1 {
			delivery.AttemptCount++
			claimed = append(claimed, delivery)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit push delivery claims: %w", err)
	}
	return claimed, nil
}

func (r *SQLiteRepository) MarkPushDeliverySent(
	ctx context.Context,
	deliveryID int64,
	sentAt time.Time,
	statusCode int,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin push delivery completion: %w", err)
	}
	defer tx.Rollback()
	var subscriptionID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT subscription_id FROM notification_deliveries WHERE id = ?
	`, deliveryID).Scan(&subscriptionID); err != nil {
		return fmt.Errorf("find completed push delivery: %w", err)
	}
	timestamp := sentAt.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `
		UPDATE notification_deliveries
		SET status = 'sent', sent_at = ?, lease_until = NULL,
			last_status_code = ?, last_error = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'sending'
	`, timestamp, statusCode, deliveryID); err != nil {
		return fmt.Errorf("complete push delivery: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET failure_count = 0, last_success_at = ?, last_failure_at = NULL,
			last_status_code = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, timestamp, statusCode, subscriptionID); err != nil {
		return fmt.Errorf("record push subscription success: %w", err)
	}
	if err := r.updateNotificationStatusTx(ctx, tx, deliveryID, sentAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit push delivery completion: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) RetryPushDelivery(
	ctx context.Context,
	deliveryID int64,
	nextAttemptAt time.Time,
	statusCode int,
	reason string,
) error {
	reason = shortStoreError(reason)
	result, err := r.db.ExecContext(ctx, `
		UPDATE notification_deliveries
		SET status = 'pending', next_attempt_at = ?, lease_until = NULL,
			last_status_code = NULLIF(?, 0), last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'sending'
	`, nextAttemptAt.UTC().Format(time.RFC3339Nano), statusCode, reason, deliveryID)
	if err != nil {
		return fmt.Errorf("schedule push delivery retry: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return fmt.Errorf("push delivery %d is not sending", deliveryID)
	}
	return nil
}

func (r *SQLiteRepository) FailPushDelivery(
	ctx context.Context,
	deliveryID int64,
	failedAt time.Time,
	statusCode int,
	reason string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin push delivery failure: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE notification_deliveries
		SET status = 'failed', lease_until = NULL, last_status_code = NULLIF(?, 0),
			last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'sending'
	`, statusCode, shortStoreError(reason), deliveryID); err != nil {
		return fmt.Errorf("fail push delivery: %w", err)
	}
	if err := r.updateNotificationStatusTx(ctx, tx, deliveryID, failedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit push delivery failure: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) DisablePushSubscription(
	ctx context.Context,
	deliveryID int64,
	disabledAt time.Time,
	statusCode int,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin invalid push subscription cleanup: %w", err)
	}
	defer tx.Rollback()
	var subscriptionID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT subscription_id FROM notification_deliveries WHERE id = ?
	`, deliveryID).Scan(&subscriptionID); err != nil {
		return fmt.Errorf("find invalid push subscription: %w", err)
	}
	if err := r.disablePushSubscriptionTx(ctx, tx, subscriptionID, disabledAt, statusCode); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit invalid push subscription cleanup: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) disablePushSubscriptionTx(
	ctx context.Context,
	tx *sql.Tx,
	subscriptionID int64,
	disabledAt time.Time,
	statusCode int,
) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT notification_id FROM notification_deliveries
		WHERE subscription_id = ? AND status IN ('pending', 'sending')
	`, subscriptionID)
	if err != nil {
		return fmt.Errorf("query invalid subscription deliveries: %w", err)
	}
	notificationIDs := make([]int64, 0)
	for rows.Next() {
		var notificationID int64
		if err := rows.Scan(&notificationID); err != nil {
			rows.Close()
			return fmt.Errorf("scan invalid subscription delivery: %w", err)
		}
		notificationIDs = append(notificationIDs, notificationID)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close invalid subscription deliveries: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE notification_deliveries
		SET status = 'cancelled', lease_until = NULL, last_status_code = NULLIF(?, 0),
			last_error = 'push subscription is no longer valid', updated_at = CURRENT_TIMESTAMP
		WHERE subscription_id = ? AND status IN ('pending', 'sending')
	`, statusCode, subscriptionID); err != nil {
		return fmt.Errorf("cancel invalid subscription deliveries: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM push_subscriptions WHERE id = ?", subscriptionID); err != nil {
		return fmt.Errorf("delete invalid push subscription: %w", err)
	}
	for _, notificationID := range notificationIDs {
		if err := r.updateNotificationByIDTx(ctx, tx, notificationID, disabledAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) updateNotificationStatusTx(
	ctx context.Context,
	tx *sql.Tx,
	deliveryID int64,
	completedAt time.Time,
) error {
	var notificationID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT notification_id FROM notification_deliveries WHERE id = ?
	`, deliveryID).Scan(&notificationID); err != nil {
		return fmt.Errorf("find push delivery notification: %w", err)
	}
	return r.updateNotificationByIDTx(ctx, tx, notificationID, completedAt)
}

func (r *SQLiteRepository) updateNotificationByIDTx(
	ctx context.Context,
	tx *sql.Tx,
	notificationID int64,
	completedAt time.Time,
) error {
	var active, sent int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			SUM(CASE WHEN status IN ('pending', 'sending') THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END)
		FROM notification_deliveries WHERE notification_id = ?
	`, notificationID).Scan(&active, &sent); err != nil {
		return fmt.Errorf("summarize push notification: %w", err)
	}
	if active > 0 {
		return nil
	}
	status := "failed"
	if sent > 0 {
		status = "sent"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE notifications
		SET status = ?, sent_at = CASE WHEN ? = 'sent' THEN COALESCE(sent_at, ?) ELSE sent_at END
		WHERE id = ?
	`, status, status, completedAt.UTC().Format(time.RFC3339Nano), notificationID); err != nil {
		return fmt.Errorf("complete push notification: %w", err)
	}
	return nil
}

func endpointHash(endpoint string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(endpoint)))
	return hex.EncodeToString(sum[:])
}

func shortStoreError(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		return reason[:500]
	}
	return reason
}

func requiredStoredTime(value sql.NullString) (time.Time, error) {
	parsed, err := parseStoredTime(value)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, errors.New("required time is missing")
	}
	return *parsed, nil
}

func CanonicalURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return "", errors.New("listing URL must be an absolute HTTP(S) URL")
	}
	if parsedURL.User != nil {
		return "", errors.New("listing URL must not contain user information")
	}

	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	hostname := strings.ToLower(parsedURL.Hostname())
	port := parsedURL.Port()
	if (parsedURL.Scheme == "https" && port == "443") || (parsedURL.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		parsedURL.Host = net.JoinHostPort(hostname, port)
	} else {
		parsedURL.Host = hostname
	}
	parsedURL.Fragment = ""
	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	} else if parsedURL.Path != "/" {
		parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	}

	query := parsedURL.Query()
	for key := range query {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "utm_") || isTrackingParameter(lowerKey) {
			query.Del(key)
		}
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func isTrackingParameter(key string) bool {
	switch key {
	case "fbclid", "gclid", "mc_cid", "mc_eid":
		return true
	default:
		return false
	}
}

func stableListingID(company string, canonicalURL string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(company) + "\x00" + canonicalURL))
	return hex.EncodeToString(hash[:])
}

func stableOpportunityID(listingID string) string {
	return "opp-" + strings.TrimSpace(listingID)
}

func stableProgramOpportunityID(programKey string) string {
	hash := sha256.Sum256([]byte("program:" + strings.TrimSpace(programKey)))
	return "opp-program-" + hex.EncodeToString(hash[:12])
}

func stableSplitOpportunityID(listingID, normalizedTitle, normalizedLocation string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(listingID) + "\x00" + normalizedTitle + "\x00" + normalizedLocation))
	return "opp-split-" + hex.EncodeToString(hash[:])
}

func hashText(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func durationSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	return int64(value / time.Second)
}

func validateSourceRegistration(source domain.SourceRegistration) error {
	if strings.TrimSpace(source.Key) == "" || strings.TrimSpace(source.Company) == "" {
		return errors.New("source key and company are required")
	}
	switch source.PriorityGroup {
	case "primary", "secondary", "candidate":
	default:
		return fmt.Errorf("invalid source priority group %q", source.PriorityGroup)
	}
	if source.TrackingPhase != "" && source.TrackingPhase != "16.5" {
		return fmt.Errorf("invalid tracking phase %q", source.TrackingPhase)
	}
	if source.CoverageReasonCode != "" {
		switch source.CoverageReasonCode {
		case "account_required", "third_party_restricted", "no_public_job_source", "client_rendered_unverified", "periodic_program", "source_unreachable":
		default:
			return fmt.Errorf("invalid coverage reason code %q", source.CoverageReasonCode)
		}
	}
	if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.Adapter) == "" {
		return errors.New("source type and adapter are required")
	}
	if _, err := CanonicalURL(source.URL); err != nil {
		return fmt.Errorf("invalid source URL: %w", err)
	}
	mode := strings.TrimSpace(source.AccessMode)
	if mode == "" {
		mode = "legacy"
	}
	switch mode {
	case "legacy":
	case "robots", "public_api":
		if strings.TrimSpace(source.AccessScope) == "" || source.MinimumInterval <= 0 ||
			source.BaseCooldown <= 0 || source.MaximumCooldown < source.BaseCooldown {
			return fmt.Errorf("invalid %s source access policy", mode)
		}
	case "manual_only":
		if source.Enabled || source.Adapter != "manual" || source.Strategy != "manual" || source.TrackingStatus != "manual" {
			return errors.New("manual_only source must be disabled and use manual adapter, strategy and tracking")
		}
	default:
		return fmt.Errorf("invalid source access mode %q", mode)
	}
	coverage := effectiveCoverageStatus(source)
	if coverage != "automatic" && coverage != "feed" && coverage != "manual" && coverage != "researching" && coverage != "broken" {
		return fmt.Errorf("invalid source coverage status %q", coverage)
	}
	trust := effectiveTrustLevel(source)
	if trust != "official_company" && trust != "official_ats" && trust != "verified_newsletter" && trust != "aggregator" {
		return fmt.Errorf("invalid source trust level %q", trust)
	}
	return nil
}

func effectiveCoverageStatus(source domain.SourceRegistration) string {
	if status := strings.TrimSpace(source.CoverageStatus); status != "" {
		return status
	}
	if source.Enabled {
		return "automatic"
	}
	if source.Strategy == "manual" {
		return "manual"
	}
	return "researching"
}

func effectiveTrustLevel(source domain.SourceRegistration) string {
	if trust := strings.TrimSpace(source.TrustLevel); trust != "" {
		return trust
	}
	return "aggregator"
}
