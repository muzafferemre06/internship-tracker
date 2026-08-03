package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type fakeScanRunner struct {
	result orchestrator.ScanResult
}

func (f fakeScanRunner) Run(context.Context) orchestrator.ScanResult { return f.result }

type fakeDashboardRepository struct {
	snapshot store.DashboardSnapshot
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
		result: orchestrator.ScanResult{Sources: []orchestrator.SourceResult{{
			Source: "meteksan-kariyer-net", Found: 2, New: 1,
		}}},
	}, fakeDashboardRepository{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"new":1`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
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
