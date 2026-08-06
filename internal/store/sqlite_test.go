package store

import (
	"context"
	"database/sql"
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
	})
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
}

type queryDB interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}
