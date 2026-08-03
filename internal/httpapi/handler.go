package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type ScanRunner interface {
	Run(ctx context.Context, trigger string) (orchestrator.ScanResult, error)
}

type Handler struct {
	allowedOrigin  string
	logger         *slog.Logger
	startedAt      time.Time
	scanner        ScanRunner
	dashboardStore store.DashboardRepository
}

func NewHandler(
	allowedOrigin string,
	logger *slog.Logger,
	scanner ScanRunner,
	dashboardStore store.DashboardRepository,
) http.Handler {
	handler := Handler{
		allowedOrigin:  allowedOrigin,
		logger:         logger,
		startedAt:      time.Now().UTC(),
		scanner:        scanner,
		dashboardStore: dashboardStore,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.health)
	mux.HandleFunc("/api/v1/dashboard", handler.dashboard)
	mux.HandleFunc("/api/v1/scan", handler.scan)

	return handler.withMiddleware(mux)
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
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

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
