package acceptance_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type phase141ScanRunner struct {
	result orchestrator.ScanResult
	err    error
}

func (r phase141ScanRunner) Run(context.Context, string) (orchestrator.ScanResult, error) {
	return r.result, r.err
}

func TestPhase141HistorySurvivesRestartAndRestoreAndScanAlwaysReturnsJSON(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "persistent", "tracker.db")
	db, repository := openPhase141Repository(t, databasePath)
	registerSource(t, repository, commencisURL)
	fetchedAt := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	listingID, _, err := repository.UpsertRawListing(ctx, domain.RawListing{
		Company: "Commencis", SourceID: "commencis-lever-spring-boot-camp-2026",
		Title: commencisTitle, URL: commencisURL, RawText: "Backend internship", FetchedAt: fetchedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := fetchedAt.Add(48 * time.Hour)
	interview := fetchedAt.Add(96 * time.Hour)
	if err := repository.SaveAnalysis(ctx, listingID, domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: false,
		Eligibility: domain.EligibilityUnsuitable, Summary: "Dashboard kovaları dışında kalan kanıt.",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveApplication(ctx, listingID, store.ApplicationTracking{
		Status: domain.ApplicationSubmitted, Deadline: &deadline, InterviewAt: &interview, Notes: "Kalıcı kullanıcı notu.",
	}); err != nil {
		t.Fatal(err)
	}
	detail, err := repository.ListingDetail(ctx, listingID)
	if err != nil {
		t.Fatal(err)
	}
	opportunityID := detail.OpportunityID
	if err := repository.UpdateOpportunityLifecycle(ctx, opportunityID, domain.OpportunityArchived); err != nil {
		t.Fatal(err)
	}

	handler := httpapi.NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, repository, nil)
	historyResponse := serveRequest(handler, http.MethodGet, "/api/v1/opportunities?lifecycle=arsivlendi&page=1&page_size=10", "")
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), opportunityID) ||
		!strings.Contains(historyResponse.Body.String(), `"total":1`) {
		t.Fatalf("previous opportunity is not visible in history: status=%d body=%s", historyResponse.Code, historyResponse.Body.String())
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, repository = openPhase141Repository(t, databasePath)
	assertPhase141UserData(t, repository, listingID, opportunityID, deadline, interview)

	backupPath := filepath.Join(root, "restored.db")
	if err := database.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("create restore proof: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, restored := openPhase141Repository(t, backupPath)
	assertPhase141UserData(t, restored, listingID, opportunityID, deadline, interview)

	for _, test := range []struct {
		name   string
		runner phase141ScanRunner
		status int
	}{
		{name: "success", runner: phase141ScanRunner{result: orchestrator.ScanResult{RunID: 1, Status: "completed"}}, status: http.StatusOK},
		{name: "conflict", runner: phase141ScanRunner{err: orchestrator.ErrScanInProgress}, status: http.StatusConflict},
		{name: "failure", runner: phase141ScanRunner{err: errors.New("internal")}, status: http.StatusInternalServerError},
	} {
		t.Run("scan_"+test.name, func(t *testing.T) {
			scanHandler := httpapi.NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), test.runner, restored, nil)
			response := serveRequest(scanHandler, http.MethodPost, "/api/v1/scan", "")
			if response.Code != test.status || response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
				!json.Valid(response.Body.Bytes()) {
				t.Fatalf("scan response contract failed: status=%d content-type=%q body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
		})
	}
}

func openPhase141Repository(t *testing.T, path string) (*sql.DB, *store.SQLiteRepository) {
	t.Helper()
	db, err := database.Open(context.Background(), path, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return db, repository
}

func assertPhase141UserData(t *testing.T, repository *store.SQLiteRepository, listingID, opportunityID string, deadline, interview time.Time) {
	t.Helper()
	detail, err := repository.ListingDetail(context.Background(), listingID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.OpportunityID != opportunityID || detail.Lifecycle != domain.OpportunityArchived || detail.Application == nil ||
		detail.Application.Status != domain.ApplicationSubmitted || detail.Application.Notes != "Kalıcı kullanıcı notu." ||
		detail.Application.Deadline == nil || !detail.Application.Deadline.Equal(deadline) ||
		detail.Application.InterviewAt == nil || !detail.Application.InterviewAt.Equal(interview) {
		t.Fatalf("restart/restore changed durable opportunity or user data: %#v", detail)
	}
}
