package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Handler struct {
	allowedOrigin string
	logger        *slog.Logger
	startedAt     time.Time
}

func NewHandler(allowedOrigin string, logger *slog.Logger) http.Handler {
	handler := Handler{
		allowedOrigin: allowedOrigin,
		logger:        logger,
		startedAt:     time.Now().UTC(),
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

	writeJSON(writer, http.StatusOK, map[string]any{
		"new_listings":       []any{},
		"needs_decision":     []any{},
		"active_applications": []any{},
		"last_scan":          nil,
	})
}

func (h Handler) scan(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusNotImplemented, map[string]string{
		"error": "scan runner is not wired yet",
	})
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
