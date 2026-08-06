package acceptance_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestPhase4UserCanInspectAndTrackApplication(t *testing.T) {
	_, repository := openRepository(t)
	registerSource(t, repository, commencisURL)
	fetchedAt := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Commencis", SourceID: "commencis-lever-spring-boot-camp-2026",
		Title: commencisTitle, URL: commencisURL, RawText: "Backend internship", FetchedAt: fetchedAt,
	})
	if err != nil {
		t.Fatalf("insert acceptance listing: %v", err)
	}
	applicationDue := time.Date(2026, 8, 14, 20, 59, 59, 0, time.UTC)
	if err := repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		MatchingAreas: []string{"backend"}, Location: "İstanbul", WorkModel: "uzaktan",
		Eligibility: domain.EligibilityPartlySuitable, ApplicationDueAt: &applicationDue,
		Summary: "Backend odaklı gelişim programı.", Confidence: 0.94,
	}); err != nil {
		t.Fatalf("save acceptance analysis: %v", err)
	}
	if err := repository.RecordSourceFailure(context.Background(),
		"commencis-lever-spring-boot-camp-2026", fetchedAt, "aktiflik manuel doğrulanmalı"); err != nil {
		t.Fatalf("record manual check: %v", err)
	}

	handler := httpapi.NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, repository, nil)
	detailResponse := serveRequest(handler, http.MethodGet, "/api/v1/listings/"+listingID, "")
	if detailResponse.Code != http.StatusOK ||
		!strings.Contains(detailResponse.Body.String(), `"summary":"Backend odaklı gelişim programı."`) {
		t.Fatalf("listing cannot be inspected: status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}

	updateBody := `{"status":"basvuruldu","deadline":"2026-08-12T18:00:00+03:00",` +
		`"interview_at":"2026-08-18T10:30:00+03:00","notes":"Teknik görüşme için Go çalış."}`
	updateResponse := serveRequest(handler, http.MethodPut,
		"/api/v1/listings/"+listingID+"/application", updateBody)
	if updateResponse.Code != http.StatusOK ||
		!strings.Contains(updateResponse.Body.String(), `"status":"basvuruldu"`) {
		t.Fatalf("application cannot be updated: status=%d body=%s", updateResponse.Code, updateResponse.Body.String())
	}

	dashboardResponse := serveRequest(handler, http.MethodGet, "/api/v1/dashboard", "")
	if dashboardResponse.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d body=%s", dashboardResponse.Code, dashboardResponse.Body.String())
	}
	var dashboard store.DashboardSnapshot
	if err := json.Unmarshal(dashboardResponse.Body.Bytes(), &dashboard); err != nil {
		t.Fatalf("decode phase 4 dashboard: %v", err)
	}
	if len(dashboard.ActiveApplications) != 1 ||
		dashboard.ActiveApplications[0].ApplicationStatus != domain.ApplicationSubmitted ||
		dashboard.ActiveApplications[0].TrackingDeadline == nil ||
		len(dashboard.ManualChecks) != 1 {
		t.Fatalf("tracked application or manual check is missing: %#v", dashboard)
	}
}

func serveRequest(handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
