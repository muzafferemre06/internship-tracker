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
)

type SQLiteRepository struct {
	db  *sql.DB
	now func() time.Time
}

func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &SQLiteRepository{db: db, now: time.Now}, nil
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

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO companies(name, priority_group)
		VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET
			priority_group = excluded.priority_group,
			updated_at = CURRENT_TIMESTAMP
	`, source.Company, source.PriorityGroup); err != nil {
		return fmt.Errorf("upsert company %q: %w", source.Company, err)
	}

	var companyID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM companies WHERE name = ?", source.Company).Scan(&companyID); err != nil {
		return fmt.Errorf("find company %q: %w", source.Company, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_sources(
			company_id, source_key, source_type, url, adapter_type, enabled
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_key) DO UPDATE SET
			company_id = excluded.company_id,
			source_type = excluded.source_type,
			url = excluded.url,
			adapter_type = excluded.adapter_type,
			enabled = excluded.enabled,
			updated_at = CURRENT_TIMESTAMP
	`, companyID, source.Key, source.Type, source.URL, source.Adapter, boolInt(source.Enabled)); err != nil {
		return fmt.Errorf("upsert source %q: %w", source.Key, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source registration: %w", err)
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

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO listing_analyses(
			listing_id, opportunity_type, is_application_open, is_relevant,
			matching_areas_json, class_year_requirement, gpa_requirement,
			location, work_model, eligibility_status, application_deadline,
			summary, confidence, needs_user_decision, decision_question,
			provider, analyzed_at, processing_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'deterministic', ?, 'processed')
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
			analyzed_at = excluded.analyzed_at,
			processing_status = excluded.processing_status,
			last_error = NULL
	`, listingID, analysis.OpportunityType, boolInt(analysis.ApplicationOpen), boolInt(analysis.Relevant),
		string(matchingAreas), analysis.ClassRequirement, analysis.GPARequirement, analysis.Location,
		analysis.WorkModel, analysis.Eligibility, nullableTime(analysis.ApplicationDueAt),
		analysis.Summary, analysis.Confidence, boolInt(analysis.NeedsUserDecision),
		analysis.DecisionQuestion, r.now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save listing analysis: %w", err)
	}
	return nil
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
		JOIN application_tracking ON application_tracking.listing_id = listings.id
		WHERE application_tracking.status IN ('incelenecek', 'basvuruldu', 'sinav_mulakat')
	`)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load active applications: %w", err)
	}

	lastScan, err := r.lastScan(ctx)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load last scan: %w", err)
	}

	return DashboardSnapshot{
		NewListings:        newListings,
		NeedsDecision:      needsDecision,
		ActiveApplications: activeApplications,
		LastScan:           lastScan,
	}, nil
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
		SELECT listings.id, companies.name, listings.title, listings.canonical_url,
			companies.priority_group, COALESCE(listing_analyses.eligibility_status, '')
		FROM listings
		JOIN companies ON companies.id = listings.company_id
		LEFT JOIN listing_analyses ON listing_analyses.listing_id = listings.id
	`+clause+`
		ORDER BY listings.first_seen_at DESC, listings.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]DashboardListing, 0)
	for rows.Next() {
		var listing DashboardListing
		if err := rows.Scan(
			&listing.ID,
			&listing.Company,
			&listing.Title,
			&listing.URL,
			&listing.Priority,
			&listing.Eligibility,
		); err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return listings, nil
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

func validateSourceRegistration(source domain.SourceRegistration) error {
	if strings.TrimSpace(source.Key) == "" || strings.TrimSpace(source.Company) == "" {
		return errors.New("source key and company are required")
	}
	switch source.PriorityGroup {
	case "primary", "secondary", "candidate":
	default:
		return fmt.Errorf("invalid source priority group %q", source.PriorityGroup)
	}
	if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.Adapter) == "" {
		return errors.New("source type and adapter are required")
	}
	if _, err := CanonicalURL(source.URL); err != nil {
		return fmt.Errorf("invalid source URL: %w", err)
	}
	return nil
}
