package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type ScanRunner interface {
	Run(ctx context.Context, trigger string) (orchestrator.ScanResult, error)
}

type AnalysisRetrier interface {
	ReprocessPending(ctx context.Context, limit int) (orchestrator.ReprocessResult, error)
}

type Handler struct {
	allowedOrigin   string
	logger          *slog.Logger
	startedAt       time.Time
	scanner         ScanRunner
	analysisRetrier AnalysisRetrier
	dashboardStore  store.DashboardRepository
	trackingStore   store.TrackingRepository
}

func NewHandler(
	allowedOrigin string,
	logger *slog.Logger,
	scanner ScanRunner,
	dashboardStore store.DashboardRepository,
) http.Handler {
	handler := Handler{
		allowedOrigin:   allowedOrigin,
		logger:          logger,
		startedAt:       time.Now().UTC(),
		scanner:         scanner,
		analysisRetrier: analysisRetrier(scanner),
		dashboardStore:  dashboardStore,
		trackingStore:   trackingRepository(dashboardStore),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.health)
	mux.HandleFunc("/api/v1/dashboard", handler.dashboard)
	mux.HandleFunc("/api/v1/scan", handler.scan)
	mux.HandleFunc("/api/v1/analyses/retry", handler.retryAnalyses)
	mux.HandleFunc("/api/v1/listings/{id}", handler.listingDetail)
	mux.HandleFunc("/api/v1/listings/{id}/application", handler.application)

	return handler.withMiddleware(mux)
}

func trackingRepository(repository store.DashboardRepository) store.TrackingRepository {
	if tracking, ok := repository.(store.TrackingRepository); ok {
		return tracking
	}
	return nil
}

func (h Handler) listingDetail(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if h.trackingStore == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "listing store is unavailable"})
		return
	}
	detail, err := h.trackingStore.ListingDetail(request.Context(), request.PathValue("id"))
	if errors.Is(err, store.ErrListingNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "listing was not found"})
		return
	}
	if err != nil {
		h.logger.Error("listing detail query failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "listing could not be loaded"})
		return
	}
	writeJSON(writer, http.StatusOK, detail)
}

type applicationRequest struct {
	Status      string  `json:"status"`
	Deadline    *string `json:"deadline"`
	InterviewAt *string `json:"interview_at"`
	Notes       string  `json:"notes"`
}

func (h Handler) application(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		methodNotAllowed(writer)
		return
	}
	if h.trackingStore == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "application store is unavailable"})
		return
	}
	var input applicationRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "request body is invalid"})
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "request body must contain one JSON object"})
		return
	}
	tracking := store.ApplicationTracking{Status: domain.ApplicationStatus(input.Status), Notes: input.Notes}
	var err error
	if tracking.Deadline, err = parseOptionalTime(input.Deadline); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "deadline must be an RFC3339 timestamp"})
		return
	}
	if tracking.InterviewAt, err = parseOptionalTime(input.InterviewAt); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "interview_at must be an RFC3339 timestamp"})
		return
	}
	if err := h.trackingStore.SaveApplication(request.Context(), request.PathValue("id"), tracking); err != nil {
		if errors.Is(err, store.ErrListingNotFound) {
			writeJSON(writer, http.StatusNotFound, map[string]string{"error": "listing was not found"})
			return
		}
		if strings.Contains(err.Error(), "invalid application status") || strings.Contains(err.Error(), "cannot exceed") {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.logger.Error("application tracking update failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "application could not be saved"})
		return
	}
	detail, err := h.trackingStore.ListingDetail(request.Context(), request.PathValue("id"))
	if err != nil {
		h.logger.Error("saved application reload failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "saved application could not be loaded"})
		return
	}
	writeJSON(writer, http.StatusOK, detail)
}

func parseOptionalTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("additional JSON value")
		}
		return err
	}
	return nil
}

func analysisRetrier(scanner ScanRunner) AnalysisRetrier {
	if retrier, ok := scanner.(AnalysisRetrier); ok {
		return retrier
	}
	return nil
}

func (h Handler) retryAnalyses(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	if h.analysisRetrier == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "analysis retrier is unavailable"})
		return
	}
	result, err := h.analysisRetrier.ReprocessPending(request.Context(), 25)
	if err != nil {
		h.logger.Error("pending analysis retry failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "pending analyses could not be retried"})
		return
	}
	status := http.StatusOK
	if result.Failed > 0 {
		status = http.StatusMultiStatus
	}
	writeJSON(writer, status, result)
}

func (h Handler) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"status":     "ok",
		"started_at": h.startedAt,
	})
}

func (h Handler) dashboard(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	if h.dashboardStore == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "dashboard store is unavailable"})
		return
	}
	dashboard, err := h.dashboardStore.Dashboard(request.Context())
	if err != nil {
		h.logger.Error("dashboard query failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "dashboard could not be loaded"})
		return
	}
	writeJSON(writer, http.StatusOK, dashboard)
}

func (h Handler) scan(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	if h.scanner == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "scan runner is unavailable"})
		return
	}

	result, err := h.scanner.Run(request.Context(), "manual")
	if err != nil {
		h.logger.Error("scan failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "scan could not be completed"})
		return
	}
	response := scanResponse{
		RunID: result.RunID, Status: result.Status, StartedAt: result.StartedAt,
		FinishedAt: result.FinishedAt, Sources: make([]sourceScanResponse, 0, len(result.Sources)),
	}
	status := http.StatusOK
	for _, source := range result.Sources {
		sourceResponse := sourceScanResponse{
			Source: source.Source, Found: source.Found, New: source.New,
			ProcessErrors: source.ProcessError, Skipped: source.Skipped, RetryAt: source.RetryAt,
		}
		if source.FetchError != nil {
			sourceResponse.FetchError = source.FetchError.Error()
			status = http.StatusMultiStatus
		}
		if source.ProcessError > 0 || source.Skipped {
			status = http.StatusMultiStatus
		}
		response.Found += source.Found
		response.New += source.New
		response.ProcessErrors += source.ProcessError
		response.Sources = append(response.Sources, sourceResponse)
	}
	writeJSON(writer, status, response)
}

type scanResponse struct {
	RunID         int64                `json:"run_id"`
	Status        string               `json:"status"`
	StartedAt     time.Time            `json:"started_at"`
	FinishedAt    time.Time            `json:"finished_at"`
	Found         int                  `json:"found"`
	New           int                  `json:"new"`
	ProcessErrors int                  `json:"process_errors"`
	Sources       []sourceScanResponse `json:"sources"`
}

type sourceScanResponse struct {
	Source        string     `json:"source"`
	Found         int        `json:"found"`
	New           int        `json:"new"`
	ProcessErrors int        `json:"process_errors"`
	FetchError    string     `json:"fetch_error,omitempty"`
	Skipped       bool       `json:"skipped,omitempty"`
	RetryAt       *time.Time `json:"retry_at,omitempty"`
}

func (h Handler) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", h.allowedOrigin)
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")

		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}

		started := time.Now()
		next.ServeHTTP(writer, request)
		h.logger.Info("http request",
			"method", request.Method,
			"path", request.URL.Path,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func methodNotAllowed(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{
		"error": "method not allowed",
	})
}
