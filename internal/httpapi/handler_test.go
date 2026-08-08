package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type fakeScanRunner struct {
	result    orchestrator.ScanResult
	reprocess orchestrator.ReprocessResult
	err       error
}

func (f fakeScanRunner) Run(context.Context, string) (orchestrator.ScanResult, error) {
	return f.result, f.err
}

func (f fakeScanRunner) ReprocessPending(context.Context, int) (orchestrator.ReprocessResult, error) {
	return f.reprocess, nil
}

type fakeDashboardRepository struct {
	snapshot store.DashboardSnapshot
}

type fakeReadinessChecker struct {
	err   error
	calls int
}

func (f *fakeReadinessChecker) Check(context.Context) error {
	f.calls++
	return f.err
}

type fakeTrackingRepository struct {
	detail        store.ListingDetail
	saved         store.ApplicationTracking
	dashboard     store.DashboardSnapshot
	checkedSource string
	checkedAt     time.Time
}

type fakePushRepository struct {
	fakeDashboardRepository
	saved   store.PushSubscriptionInput
	deleted string
	created bool
}

func (f *fakePushRepository) UpsertPushSubscription(_ context.Context, subscription store.PushSubscriptionInput) (bool, error) {
	f.saved = subscription
	return f.created, nil
}

func (f *fakePushRepository) DeletePushSubscription(_ context.Context, endpoint string) error {
	f.deleted = endpoint
	return nil
}

func (f *fakeTrackingRepository) Dashboard(context.Context) (store.DashboardSnapshot, error) {
	return f.dashboard, nil
}

func (f *fakeTrackingRepository) MarkSourceChecked(_ context.Context, sourceKey string, checkedAt time.Time) error {
	if sourceKey == "missing-source" {
		return store.ErrSourceNotFound
	}
	f.checkedSource = sourceKey
	f.checkedAt = checkedAt
	return nil
}

func (f *fakeTrackingRepository) ListingDetail(_ context.Context, listingID string) (store.ListingDetail, error) {
	if listingID != f.detail.ID {
		return store.ListingDetail{}, store.ErrListingNotFound
	}
	result := f.detail
	if f.saved.Status != "" {
		result.Application = &f.saved
		result.ApplicationStatus = f.saved.Status
	}
	return result, nil
}

func (f *fakeTrackingRepository) SaveApplication(_ context.Context, listingID string, tracking store.ApplicationTracking) error {
	if listingID != f.detail.ID {
		return store.ErrListingNotFound
	}
	f.saved = tracking
	return nil
}

func (f fakeDashboardRepository) Dashboard(context.Context) (store.DashboardSnapshot, error) {
	return f.snapshot, nil
}

func TestHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("http://localhost:5173", logger, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestReadyUsesHealthCheckerWithoutChangingLiveness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	checker := &fakeReadinessChecker{}
	handler := NewHandler("http://localhost:5173", logger, nil, nil, checker)

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), `"status":"ready"`) || checker.calls != 1 {
		t.Fatalf("unexpected ready response: status=%d calls=%d body=%s", ready.Code, checker.calls, ready.Body.String())
	}

	live := httptest.NewRecorder()
	handler.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/health", nil))
	if live.Code != http.StatusOK || checker.calls != 1 {
		t.Fatalf("health must remain a dependency-free liveness check: status=%d calls=%d", live.Code, checker.calls)
	}
}

func TestReadyReturnsGenericServiceUnavailableWhenDatabaseIsNotReady(t *testing.T) {
	checker := &fakeReadinessChecker{err: errors.New("schema_migrations is unavailable")}
	handler := NewHandler("http://localhost:5173", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, checker)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"status":"not_ready"`) ||
		strings.Contains(response.Body.String(), "schema_migrations") {
		t.Fatalf("unexpected failed readiness response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMiddlewareAddsSecurityHeadersAndLogsResponseStatus(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := NewHandler("http://localhost:5173", logger, nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-Frame-Options":        "DENY",
	} {
		if got := response.Header().Get(header); got != want {
			t.Fatalf("expected %s=%q, got %q", header, want, got)
		}
	}
	if csp := response.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") ||
		!strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("unexpected CSP: %q", csp)
	}
	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodOptions, "/api/v1/dashboard", nil))
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("unexpected preflight response: %d", preflight.Code)
	}
	if !strings.Contains(logs.String(), "status=200") || !strings.Contains(logs.String(), "status=204") {
		t.Fatalf("response status was not logged: %s", logs.String())
	}
}

func TestProductionOriginProtectionRejectsUnsafeRequestsAndKeepsReadEndpointsAvailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("https://tracker.example.test", logger, fakeScanRunner{
		result: orchestrator.ScanResult{RunID: 1, Status: "completed"},
	}, fakeDashboardRepository{}, nil, Options{RequireExactOrigin: true})

	for _, test := range []struct {
		name, method, path, origin string
	}{
		{"missing POST origin", http.MethodPost, "/api/v1/scan", ""},
		{"wrong PUT origin", http.MethodPut, "/api/v1/listings/listing-1/application", "https://attacker.example.test"},
		{"wrong DELETE origin", http.MethodDelete, "/api/v1/push/subscriptions", "https://attacker.example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "origin is not allowed") {
				t.Fatalf("unsafe request was not blocked: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	accepted := httptest.NewRequest(http.MethodPost, "/api/v1/scan", nil)
	accepted.Header.Set("Origin", "https://tracker.example.test")
	acceptedResponse := httptest.NewRecorder()
	handler.ServeHTTP(acceptedResponse, accepted)
	if acceptedResponse.Code != http.StatusOK {
		t.Fatalf("expected matching origin to be accepted, got %d", acceptedResponse.Code)
	}

	read := httptest.NewRequest(http.MethodGet, "/health", nil)
	read.Header.Set("Origin", "https://attacker.example.test")
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read endpoint must remain available, got %d", readResponse.Code)
	}
}

func TestCORSPreflightOnlyAllowsConfiguredOrigin(t *testing.T) {
	handler := NewHandler("https://tracker.example.test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, Options{RequireExactOrigin: true})

	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1/scan", nil)
	allowed.Header.Set("Origin", "https://tracker.example.test")
	allowed.Header.Set("Access-Control-Request-Method", http.MethodPost)
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://tracker.example.test" ||
		allowedResponse.Header().Get("Vary") != "Origin" {
		t.Fatalf("allowed preflight is invalid: status=%d headers=%#v", allowedResponse.Code, allowedResponse.Header())
	}

	denied := httptest.NewRequest(http.MethodOptions, "/api/v1/scan", nil)
	denied.Header.Set("Origin", "https://attacker.example.test")
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden || deniedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("untrusted preflight was not rejected: status=%d headers=%#v", deniedResponse.Code, deniedResponse.Header())
	}
}

func TestScanReturnsAggregatedResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("http://localhost:5173", logger, fakeScanRunner{
		result: orchestrator.ScanResult{RunID: 7, Status: "completed", Sources: []orchestrator.SourceResult{{
			Source: "meteksan-kariyer-net", Found: 2, New: 1,
		}}},
	}, fakeDashboardRepository{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"new":1`) || !strings.Contains(response.Body.String(), `"run_id":7`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestScanReturnsSkippedSourceRetryTime(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	retryAt := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	handler := NewHandler("http://localhost:5173", logger, fakeScanRunner{
		result: orchestrator.ScanResult{RunID: 8, Status: "failed", Sources: []orchestrator.SourceResult{{
			Source: "aselsan-kariyer-net", Skipped: true, RetryAt: &retryAt,
		}}},
	}, fakeDashboardRepository{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMultiStatus || !strings.Contains(response.Body.String(), `"skipped":true`) ||
		!strings.Contains(response.Body.String(), `"retry_at":"2026-08-04T09:00:00Z"`) {
		t.Fatalf("unexpected guarded scan response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestScanReturnsConflictWhenAnotherScanIsRunning(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("http://localhost:5173", logger, fakeScanRunner{
		err: orchestrator.ErrScanInProgress,
	}, fakeDashboardRepository{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "already in progress") {
		t.Fatalf("unexpected concurrent scan response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDashboardUsesRepository(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("http://localhost:5173", logger, nil, fakeDashboardRepository{
		snapshot: store.DashboardSnapshot{NewListings: []store.DashboardListing{{
			ID: "1", Company: "Meteksan", Title: "Staj", URL: "https://example.test/1", Priority: "primary",
		}}},
	}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"company":"Meteksan"`) {
		t.Fatalf("unexpected dashboard response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRetryAnalysesReturnsReprocessResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("http://localhost:5173", logger, fakeScanRunner{
		reprocess: orchestrator.ReprocessResult{Found: 3, Processed: 2, Failed: 1},
	}, fakeDashboardRepository{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/analyses/retry", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMultiStatus || !strings.Contains(response.Body.String(), `"processed":2`) {
		t.Fatalf("unexpected retry response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestListingDetailAndApplicationUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := &fakeTrackingRepository{detail: store.ListingDetail{
		DashboardListing: store.DashboardListing{
			ID: "listing-1", Company: "Meteksan", Title: "Backend Stajyeri",
			URL: "https://example.test/listing-1", Priority: "primary",
			Eligibility: domain.EligibilitySuitable, Summary: "Backend stajı",
		},
		MatchingAreas: []string{"backend"},
	}}
	handler := NewHandler("http://localhost:5173", logger, nil, repository, nil)

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/listings/listing-1", nil)
	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), `"summary":"Backend stajı"`) {
		t.Fatalf("unexpected detail response: status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}

	body := `{"status":"basvuruldu","deadline":"2026-08-10T18:00:00+03:00","interview_at":null,"notes":"Dönüş bekleniyor"}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/v1/listings/listing-1/application", strings.NewReader(body))
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK || repository.saved.Status != domain.ApplicationSubmitted ||
		repository.saved.Deadline == nil || repository.saved.Notes != "Dönüş bekleniyor" {
		t.Fatalf("application was not saved: status=%d saved=%#v body=%s", updateResponse.Code, repository.saved, updateResponse.Body.String())
	}
}

func TestApplicationUpdateRejectsInvalidTimestamp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := &fakeTrackingRepository{detail: store.ListingDetail{
		DashboardListing: store.DashboardListing{ID: "listing-1"},
	}}
	handler := NewHandler("http://localhost:5173", logger, nil, repository, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/listings/listing-1/application",
		strings.NewReader(`{"status":"basvuruldu","deadline":"tomorrow"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || repository.saved.Status != "" {
		t.Fatalf("unexpected invalid timestamp response: status=%d saved=%#v", response.Code, repository.saved)
	}
}

func TestWatchlistCheckedMarksSourceAndReturnsDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := &fakeTrackingRepository{
		dashboard: store.DashboardSnapshot{Watchlist: []store.WatchlistEntry{{SourceID: "akdogan-tech-career", Company: "Akdoğan Tech"}}},
	}
	handler := NewHandler("http://localhost:5173", logger, nil, repository, nil)

	request := httptest.NewRequest(http.MethodPut, "/api/v1/watchlist/akdogan-tech-career/checked", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || repository.checkedSource != "akdogan-tech-career" ||
		!strings.Contains(response.Body.String(), "akdogan-tech-career") {
		t.Fatalf("unexpected watchlist checked response: status=%d checked=%q body=%s",
			response.Code, repository.checkedSource, response.Body.String())
	}
}

func TestWatchlistCheckedRejectsUnknownSourceAndWrongMethod(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := &fakeTrackingRepository{}
	handler := NewHandler("http://localhost:5173", logger, nil, repository, nil)

	notFoundRequest := httptest.NewRequest(http.MethodPut, "/api/v1/watchlist/missing-source/checked", nil)
	notFoundResponse := httptest.NewRecorder()
	handler.ServeHTTP(notFoundResponse, notFoundRequest)
	if notFoundResponse.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown source, got %d", notFoundResponse.Code)
	}

	wrongMethodRequest := httptest.NewRequest(http.MethodGet, "/api/v1/watchlist/akdogan-tech-career/checked", nil)
	wrongMethodResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongMethodResponse, wrongMethodRequest)
	if wrongMethodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", wrongMethodResponse.Code)
	}
}

func TestPushSubscriptionAPIValidatesAndPersistsBrowserSubscription(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := &fakePushRepository{created: true}
	handler := NewHandler("http://localhost:5173", logger, nil, repository, nil, Options{Push: PushOptions{
		Enabled: true, PublicKey: "public-vapid-key", Store: repository,
	}})
	keyRequest := httptest.NewRequest(http.MethodGet, "/api/v1/push/public-key", nil)
	keyResponse := httptest.NewRecorder()
	handler.ServeHTTP(keyResponse, keyRequest)
	if keyResponse.Code != http.StatusOK || !strings.Contains(keyResponse.Body.String(), `"public_key":"public-vapid-key"`) {
		t.Fatalf("unexpected public key response: status=%d body=%s", keyResponse.Code, keyResponse.Body.String())
	}

	privateKey, err := ecdh.P256().GenerateKey(strings.NewReader(strings.Repeat("k", 256)))
	if err != nil {
		t.Fatal(err)
	}
	p256dh := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes())
	auth := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef"))
	body := `{"endpoint":"https://push.example.test/device","expirationTime":null,"keys":{"p256dh":"` + p256dh + `","auth":"` + auth + `"}}`
	subscribeRequest := httptest.NewRequest(http.MethodPut, "/api/v1/push/subscriptions", strings.NewReader(body))
	subscribeRequest.Header.Set("Content-Type", "application/json")
	subscribeResponse := httptest.NewRecorder()
	handler.ServeHTTP(subscribeResponse, subscribeRequest)
	if subscribeResponse.Code != http.StatusCreated || repository.saved.Endpoint != "https://push.example.test/device" {
		t.Fatalf("subscription was not saved: status=%d saved=%#v body=%s", subscribeResponse.Code, repository.saved, subscribeResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/push/subscriptions", strings.NewReader(`{"endpoint":"https://push.example.test/device"}`))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || repository.deleted != "https://push.example.test/device" {
		t.Fatalf("subscription was not deleted: status=%d endpoint=%q", deleteResponse.Code, repository.deleted)
	}
}

func TestPushSubscriptionAPIRejectsUnsafeAndNonStrictRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := &fakePushRepository{}
	handler := NewHandler("http://localhost:5173", logger, nil, repository, nil, Options{Push: PushOptions{
		Enabled: true, PublicKey: "public", Store: repository,
	}})
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "content type", contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unknown field", contentType: "application/json", body: `{"endpoint":"https://push.example.test/x","unknown":true}`, wantStatus: http.StatusBadRequest},
		{name: "unsafe endpoint", contentType: "application/json", body: `{"endpoint":"http://127.0.0.1/x","keys":{"p256dh":"x","auth":"x"}}`, wantStatus: http.StatusBadRequest},
		{name: "trailing JSON", contentType: "application/json", body: `{} {}`, wantStatus: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/v1/push/subscriptions", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("expected %d, got %d: %s", test.wantStatus, response.Code, response.Body.String())
			}
		})
	}
}

func TestPushAPIIsUnavailableWhenDisabled(t *testing.T) {
	handler := NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, fakeDashboardRepository{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/push/public-key", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected disabled push API to return 503, got %d", response.Code)
	}
}
