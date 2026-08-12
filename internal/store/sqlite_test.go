package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
)

func TestSQLiteRepositoryDeduplicatesCanonicalURL(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)

	firstSeen := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	listing := domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Yazılım Stajyeri",
		URL:     "HTTPS://WWW.KARIYER.NET:443/is-ilani/staj-123/?utm_source=test&ref=profile#details",
		RawText: "İlk içerik", FetchedAt: firstSeen,
	}

	listingID, isNew, err := repository.UpsertRawListing(context.Background(), listing)
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}
	if !isNew || listingID == "" {
		t.Fatalf("expected a new listing, got id=%q new=%v", listingID, isNew)
	}

	listing.URL = "https://www.kariyer.net/is-ilani/staj-123?ref=profile&gclid=tracking"
	listing.RawText = "Güncellenmiş içerik"
	listing.FetchedAt = firstSeen.Add(time.Hour)
	secondID, isNew, err := repository.UpsertRawListing(context.Background(), listing)
	if err != nil {
		t.Fatalf("update listing: %v", err)
	}
	if isNew || secondID != listingID {
		t.Fatalf("expected duplicate id %q, got id=%q new=%v", listingID, secondID, isNew)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM listings").Scan(&count); err != nil {
		t.Fatalf("count listings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one listing, got %d", count)
	}

	var rawText, lastSeen string
	if err := db.QueryRow("SELECT raw_text, last_seen_at FROM listings WHERE id = ?", listingID).Scan(&rawText, &lastSeen); err != nil {
		t.Fatalf("read listing: %v", err)
	}
	if rawText != "Güncellenmiş içerik" || lastSeen != listing.FetchedAt.Format(time.RFC3339Nano) {
		t.Fatalf("listing was not refreshed: text=%q last_seen=%q", rawText, lastSeen)
	}
}

func TestSQLiteRepositoryPersistsPhase19AssessmentOnCanonicalOpportunity(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://example.test/jobs/phase-19", RawText: "Ankara backend staj", FetchedAt: time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert listing: %v", err)
	}
	if err := repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		OpportunityType: domain.OpportunityInternship, ApplicationOpen: true, Relevant: true,
		Eligibility: domain.EligibilitySuitable, Confidence: 0.91,
		Assessment: domain.MatchAssessment{Score: 100, FocusScore: 40, TypeScore: 25, LocationScore: 20, EligibilityScore: 10, RequirementScore: 5, Visibility: domain.VisibilityNotification, PushEligible: true, Reason: "strong_match"},
	}); err != nil {
		t.Fatalf("save analysis: %v", err)
	}
	var kind, layer, reason string
	var score, evidenceCount int
	if err := db.QueryRow(`SELECT opportunity_type, visibility_layer, assessment_reason, match_score FROM opportunities WHERE id = (SELECT opportunity_id FROM listing_opportunities WHERE listing_id = ?)`, listingID).Scan(&kind, &layer, &reason, &score); err != nil {
		t.Fatalf("read assessment: %v", err)
	}
	if kind != string(domain.OpportunityInternship) || layer != string(domain.VisibilityNotification) || reason != "strong_match" || score != 100 {
		t.Fatalf("unexpected canonical assessment: kind=%q layer=%q reason=%q score=%d", kind, layer, reason, score)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM opportunity_evidence WHERE listing_id = ? AND source_type = 'web'`, listingID).Scan(&evidenceCount); err != nil || evidenceCount != 1 {
		t.Fatalf("listing source evidence was not persisted: count=%d err=%v", evidenceCount, err)
	}
}

func TestSQLiteRepositoryRequiresRegisteredSource(t *testing.T) {
	repository, _ := newTestRepository(t)

	_, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "missing", Title: "Staj",
		URL: "https://example.test/is-ilani/1", RawText: "Staj",
	})
	if err == nil {
		t.Fatal("expected unregistered source to fail")
	}
}

func TestSQLiteRepositoryVersionsSourceRecipesAndPersistsGoldenSnapshot(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	ctx := context.Background()

	first, err := repository.SaveSourceRecipe(ctx, domain.SourceRecipe{
		SourceKey: "meteksan-kariyer-net", IdentitySelector: "#careers", IdentityText: "Meteksan",
		ListingSelector: ".opening", TitleSelector: ".title", LinkSelector: "a.apply",
		GoldenListingCount: 2, GoldenFingerprint: "first-fingerprint",
	})
	if err != nil {
		t.Fatalf("save first source recipe: %v", err)
	}
	second, err := repository.SaveSourceRecipe(ctx, domain.SourceRecipe{
		SourceKey: "meteksan-kariyer-net", IdentitySelector: ".jobs", IdentityText: "Meteksan",
		ListingSelector: ".position", TitleSelector: "h3", LinkSelector: "a",
		GoldenListingCount: 3, GoldenFingerprint: "second-fingerprint",
	})
	if err != nil {
		t.Fatalf("save repaired source recipe: %v", err)
	}
	if first.Version != 1 || second.Version != 2 {
		t.Fatalf("recipe versions must increase monotonically: first=%d second=%d", first.Version, second.Version)
	}

	loaded, ok, err := repository.LoadSourceRecipe(ctx, "meteksan-kariyer-net")
	if err != nil {
		t.Fatalf("load active source recipe: %v", err)
	}
	if !ok || loaded.Version != 2 || loaded.ListingSelector != ".position" || loaded.GoldenListingCount != 3 {
		t.Fatalf("unexpected active recipe: ok=%v recipe=%#v", ok, loaded)
	}
	if err := repository.UpdateSourceRecipeSnapshot(ctx, loaded.SourceKey, loaded.Version, 4, "updated-fingerprint"); err != nil {
		t.Fatalf("update source recipe snapshot: %v", err)
	}
	loaded, _, err = repository.LoadSourceRecipe(ctx, "meteksan-kariyer-net")
	if err != nil {
		t.Fatalf("reload source recipe: %v", err)
	}
	if loaded.GoldenListingCount != 4 || loaded.GoldenFingerprint != "updated-fingerprint" {
		t.Fatalf("golden snapshot did not persist: %#v", loaded)
	}

	var versions, active int
	if err := db.QueryRow(`SELECT COUNT(*), SUM(active) FROM source_extraction_recipes WHERE source_id = (SELECT id FROM company_sources WHERE source_key = ?)`, "meteksan-kariyer-net").Scan(&versions, &active); err != nil {
		t.Fatalf("inspect recipe history: %v", err)
	}
	if versions != 2 || active != 1 {
		t.Fatalf("recipe history must retain two versions and one active row: versions=%d active=%d", versions, active)
	}
}

func TestSQLiteRepositoryPersistsGenericExtractionBlocksAcrossRestart(t *testing.T) {
	repository, _ := newTestRepository(t)
	registerMeteksan(t, repository)
	ctx := context.Background()
	want := map[string][]domain.RawListing{
		"block-hash": {{Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Staj", URL: "https://example.test/jobs/1", RawText: "Go stajı"}},
	}
	if err := repository.SaveExtractionBlocks(ctx, "meteksan-kariyer-net", want); err != nil {
		t.Fatalf("save extraction block cache: %v", err)
	}
	got, err := repository.LoadExtractionBlocks(ctx, "meteksan-kariyer-net", []string{"block-hash", "missing"})
	if err != nil {
		t.Fatalf("load extraction block cache: %v", err)
	}
	if len(got) != 1 || len(got["block-hash"]) != 1 || got["block-hash"][0].Title != "Backend Staj" {
		t.Fatalf("unexpected persisted block cache: %#v", got)
	}
}

func TestSQLiteRepositorySavesAnalysis(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://example.test/is-ilani/1", RawText: "Backend ve Go",
	})
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}

	err = repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		MatchingAreas: []string{"backend"}, Eligibility: domain.EligibilitySuitable,
		Summary: "Backend odaklı staj ilanı", Confidence: 0.9,
	})
	if err != nil {
		t.Fatalf("save analysis: %v", err)
	}

	var status, provider string
	if err := db.QueryRow("SELECT processing_status, provider FROM listing_analyses WHERE listing_id = ?", listingID).Scan(&status, &provider); err != nil {
		t.Fatalf("read analysis: %v", err)
	}
	if status != "processed" || provider != "deterministic" {
		t.Fatalf("unexpected analysis state: status=%q provider=%q", status, provider)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	if len(dashboard.NewListings) != 1 || dashboard.NewListings[0].ID != listingID {
		t.Fatalf("analysis was not exposed on dashboard: %#v", dashboard)
	}
}

func TestSQLiteRepositoryPersistsAnalysisUsageAndRecoversPendingFailure(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://example.test/is-ilani/pending", RawText: "Backend ve Go",
	})
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}

	if err := repository.SaveAnalysisFailure(context.Background(), listingID, "openrouter", "test/model", "rate limited"); err != nil {
		t.Fatalf("save failed analysis: %v", err)
	}
	required, err := repository.AnalysisRequired(context.Background(), listingID)
	if err != nil || !required {
		t.Fatalf("pending analysis must be retryable: required=%v err=%v", required, err)
	}
	dashboard, err := repository.Dashboard(context.Background())
	if err != nil || len(dashboard.NeedsDecision) != 1 || dashboard.NeedsDecision[0].ID != listingID {
		t.Fatalf("failed listing was not preserved for decision: dashboard=%#v err=%v", dashboard, err)
	}

	if err := repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		MatchingAreas: []string{"backend"}, Location: "Ankara", WorkModel: "hibrit",
		Eligibility: domain.EligibilitySuitable, Summary: "Backend stajı", Confidence: 0.9,
		Provider: "openrouter", Model: "test/model", PromptTokens: 100,
		CompletionTokens: 25, TotalTokens: 125, EstimatedCostUSD: 0.00015,
	}); err != nil {
		t.Fatalf("save recovered analysis: %v", err)
	}
	required, err = repository.AnalysisRequired(context.Background(), listingID)
	if err != nil || required {
		t.Fatalf("processed analysis should not be retried: required=%v err=%v", required, err)
	}
	var status, provider, model string
	var retryCount, promptTokens, completionTokens, totalTokens int
	var estimatedCost float64
	var lastError sql.NullString
	if err := db.QueryRow(`
		SELECT processing_status, provider, model, retry_count, last_error,
			prompt_tokens, completion_tokens, total_tokens, estimated_cost_usd
		FROM listing_analyses WHERE listing_id = ?
	`, listingID).Scan(&status, &provider, &model, &retryCount, &lastError,
		&promptTokens, &completionTokens, &totalTokens, &estimatedCost); err != nil {
		t.Fatalf("read recovered analysis: %v", err)
	}
	if status != "processed" || provider != "openrouter" || model != "test/model" || retryCount != 1 || lastError.Valid {
		t.Fatalf("unexpected recovered state: status=%q provider=%q model=%q retries=%d error=%#v", status, provider, model, retryCount, lastError)
	}
	if promptTokens != 100 || completionTokens != 25 || totalTokens != 125 || estimatedCost != 0.00015 {
		t.Fatalf("usage was not persisted: prompt=%d completion=%d total=%d cost=%f", promptTokens, completionTokens, totalTokens, estimatedCost)
	}
}

func TestSQLiteRepositorySavesAnalysisAndNotificationOutboxAtomically(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	if _, err := repository.UpsertPushSubscription(context.Background(), PushSubscriptionInput{
		Endpoint: "https://push.example.test/device-one", P256DH: "test-public", Auth: "test-auth",
	}); err != nil {
		t.Fatalf("save subscription: %v", err)
	}
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://example.test/push/one", RawText: "Backend stajı",
	})
	if err != nil {
		t.Fatal(err)
	}
	analysis := domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		Eligibility: domain.EligibilitySuitable, Summary: "Uygun staj",
	}
	if err := repository.SaveAnalysis(context.Background(), listingID, analysis); err != nil {
		t.Fatalf("save analysis and outbox: %v", err)
	}
	if err := repository.SaveAnalysis(context.Background(), listingID, analysis); err != nil {
		t.Fatalf("repeat analysis save: %v", err)
	}
	var notifications, deliveries int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE listing_id = ?", listingID).Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM notification_deliveries
		JOIN notifications ON notifications.id = notification_deliveries.notification_id
		WHERE notifications.listing_id = ?
	`, listingID).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 || deliveries != 1 {
		t.Fatalf("expected one deduplicated event and delivery, got events=%d deliveries=%d", notifications, deliveries)
	}

	if _, err := db.Exec(`
		CREATE TRIGGER reject_push_event BEFORE INSERT ON notifications
		BEGIN SELECT RAISE(ABORT, 'outbox unavailable'); END
	`); err != nil {
		t.Fatal(err)
	}
	rollbackID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Rollback Stajı",
		URL: "https://example.test/push/rollback", RawText: "Backend stajı",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveAnalysis(context.Background(), rollbackID, analysis); err == nil {
		t.Fatal("expected notification failure to roll back analysis")
	}
	var analyses int
	if err := db.QueryRow("SELECT COUNT(*) FROM listing_analyses WHERE listing_id = ?", rollbackID).Scan(&analyses); err != nil {
		t.Fatal(err)
	}
	if analyses != 0 {
		t.Fatalf("analysis escaped failed outbox transaction: count=%d", analyses)
	}
}

func TestSQLiteRepositoryNotificationEligibilityAndFailedAnalysisRecovery(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "secondary-source", Company: "Secondary", PriorityGroup: "secondary",
		Type: "career_page", URL: "https://example.test/secondary", Adapter: "lever", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpsertPushSubscription(context.Background(), PushSubscriptionInput{
		Endpoint: "https://push.example.test/device", P256DH: "test-public", Auth: "test-auth",
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(company, source, suffix string) string {
		t.Helper()
		id, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
			Company: company, SourceID: source, Title: "Staj", URL: "https://example.test/" + suffix, RawText: "Staj",
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	partialID := insert("Meteksan Savunma", "meteksan-kariyer-net", "partial")
	secondaryID := insert("Secondary", "secondary-source", "secondary")
	recoveredID := insert("Meteksan Savunma", "meteksan-kariyer-net", "recovered")
	if err := repository.SaveAnalysis(context.Background(), partialID, domain.ListingAnalysis{
		ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilityPartlySuitable,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveAnalysis(context.Background(), secondaryID, domain.ListingAnalysis{
		ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilitySuitable,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveAnalysisFailure(context.Background(), recoveredID, "test", "", "temporary"); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveAnalysis(context.Background(), recoveredID, domain.ListingAnalysis{
		ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilitySuitable,
	}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("only recovered primary suitable listing should notify, got %d events", count)
	}
}

func TestSQLiteRepositoryDisablesOnlyGoneDeliverySubscription(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	for _, endpoint := range []string{"https://push.example.test/one", "https://push.example.test/two"} {
		if _, err := repository.UpsertPushSubscription(context.Background(), PushSubscriptionInput{
			Endpoint: endpoint, P256DH: "test-public", Auth: "test-auth",
		}); err != nil {
			t.Fatal(err)
		}
	}
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Staj",
		URL: "https://example.test/gone", RawText: "Staj",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilitySuitable,
	}); err != nil {
		t.Fatal(err)
	}
	deliveries, err := repository.ClaimPushDeliveries(context.Background(), 10, time.Now().UTC(), time.Minute)
	if err != nil || len(deliveries) != 2 {
		t.Fatalf("claim two devices: deliveries=%d err=%v", len(deliveries), err)
	}
	disabled := deliveries[0]
	if err := repository.DisablePushSubscription(context.Background(), disabled.ID, time.Now().UTC(), 410); err != nil {
		t.Fatalf("disable gone subscription: %v", err)
	}
	var subscriptions, cancelled, sending int
	if err := db.QueryRow("SELECT COUNT(*) FROM push_subscriptions").Scan(&subscriptions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notification_deliveries WHERE status = 'cancelled'").Scan(&cancelled); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notification_deliveries WHERE status = 'sending'").Scan(&sending); err != nil {
		t.Fatal(err)
	}
	if subscriptions != 1 || cancelled != 1 || sending != 1 {
		t.Fatalf("wrong device was disabled: subscriptions=%d cancelled=%d sending=%d", subscriptions, cancelled, sending)
	}
}

func TestSQLiteRepositoryPersistsScanReportAndSourceState(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	startedAt := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)

	runID, err := repository.StartScanRun(context.Background(), "manual", startedAt)
	if err != nil {
		t.Fatalf("start scan: %v", err)
	}
	if err := repository.RecordSourceFailure(
		context.Background(), "meteksan-kariyer-net", finishedAt, "unexpected HTTP status 429",
	); err != nil {
		t.Fatalf("record source failure: %v", err)
	}
	if err := repository.FinishScanRun(context.Background(), runID, ScanCompletion{
		FinishedAt: finishedAt, Status: "failed", SourcesFailed: 1,
		ErrorSummary: `[{"source":"meteksan-kariyer-net","error":"unexpected HTTP status 429"}]`,
	}); err != nil {
		t.Fatalf("finish scan: %v", err)
	}

	var lastError string
	if err := db.QueryRow(
		"SELECT last_error FROM company_sources WHERE source_key = ?", "meteksan-kariyer-net",
	).Scan(&lastError); err != nil {
		t.Fatalf("read source state: %v", err)
	}
	if !strings.Contains(lastError, finishedAt.Format(time.RFC3339Nano)) || !strings.Contains(lastError, "429") {
		t.Fatalf("source error lacks time or reason: %q", lastError)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	if dashboard.LastScan == nil || dashboard.LastScan.ID != runID || dashboard.LastScan.Status != "failed" || dashboard.LastScan.SourcesFailed != 1 {
		t.Fatalf("unexpected last scan: %#v", dashboard.LastScan)
	}
	if len(dashboard.ManualChecks) != 1 || dashboard.ManualChecks[0].SourceID != "meteksan-kariyer-net" ||
		!strings.Contains(dashboard.ManualChecks[0].Reason, "429") {
		t.Fatalf("failed source was not exposed for manual checking: %#v", dashboard.ManualChecks)
	}

	if err := repository.RecordSourceSuccess(
		context.Background(), "meteksan-kariyer-net", finishedAt.Add(time.Minute),
	); err != nil {
		t.Fatalf("record source recovery: %v", err)
	}
	var clearedError sql.NullString
	if err := db.QueryRow(
		"SELECT last_error FROM company_sources WHERE source_key = ?", "meteksan-kariyer-net",
	).Scan(&clearedError); err != nil {
		t.Fatalf("read recovered source: %v", err)
	}
	if clearedError.Valid {
		t.Fatalf("expected source error to clear after success, got %q", clearedError.String)
	}
}

func TestSQLiteRepositoryWatchlistIsSeparateFromManualChecks(t *testing.T) {
	repository, _ := newTestRepository(t)
	registerMeteksan(t, repository)
	if err := repository.RecordSourceFailure(
		context.Background(), "meteksan-kariyer-net", time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC), "unexpected HTTP status 429",
	); err != nil {
		t.Fatalf("record source failure: %v", err)
	}
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "akdogan-tech-career", Company: "Akdoğan Tech", PriorityGroup: "primary",
		Type: "career_page", URL: "https://akdogan.tech/career", Adapter: "manual", Strategy: "manual",
		TrackingStatus: "manual", Enabled: false,
	}); err != nil {
		t.Fatalf("register manual source: %v", err)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}

	if len(dashboard.ManualChecks) != 1 || dashboard.ManualChecks[0].SourceID != "meteksan-kariyer-net" {
		t.Fatalf("expected only the failing source in manual checks, got %#v", dashboard.ManualChecks)
	}
	if len(dashboard.Watchlist) != 1 || dashboard.Watchlist[0].SourceID != "akdogan-tech-career" ||
		dashboard.Watchlist[0].Company != "Akdoğan Tech" || dashboard.Watchlist[0].LastCheckedAt != nil {
		t.Fatalf("expected only the manual company in the watchlist, got %#v", dashboard.Watchlist)
	}

	checkedAt := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	if err := repository.MarkSourceChecked(context.Background(), "akdogan-tech-career", checkedAt); err != nil {
		t.Fatalf("mark source checked: %v", err)
	}
	dashboard, err = repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("reload dashboard: %v", err)
	}
	if dashboard.Watchlist[0].LastCheckedAt == nil || !dashboard.Watchlist[0].LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected watchlist entry to record checked time, got %#v", dashboard.Watchlist[0].LastCheckedAt)
	}

	if err := repository.MarkSourceChecked(context.Background(), "does-not-exist", checkedAt); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound for unknown source, got %v", err)
	}
}

func TestSQLiteRepositoryPersistsManualOnlyAccessPolicyOnWatchlist(t *testing.T) {
	repository, db := newTestRepository(t)
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "havelsan-linkedin", Company: "Havelsan", PriorityGroup: "primary",
		Type: "social_profile", URL: "https://www.linkedin.com/company/havelsan/jobs/",
		Adapter: "manual", Strategy: "manual", TrackingStatus: "manual", Enabled: false,
		AccessMode: "manual_only", AccessScope: "linkedin.com",
	}); err != nil {
		t.Fatalf("register manual-only social source: %v", err)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Watchlist) != 1 || dashboard.Watchlist[0].AccessMode != "manual_only" ||
		!strings.Contains(dashboard.Watchlist[0].Reason, "otomatik erişim") {
		t.Fatalf("manual-only reason is not visible on watchlist: %#v", dashboard.Watchlist)
	}

	var mode, scope string
	var minimum, base, maximum int
	if err := db.QueryRow(`
		SELECT access_mode, access_scope, minimum_interval_seconds,
			base_cooldown_seconds, maximum_cooldown_seconds
		FROM company_sources WHERE source_key = ?
	`, "havelsan-linkedin").Scan(&mode, &scope, &minimum, &base, &maximum); err != nil {
		t.Fatal(err)
	}
	if mode != "manual_only" || scope != "linkedin.com" || minimum != 0 || base != 0 || maximum != 0 {
		t.Fatalf("unexpected persisted access policy: mode=%q scope=%q min=%d base=%d max=%d", mode, scope, minimum, base, maximum)
	}
}

func TestSQLiteRepositoryManagesApplicationTrackingAndListingDetail(t *testing.T) {
	repository, _ := newTestRepository(t)
	registerMeteksan(t, repository)
	fetchedAt := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://example.test/is-ilani/tracking", RawText: "Go ve backend", FetchedAt: fetchedAt,
	})
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}
	applicationDue := fetchedAt.Add(7 * 24 * time.Hour)
	if err := repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		MatchingAreas: []string{"backend", "go"}, ClassRequirement: intPointer(3),
		GPARequirement: floatPointer(2.5), Location: "Ankara", WorkModel: "hibrit",
		Eligibility: domain.EligibilitySuitable, ApplicationDueAt: &applicationDue,
		Summary: "Backend ekibi için staj.", Confidence: 0.92,
	}); err != nil {
		t.Fatalf("save analysis: %v", err)
	}
	manualDeadline := fetchedAt.Add(5 * 24 * time.Hour)
	interviewAt := fetchedAt.Add(10 * 24 * time.Hour)
	if err := repository.SaveApplication(context.Background(), listingID, ApplicationTracking{
		Status: domain.ApplicationSubmitted, Deadline: &manualDeadline,
		InterviewAt: &interviewAt, Notes: "İK dönüşü bekleniyor.",
	}); err != nil {
		t.Fatalf("save application: %v", err)
	}

	detail, err := repository.ListingDetail(context.Background(), listingID)
	if err != nil {
		t.Fatalf("load detail: %v", err)
	}
	if detail.Summary != "Backend ekibi için staj." || len(detail.MatchingAreas) != 2 ||
		detail.Application == nil || detail.Application.Status != domain.ApplicationSubmitted ||
		detail.Application.Deadline == nil || !detail.Application.Deadline.Equal(manualDeadline) {
		t.Fatalf("unexpected listing detail: %#v", detail)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil || len(dashboard.ActiveApplications) != 1 ||
		dashboard.ActiveApplications[0].ApplicationStatus != domain.ApplicationSubmitted {
		t.Fatalf("application was not exposed on dashboard: dashboard=%#v err=%v", dashboard, err)
	}
}

func TestSQLiteRepositoryRejectsInvalidApplicationTracking(t *testing.T) {
	repository, _ := newTestRepository(t)
	if err := repository.SaveApplication(context.Background(), "missing", ApplicationTracking{
		Status: domain.ApplicationStatus("unknown"),
	}); err == nil || !strings.Contains(err.Error(), "invalid application status") {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if _, err := repository.ListingDetail(context.Background(), "missing"); err != ErrListingNotFound {
		t.Fatalf("expected listing not found, got %v", err)
	}
}

func TestSQLiteRepositoryListsAllOpportunityHistoryWithLifecycleFiltersAndPagination(t *testing.T) {
	repository, _ := newTestRepository(t)
	registerMeteksan(t, repository)
	ctx := context.Background()
	fetchedAt := time.Date(2026, 8, 9, 9, 0, 0, 0, time.UTC)

	insert := func(title, suffix string, analysis *domain.ListingAnalysis) (string, string) {
		t.Helper()
		listingID, _, err := repository.UpsertRawListing(ctx, domain.RawListing{
			Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: title,
			URL: "https://example.test/history/" + suffix, RawText: title, FetchedAt: fetchedAt,
		})
		if err != nil {
			t.Fatal(err)
		}
		if analysis != nil {
			if err := repository.SaveAnalysis(ctx, listingID, *analysis); err != nil {
				t.Fatal(err)
			}
		}
		detail, err := repository.ListingDetail(ctx, listingID)
		if err != nil {
			t.Fatal(err)
		}
		return listingID, detail.OpportunityID
	}

	_, firstOpportunity := insert("Uygun Staj", "suitable", &domain.ListingAnalysis{
		ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilitySuitable,
	})
	insert("Uygun Olmayan Rol", "unsuitable", &domain.ListingAnalysis{
		ApplicationOpen: false, Relevant: false, Eligibility: domain.EligibilityUnsuitable,
	})
	insert("Henüz Analiz Edilmedi", "pending", nil)

	firstPage, err := repository.OpportunityHistory(ctx, OpportunityHistoryQuery{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if firstPage.Total != 3 || firstPage.Page != 1 || firstPage.PageSize != 2 || len(firstPage.Items) != 2 {
		t.Fatalf("all opportunities must remain discoverable with stable pagination: %#v", firstPage)
	}

	if err := repository.UpdateOpportunityLifecycle(ctx, firstOpportunity, domain.OpportunityArchived); err != nil {
		t.Fatalf("archive opportunity: %v", err)
	}
	archived, err := repository.OpportunityHistory(ctx, OpportunityHistoryQuery{
		Page: 1, PageSize: 20, Lifecycle: domain.OpportunityArchived, Company: "meteksan", Query: "uygun staj",
	})
	if err != nil {
		t.Fatal(err)
	}
	if archived.Total != 1 || len(archived.Items) != 1 || archived.Items[0].OpportunityID != firstOpportunity ||
		archived.Items[0].Lifecycle != domain.OpportunityArchived {
		t.Fatalf("archived opportunity must remain visible through filters: %#v", archived)
	}

	if err := repository.UpdateOpportunityLifecycle(ctx, firstOpportunity, domain.OpportunityLifecycle("deleted")); err == nil {
		t.Fatal("unsupported lifecycle must be rejected")
	}
	if err := repository.UpdateOpportunityLifecycle(ctx, "missing", domain.OpportunityOpen); !errors.Is(err, ErrOpportunityNotFound) {
		t.Fatalf("missing opportunity must return ErrOpportunityNotFound, got %v", err)
	}
}

func intPointer(value int) *int { return &value }

func floatPointer(value float64) *float64 { return &value }

func TestSQLiteRepositoryPersistsDomainAccessReservationAndCooldown(t *testing.T) {
	repository, db := newTestRepository(t)
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)

	decision, err := repository.ReserveSourceAccess(context.Background(), "KARIYER.NET", now, 24*time.Hour)
	if err != nil {
		t.Fatalf("reserve first access: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected first access to be allowed: %#v", decision)
	}

	decision, err = repository.ReserveSourceAccess(context.Background(), "kariyer.net", now.Add(time.Hour), 24*time.Hour)
	if err != nil {
		t.Fatalf("reserve repeated access: %v", err)
	}
	if decision.Allowed || decision.RetryAt == nil || !decision.RetryAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("expected daily access budget to deny retry: %#v", decision)
	}

	retryAfter := now.Add(30 * time.Hour)
	decision, err = repository.RecordSourceAccessFailure(
		context.Background(), "kariyer.net", now.Add(2*time.Hour), AccessFailure{
			StatusCode: 429, RetryAfter: &retryAfter, Server: "cloudflare",
			CFRay: "test-ray-IST", Reason: "unexpected HTTP status 429",
		}, 6*time.Hour, 24*time.Hour,
	)
	if err != nil {
		t.Fatalf("record access failure: %v", err)
	}
	if decision.FailCount != 1 || decision.RetryAt == nil || !decision.RetryAt.Equal(retryAfter) {
		t.Fatalf("expected Retry-After to extend cooldown: %#v", decision)
	}

	var failures, status int
	var server, cfRay, blockedUntil string
	if err := db.QueryRow(`
		SELECT failure_count, last_status_code, last_server, last_cf_ray, blocked_until
		FROM source_access_states WHERE scope = 'kariyer.net'
	`).Scan(&failures, &status, &server, &cfRay, &blockedUntil); err != nil {
		t.Fatalf("read access state: %v", err)
	}
	if failures != 1 || status != 429 || server != "cloudflare" || cfRay != "test-ray-IST" || blockedUntil != retryAfter.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected persisted access diagnostics: failures=%d status=%d server=%q ray=%q blocked=%q", failures, status, server, cfRay, blockedUntil)
	}

	if err := repository.RecordSourceAccessSuccess(context.Background(), "kariyer.net", retryAfter.Add(time.Minute)); err != nil {
		t.Fatalf("record access recovery: %v", err)
	}
	var clearedStatus sql.NullInt64
	if err := db.QueryRow(`
		SELECT failure_count, last_status_code FROM source_access_states WHERE scope = 'kariyer.net'
	`).Scan(&failures, &clearedStatus); err != nil {
		t.Fatalf("read recovered access state: %v", err)
	}
	if failures != 0 || clearedStatus.Valid {
		t.Fatalf("expected diagnostics to reset after success: failures=%d status=%#v", failures, clearedStatus)
	}
}

func TestExponentialCooldownCapsAtMaximum(t *testing.T) {
	for _, test := range []struct {
		failures int
		want     time.Duration
	}{
		{failures: 1, want: 6 * time.Hour},
		{failures: 2, want: 12 * time.Hour},
		{failures: 3, want: 24 * time.Hour},
		{failures: 4, want: 24 * time.Hour},
	} {
		if got := exponentialCooldown(6*time.Hour, 24*time.Hour, test.failures); got != test.want {
			t.Fatalf("failure %d: expected %s, got %s", test.failures, test.want, got)
		}
	}
}

func TestCanonicalURL(t *testing.T) {
	canonical, err := CanonicalURL("HTTPS://Example.COM:443/jobs/42/?b=2&utm_medium=email&a=1#apply")
	if err != nil {
		t.Fatalf("canonicalize URL: %v", err)
	}
	if canonical != "https://example.com/jobs/42?a=1&b=2" {
		t.Fatalf("unexpected canonical URL %q", canonical)
	}
}

func TestSQLiteRepositoryLinksCrossSourceListingsToOneOpportunity(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksanSources(t, repository)
	ctx := context.Background()

	firstID := insertOpportunityListing(t, repository, "meteksan-kariyer-net", "Yazılım Geliştirme Stajyeri", "https://example.test/kariyer/42")
	secondID := insertOpportunityListing(t, repository, "meteksan-careers", "YAZILIM GELISTIRME STAJI", "https://careers.example.test/jobs/99")
	analysis := domain.ListingAnalysis{Location: "İstanbul, Türkiye", Relevant: true, Eligibility: domain.EligibilitySuitable}
	if err := repository.SaveAnalysis(ctx, firstID, analysis); err != nil {
		t.Fatalf("analyze first listing: %v", err)
	}
	if err := repository.SaveAnalysis(ctx, secondID, analysis); err != nil {
		t.Fatalf("analyze duplicate listing: %v", err)
	}

	var active, distinctLinks, merges int
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunities WHERE status = 'active'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(DISTINCT opportunity_id) FROM listing_opportunities WHERE listing_id IN (?, ?)", firstID, secondID).Scan(&distinctLinks); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunity_match_events WHERE listing_id = ? AND outcome = 'auto_merge'", secondID).Scan(&merges); err != nil {
		t.Fatal(err)
	}
	if active != 1 || distinctLinks != 1 || merges != 1 {
		t.Fatalf("cross-source listings were not canonically linked: active=%d links=%d merges=%d", active, distinctLinks, merges)
	}
}

func TestSQLiteRepositoryKeepsAmbiguousMatchSeparateAndAuditable(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksanSources(t, repository)
	ctx := context.Background()

	firstID := insertOpportunityListing(t, repository, "meteksan-kariyer-net", "Backend Stajyeri", "https://example.test/jobs/one")
	secondID := insertOpportunityListing(t, repository, "meteksan-careers", "Backend Stajı", "https://careers.example.test/jobs/two")
	if err := repository.SaveAnalysis(ctx, firstID, domain.ListingAnalysis{Location: "Ankara", Relevant: true, Eligibility: domain.EligibilitySuitable}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveAnalysis(ctx, secondID, domain.ListingAnalysis{Relevant: true, Eligibility: domain.EligibilitySuitable}); err != nil {
		t.Fatal(err)
	}

	var active, ambiguous int
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunities WHERE status = 'active'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunity_match_events WHERE listing_id = ? AND outcome = 'ambiguous' AND reason = 'location_missing'", secondID).Scan(&ambiguous); err != nil {
		t.Fatal(err)
	}
	if active != 2 || ambiguous != 1 {
		t.Fatalf("ambiguous evidence must remain separate and auditable: active=%d ambiguous=%d", active, ambiguous)
	}
}

func TestSQLiteRepositorySplitsMergedListingWhenEvidenceConflicts(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksanSources(t, repository)
	ctx := context.Background()

	firstID := insertOpportunityListing(t, repository, "meteksan-kariyer-net", "Backend Stajyeri", "https://example.test/jobs/same")
	secondID := insertOpportunityListing(t, repository, "meteksan-careers", "Backend Stajı", "https://careers.example.test/jobs/same")
	for _, listingID := range []string{firstID, secondID} {
		if err := repository.SaveAnalysis(ctx, listingID, domain.ListingAnalysis{Location: "Ankara", Relevant: true, Eligibility: domain.EligibilitySuitable}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.SaveAnalysis(ctx, secondID, domain.ListingAnalysis{Location: "İstanbul", Relevant: true, Eligibility: domain.EligibilitySuitable}); err != nil {
		t.Fatal(err)
	}

	var active, distinctLinks, splits int
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunities WHERE status = 'active'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(DISTINCT opportunity_id) FROM listing_opportunities WHERE listing_id IN (?, ?)", firstID, secondID).Scan(&distinctLinks); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunity_match_events WHERE listing_id = ? AND outcome = 'split' AND reason = 'location_conflict'", secondID).Scan(&splits); err != nil {
		t.Fatal(err)
	}
	if active != 2 || distinctLinks != 2 || splits != 1 {
		t.Fatalf("conflicting evidence did not reverse merge: active=%d links=%d splits=%d", active, distinctLinks, splits)
	}
}

func TestSQLiteRepositoryReconcilesBackfilledAnalyzedListings(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksanSources(t, repository)
	ctx := context.Background()
	firstID := insertOpportunityListing(t, repository, "meteksan-kariyer-net", "Yazılım Stajyeri", "https://example.test/legacy/one")
	secondID := insertOpportunityListing(t, repository, "meteksan-careers", "Yazılım Stajı", "https://careers.example.test/legacy/two")
	analysis := domain.ListingAnalysis{Location: "Ankara", Relevant: true, Eligibility: domain.EligibilitySuitable}
	for _, listingID := range []string{firstID, secondID} {
		if err := repository.SaveAnalysis(ctx, listingID, analysis); err != nil {
			t.Fatal(err)
		}
	}

	secondOpportunityID := stableOpportunityID(secondID)
	if _, err := db.Exec("UPDATE opportunities SET status = 'active', normalized_title = '', normalized_location = '' WHERE id = ?", secondOpportunityID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE listing_opportunities SET opportunity_id = ?, match_method = 'backfill' WHERE listing_id = ?", secondOpportunityID, secondID); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReconcileOpportunities(ctx); err != nil {
		t.Fatalf("reconcile backfilled opportunities: %v", err)
	}

	var distinctLinks int
	if err := db.QueryRow("SELECT COUNT(DISTINCT opportunity_id) FROM listing_opportunities WHERE listing_id IN (?, ?)", firstID, secondID).Scan(&distinctLinks); err != nil {
		t.Fatal(err)
	}
	if distinctLinks != 1 {
		t.Fatalf("expected analyzed backfill to reconcile to one opportunity, got %d", distinctLinks)
	}
}

func TestSQLiteDashboardAndNotificationsDeduplicateCanonicalOpportunity(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksanSources(t, repository)
	ctx := context.Background()
	if _, err := repository.UpsertPushSubscription(ctx, PushSubscriptionInput{
		Endpoint: "https://push.example.test/phase13", P256DH: "test-public", Auth: "test-auth",
	}); err != nil {
		t.Fatal(err)
	}
	firstID := insertOpportunityListing(t, repository, "meteksan-kariyer-net", "Backend Stajyeri", "https://example.test/dashboard/one")
	secondID := insertOpportunityListing(t, repository, "meteksan-careers", "Backend Stajı", "https://careers.example.test/dashboard/two")
	analysis := domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		Location: "Ankara", Eligibility: domain.EligibilitySuitable, Summary: "Uygun backend stajı",
	}
	for _, listingID := range []string{firstID, secondID} {
		if err := repository.SaveAnalysis(ctx, listingID, analysis); err != nil {
			t.Fatal(err)
		}
	}

	dashboard, err := repository.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.NewListings) != 1 || dashboard.NewListings[0].OpportunityID == "" {
		t.Fatalf("dashboard must expose one canonical opportunity: %#v", dashboard.NewListings)
	}
	var notifications, deliveries int
	var dedupKey string
	if err := db.QueryRow("SELECT COUNT(*), MIN(dedup_key) FROM notifications").Scan(&notifications, &dedupKey); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notification_deliveries").Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 || deliveries != 1 || !strings.HasPrefix(dedupKey, "opportunity:") {
		t.Fatalf("expected one opportunity event/delivery, got notifications=%d deliveries=%d key=%q", notifications, deliveries, dedupKey)
	}
}

func TestSQLiteRepositoryNeverMatchesAcrossCompanies(t *testing.T) {
	repository, db := newTestRepository(t)
	ctx := context.Background()
	for _, registration := range []domain.SourceRegistration{
		{Key: "company-a", Company: "Company A", PriorityGroup: "secondary", Type: "fixture", URL: "https://a.example.test", Adapter: "fixture", Enabled: true},
		{Key: "company-b", Company: "Company B", PriorityGroup: "secondary", Type: "fixture", URL: "https://b.example.test", Adapter: "fixture", Enabled: true},
	} {
		if err := repository.RegisterSource(ctx, registration); err != nil {
			t.Fatal(err)
		}
	}
	for _, listing := range []domain.RawListing{
		{Company: "Company A", SourceID: "company-a", Title: "Backend Stajyeri", URL: "https://a.example.test/jobs/1", RawText: "Backend"},
		{Company: "Company B", SourceID: "company-b", Title: "Backend Stajı", URL: "https://b.example.test/jobs/1", RawText: "Backend"},
	} {
		listingID, _, err := repository.UpsertRawListing(ctx, listing)
		if err != nil {
			t.Fatal(err)
		}
		if err := repository.SaveAnalysis(ctx, listingID, domain.ListingAnalysis{Location: "Ankara", Eligibility: domain.EligibilitySuitable}); err != nil {
			t.Fatal(err)
		}
	}
	var active int
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunities WHERE status = 'active'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 2 {
		t.Fatalf("company boundary must keep identical roles separate, active=%d", active)
	}
}

func TestRegisterProgramWindowUpsertsCurrentState(t *testing.T) {
	repository, db := newTestRepository(t)
	ctx := context.Background()
	if err := repository.RegisterSource(ctx, domain.SourceRegistration{
		Key: "turkcell-program", Company: "Turkcell", PriorityGroup: "primary",
		Type: "program_page", URL: "https://example.test/program", Adapter: "manual",
		Strategy: "manual", TrackingStatus: "manual", Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	opens := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	closes := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	window := domain.ProgramWindow{
		Key: "gncytnk-staj", Company: "Turkcell", Name: "GNÇYTNK Staj",
		Type: "internship", URL: "https://example.test/apply", Status: "closed",
		OpensAt: &opens, ClosesAt: &closes,
	}
	if err := repository.RegisterProgramWindow(ctx, window); err != nil {
		t.Fatalf("register program window: %v", err)
	}
	window.Status = "open"
	if err := repository.RegisterProgramWindow(ctx, window); err != nil {
		t.Fatalf("update program window: %v", err)
	}
	var count int
	var status, sourceURL string
	if err := db.QueryRow(`SELECT COUNT(*), status, url FROM program_windows WHERE program_key = ?`, window.Key).Scan(&count, &status, &sourceURL); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != "open" || sourceURL != window.URL {
		t.Fatalf("unexpected persisted program: count=%d status=%q url=%q", count, status, sourceURL)
	}
}

func TestRegisterProgramWindowProjectsOpenProgramAsReviewableOpportunityEvidence(t *testing.T) {
	repository, db := newTestRepository(t)
	ctx := context.Background()
	if err := repository.RegisterSource(ctx, domain.SourceRegistration{Key: "program-source", Company: "Program Co", PriorityGroup: "secondary", Type: "program_page", URL: "https://example.test/program", Adapter: "manual", Strategy: "manual", TrackingStatus: "manual"}); err != nil {
		t.Fatal(err)
	}
	verified := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	if err := repository.RegisterProgramWindow(ctx, domain.ProgramWindow{Key: "program-co-intern", Company: "Program Co", Name: "Program Co Staj", Type: "internship", URL: "https://example.test/apply", Status: "open", LastVerifiedAt: &verified}); err != nil {
		t.Fatal(err)
	}
	var kind, layer, sourceType string
	var evidenceCount int
	if err := db.QueryRow(`
		SELECT o.opportunity_type, o.visibility_layer, e.source_type
		FROM opportunities o JOIN opportunity_evidence e ON e.opportunity_id = o.id
		JOIN program_windows p ON p.id = e.program_window_id WHERE p.program_key = ?
	`, "program-co-intern").Scan(&kind, &layer, &sourceType); err != nil {
		t.Fatalf("read program opportunity: %v", err)
	}
	if kind != string(domain.OpportunityInternship) || layer != string(domain.VisibilityReview) || sourceType != "program_window" {
		t.Fatalf("unexpected program projection: kind=%q layer=%q source=%q", kind, layer, sourceType)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM opportunity_evidence WHERE program_window_id IS NOT NULL`).Scan(&evidenceCount); err != nil || evidenceCount != 1 {
		t.Fatalf("expected one program evidence: count=%d err=%v", evidenceCount, err)
	}
}

func TestRegisterSourcePersistsCoverageAndTrust(t *testing.T) {
	repository, db := newTestRepository(t)
	err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "official", Company: "Example", PriorityGroup: "primary", Type: "career_page",
		URL: "https://example.test/careers", Adapter: "manual", Strategy: "manual",
		TrackingStatus: "manual", CoverageStatus: "manual", CoverageReason: "Login gerekiyor.",
		TrustLevel: "official_company", Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	var coverage, reason, trust string
	if err := db.QueryRow(`SELECT coverage_status, coverage_reason, trust_level FROM company_sources WHERE source_key = 'official'`).Scan(&coverage, &reason, &trust); err != nil {
		t.Fatal(err)
	}
	if coverage != "manual" || reason != "Login gerekiyor." || trust != "official_company" {
		t.Fatalf("unexpected classification: %q %q %q", coverage, reason, trust)
	}
}

func TestCoverageSeparatesTrackingPhaseAndPersistsResearchMetadata(t *testing.T) {
	repository, db := newTestRepository(t)
	verified := time.Date(2026, 8, 11, 0, 0, 0, 0, time.FixedZone("TRT", 3*60*60))
	err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "phase165", Company: "Research Co", PriorityGroup: "secondary", TrackingPhase: "16.5",
		Type: "career_page", URL: "https://example.test/careers", Adapter: "manual", Strategy: "manual",
		TrackingStatus: "manual", CoverageStatus: "manual", CoverageReason: "Aday hesabı gerekiyor.",
		CoverageReasonCode: "account_required", LastVerifiedAt: &verified, TrustLevel: "official_company", Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	var phase, reasonCode, verifiedAt string
	if err := db.QueryRow(`SELECT companies.tracking_phase, company_sources.coverage_reason_code, company_sources.last_verified_at
		FROM companies JOIN company_sources ON company_sources.company_id = companies.id WHERE source_key = 'phase165'`).Scan(&phase, &reasonCode, &verifiedAt); err != nil {
		t.Fatal(err)
	}
	if phase != "16.5" || reasonCode != "account_required" || verifiedAt == "" {
		t.Fatalf("unexpected persisted research metadata: phase=%q reason=%q verified=%q", phase, reasonCode, verifiedAt)
	}
	report, err := repository.Coverage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	phaseSummary := report.SectionSummaries["phase_16_5"]
	if phaseSummary.TotalCompanies != 1 || phaseSummary.ManualSources != 1 || report.SectionSummaries["secondary"].TotalCompanies != 0 {
		t.Fatalf("unexpected section summaries: %#v", report.SectionSummaries)
	}
	company := report.Companies[0]
	if company.TrackingPhase != "16.5" || company.Sources[0].ReasonCode != "account_required" || company.Sources[0].LastVerifiedAt == nil {
		t.Fatalf("unexpected coverage detail: %#v", company)
	}
}

func TestNotificationRequiresHighTrustSource(t *testing.T) {
	repository, db := newTestRepository(t)
	ctx := context.Background()
	for _, source := range []domain.SourceRegistration{
		{Key: "official", Company: "Official Co", PriorityGroup: "primary", Type: "official_ats_posting", URL: "https://official.test/jobs/1", Adapter: "lever", Enabled: true, TrustLevel: "official_ats"},
		{Key: "aggregator", Company: "Aggregator Co", PriorityGroup: "primary", Type: "career_page", URL: "https://aggregator.test/company", Adapter: "kariyer_net", Enabled: true, TrustLevel: "aggregator"},
	} {
		if err := repository.RegisterSource(ctx, source); err != nil {
			t.Fatal(err)
		}
		listingID, _, err := repository.UpsertRawListing(ctx, domain.RawListing{Company: source.Company, SourceID: source.Key, Title: "Backend Stajyeri", URL: source.URL, RawText: "Backend staj"})
		if err != nil {
			t.Fatal(err)
		}
		if err := repository.SaveAnalysis(ctx, listingID, suitablePrimaryAnalysis()); err != nil {
			t.Fatal(err)
		}
	}
	var notifications int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("only the high-trust source should notify, got %d events", notifications)
	}
}

func TestSecondaryNotificationRequiresStrongFocusMatchAndHighTrustSource(t *testing.T) {
	repository, db := newTestRepository(t)
	ctx := context.Background()
	tests := []struct {
		source domain.SourceRegistration
		strong bool
	}{
		{source: domain.SourceRegistration{Key: "official-strong", Company: "Official Strong", PriorityGroup: "secondary", Type: "career_page", URL: "https://official.test/jobs/strong", Adapter: "career_links", Enabled: true, TrustLevel: "official_company"}, strong: true},
		{source: domain.SourceRegistration{Key: "official-weak", Company: "Official Weak", PriorityGroup: "secondary", Type: "career_page", URL: "https://official.test/jobs/weak", Adapter: "career_links", Enabled: true, TrustLevel: "official_company"}},
		{source: domain.SourceRegistration{Key: "aggregator-strong", Company: "Aggregator Strong", PriorityGroup: "secondary", Type: "career_page", URL: "https://aggregator.test/jobs/strong", Adapter: "kariyer_net", Enabled: true, TrustLevel: "aggregator"}, strong: true},
	}
	for _, test := range tests {
		if err := repository.RegisterSource(ctx, test.source); err != nil {
			t.Fatal(err)
		}
		listingID, _, err := repository.UpsertRawListing(ctx, domain.RawListing{
			Company: test.source.Company, SourceID: test.source.Key, Title: "Backend Intern",
			URL: test.source.URL, RawText: "Backend internship",
		})
		if err != nil {
			t.Fatal(err)
		}
		analysis := domain.ListingAnalysis{OpportunityType: "staj", ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilitySuitable, Confidence: 0.7}
		if test.strong {
			analysis.MatchingAreas = []string{"backend"}
		}
		if err := repository.SaveAnalysis(ctx, listingID, analysis); err != nil {
			t.Fatal(err)
		}
	}
	var notifications int
	var eventType string
	if err := db.QueryRow("SELECT COUNT(*), MIN(event_type) FROM notifications").Scan(&notifications, &eventType); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 || eventType != domain.NewSecondaryStrongMatchEvent {
		t.Fatalf("only high-trust strong secondary match should notify: count=%d event=%q", notifications, eventType)
	}
}

func suitablePrimaryAnalysis() domain.ListingAnalysis {
	return domain.ListingAnalysis{ApplicationOpen: true, Relevant: true, Eligibility: domain.EligibilitySuitable, Confidence: 0.95}
}

func TestCoverageReportsPrimaryAndSecondarySourcesSeparatelyAndExcludesManualFromAutomaticDenominator(t *testing.T) {
	repository, _ := newTestRepository(t)
	ctx := context.Background()
	registrations := []domain.SourceRegistration{
		{Key: "automatic", Company: "Auto Co", PriorityGroup: "primary", Type: "career_page", URL: "https://auto.test/jobs", Adapter: "json_ld", Strategy: "json_ld", Enabled: true, CoverageStatus: "automatic", TrustLevel: "official_company"},
		{Key: "manual", Company: "Manual Co", PriorityGroup: "primary", Type: "career_page", URL: "https://manual.test/jobs", Adapter: "manual", Strategy: "manual", TrackingStatus: "manual", Enabled: false, CoverageStatus: "manual", CoverageReason: "Elle izleniyor.", TrustLevel: "official_company"},
		{Key: "research", Company: "Research Co", PriorityGroup: "primary", Type: "career_page", URL: "https://research.test/jobs", Adapter: "manual", Strategy: "manual", TrackingStatus: "manual", Enabled: false, CoverageStatus: "researching", CoverageReason: "Adapter araştırılıyor.", TrustLevel: "official_company"},
		{Key: "secondary", Company: "Secondary Co", PriorityGroup: "secondary", Type: "career_page", URL: "https://secondary.test/jobs", Adapter: "json_ld", Strategy: "json_ld", Enabled: true, CoverageStatus: "automatic", TrustLevel: "official_company"},
	}
	for _, registration := range registrations {
		if err := repository.RegisterSource(ctx, registration); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.RegisterProgramWindow(ctx, domain.ProgramWindow{Key: "manual-program", Company: "Manual Co", Name: "Staj", Type: "internship", URL: "https://manual.test/apply", Status: "closed"}); err != nil {
		t.Fatal(err)
	}
	report, err := repository.Coverage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.TotalCompanies != 4 || report.Summary.TotalSources != 4 || report.Summary.AutomaticSources != 2 || report.Summary.ManualSources != 1 || report.Summary.ResearchingSources != 1 {
		t.Fatalf("unexpected coverage summary: %#v", report.Summary)
	}
	if report.Summary.AutomaticEligibleSources != 3 || report.Summary.AutomaticCoveragePercent < 66.6 || report.Summary.AutomaticCoveragePercent > 66.7 {
		t.Fatalf("manual source must be excluded from automatic denominator: %#v", report.Summary)
	}
	primary := report.PrioritySummaries["primary"]
	if primary.TotalCompanies != 3 || primary.AutomaticSources != 1 || primary.ManualSources != 1 ||
		primary.ResearchingSources != 1 || primary.AutomaticEligibleSources != 2 || primary.AutomaticCoveragePercent != 50 {
		t.Fatalf("unexpected primary coverage: %#v", primary)
	}
	secondary := report.PrioritySummaries["secondary"]
	if secondary.TotalCompanies != 1 || secondary.AutomaticSources != 1 || secondary.AutomaticEligibleSources != 1 || secondary.AutomaticCoveragePercent != 100 {
		t.Fatalf("unexpected secondary coverage: %#v", secondary)
	}
	if len(report.Companies) != 4 || len(report.Programs) != 1 || report.Programs[0].Status != "closed" {
		t.Fatalf("unexpected coverage detail: %#v", report)
	}
}

func registerMeteksanSources(t *testing.T, repository *SQLiteRepository) {
	t.Helper()
	registerMeteksan(t, repository)
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "meteksan-careers", Company: "Meteksan Savunma", PriorityGroup: "primary",
		Type: "career_page", URL: "https://careers.example.test/meteksan", Adapter: "json_ld", Enabled: true,
		TrustLevel: "official_company",
	}); err != nil {
		t.Fatalf("register second source: %v", err)
	}
}

func insertOpportunityListing(t *testing.T, repository *SQLiteRepository, source, title, listingURL string) string {
	t.Helper()
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: source, Title: title,
		URL: listingURL, RawText: title,
	})
	if err != nil {
		t.Fatalf("insert opportunity listing: %v", err)
	}
	return listingID
}

func newTestRepository(t *testing.T) (*SQLiteRepository, queryDB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "tracker.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	return repository, db
}

func registerMeteksan(t *testing.T, repository *SQLiteRepository) {
	t.Helper()
	err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "meteksan-kariyer-net", Company: "Meteksan Savunma", PriorityGroup: "primary",
		Type: "career_page", URL: "https://www.kariyer.net/firma-profil/meteksan", Adapter: "kariyer_net", Enabled: true,
		TrustLevel: "official_company",
	})
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
}

type queryDB interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}
