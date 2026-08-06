package httpapi

import (
	"context"
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

type fakeTrackingRepository struct {
	detail store.ListingDetail
	saved  store.ApplicationTracking
}

func (f *fakeTrackingRepository) Dashboard(context.Context) (store.DashboardSnapshot, error) {
	return store.DashboardSnapshot{}, nil
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
	handler := NewHandler("http://localhost:5173", logger, nil, nil)
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

func TestScanReturnsAggregatedResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler("http://localhost:5173", logger, fakeScanRunner{
		result: orchestrator.ScanResult{RunID: 7, Status: "completed", Sources: []orchestrator.SourceResult{{
			Source: "meteksan-kariyer-net", Found: 2, New: 1,
		}}},
	}, fakeDashboardRepository{})
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
	}, fakeDashboardRepository{})
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
	}, fakeDashboardRepository{})
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
	})
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
	}, fakeDashboardRepository{})
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
	handler := NewHandler("http://localhost:5173", logger, nil, repository)

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
	handler := NewHandler("http://localhost:5173", logger, nil, repository)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/listings/listing-1/application",
		strings.NewReader(`{"status":"basvuruldu","deadline":"tomorrow"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || repository.saved.Status != "" {
		t.Fatalf("unexpected invalid timestamp response: status=%d saved=%#v", response.Code, repository.saved)
	}
}
