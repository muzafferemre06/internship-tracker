package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/push"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type ScanRunner interface {
	Run(ctx context.Context, trigger string) (orchestrator.ScanResult, error)
}

type AnalysisRetrier interface {
	ReprocessPending(ctx context.Context, limit int) (orchestrator.ReprocessResult, error)
}

type ReadinessChecker interface {
	Check(ctx context.Context) error
}

type Handler struct {
	allowedOrigin   string
	logger          *slog.Logger
	startedAt       time.Time
	scanner         ScanRunner
	analysisRetrier AnalysisRetrier
	dashboardStore  store.DashboardRepository
	trackingStore   store.TrackingRepository
	readiness       ReadinessChecker
	pushEnabled     bool
	pushPublicKey   string
	pushStore       store.PushSubscriptionRepository
}

type PushOptions struct {
	Enabled   bool
	PublicKey string
	Store     store.PushSubscriptionRepository
}

func NewHandler(
	allowedOrigin string,
	logger *slog.Logger,
	scanner ScanRunner,
	dashboardStore store.DashboardRepository,
	readiness ReadinessChecker,
	pushOptions ...PushOptions,
) http.Handler {
	handler := Handler{
		allowedOrigin:   allowedOrigin,
		logger:          logger,
		startedAt:       time.Now().UTC(),
		scanner:         scanner,
		analysisRetrier: analysisRetrier(scanner),
		dashboardStore:  dashboardStore,
		trackingStore:   trackingRepository(dashboardStore),
		readiness:       readiness,
	}
	if len(pushOptions) > 0 {
		handler.pushEnabled = pushOptions[0].Enabled
		handler.pushPublicKey = pushOptions[0].PublicKey
		handler.pushStore = pushOptions[0].Store
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.health)
	mux.HandleFunc("/ready", handler.ready)
	mux.HandleFunc("/api/v1/dashboard", handler.dashboard)
	mux.HandleFunc("/api/v1/scan", handler.scan)
	mux.HandleFunc("/api/v1/analyses/retry", handler.retryAnalyses)
	mux.HandleFunc("/api/v1/listings/{id}", handler.listingDetail)
	mux.HandleFunc("/api/v1/listings/{id}/application", handler.application)
	mux.HandleFunc("/api/v1/push/public-key", handler.pushKey)
	mux.HandleFunc("/api/v1/push/subscriptions", handler.pushSubscriptions)

	return handler.withMiddleware(mux)
}

type pushSubscriptionRequest struct {
	Endpoint       string   `json:"endpoint"`
	ExpirationTime *float64 `json:"expirationTime"`
	Keys           struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type pushUnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

func (h Handler) pushKey(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if !h.pushEnabled {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "Web Push is disabled"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"public_key": h.pushPublicKey})
}

func (h Handler) pushSubscriptions(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut && request.Method != http.MethodDelete {
		methodNotAllowed(writer)
		return
	}
	if !h.pushEnabled || h.pushStore == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "Web Push is disabled"})
		return
	}
	if !isJSONRequest(request) {
		writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if request.Method == http.MethodDelete {
		var input pushUnsubscribeRequest
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil || strings.TrimSpace(input.Endpoint) == "" {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "push unsubscribe request is invalid"})
			return
		}
		if err := push.ValidateEndpoint(input.Endpoint); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := h.pushStore.DeletePushSubscription(request.Context(), input.Endpoint); err != nil {
			h.logger.Error("push subscription deletion failed", "error", err)
			writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "push subscription could not be deleted"})
			return
		}
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	var input pushSubscriptionRequest
	if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "push subscription request is invalid"})
		return
	}
	if err := push.ValidateSubscription(input.Endpoint, input.Keys.P256DH, input.Keys.Auth); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var expirationAt *time.Time
	if input.ExpirationTime != nil {
		if math.IsNaN(*input.ExpirationTime) || math.IsInf(*input.ExpirationTime, 0) || *input.ExpirationTime <= 0 || *input.ExpirationTime > 32503680000000 {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "push expirationTime is invalid"})
			return
		}
		value := time.UnixMilli(int64(*input.ExpirationTime)).UTC()
		expirationAt = &value
	}
	created, err := h.pushStore.UpsertPushSubscription(request.Context(), store.PushSubscriptionInput{
		Endpoint: input.Endpoint, P256DH: input.Keys.P256DH, Auth: input.Keys.Auth, ExpirationAt: expirationAt,
	})
	if err != nil {
		h.logger.Error("push subscription save failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "push subscription could not be saved"})
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(writer, status, map[string]bool{"subscribed": true})
}

func isJSONRequest(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
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

func (h Handler) ready(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if h.readiness == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := h.readiness.Check(ctx); err != nil {
		h.logger.Error("readiness check failed", "error", err)
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
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
		if errors.Is(err, orchestrator.ErrScanInProgress) {
			writeJSON(writer, http.StatusConflict, map[string]string{"error": "a scan is already in progress"})
			return
		}
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
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; worker-src 'self'")
		writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Access-Control-Allow-Origin", h.allowedOrigin)
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		started := time.Now()
		response := &statusResponseWriter{ResponseWriter: writer, status: http.StatusOK}
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
		} else {
			next.ServeHTTP(response, request)
		}
		h.logger.Info("http request",
			"method", request.Method,
			"path", request.URL.Path,
			"status", response.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
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
