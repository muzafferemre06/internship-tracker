package acceptance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/push"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type suitableAnalyzer struct{}

func (suitableAnalyzer) Analyze(context.Context, domain.RawListing, analyzer.CandidateProfile) (domain.ListingAnalysis, error) {
	return domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		Eligibility: domain.EligibilitySuitable, Summary: "Uygun backend stajı", Confidence: 0.95,
	}, nil
}

type acceptancePushSender struct {
	messages []push.Message
}

func (s *acceptancePushSender) Send(_ context.Context, _ store.PushSubscription, message push.Message) (push.SendResult, error) {
	s.messages = append(s.messages, message)
	return push.SendResult{StatusCode: http.StatusCreated}, nil
}

func TestPhase5FixtureQueuesAndDispatchesOneDeepLinkedPush(t *testing.T) {
	fixture, err := os.ReadFile("../scraper/testdata/lever/commencis-posting.html")
	if err != nil {
		t.Fatalf("read Lever fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := scraper.NewLeverSource("commencis-lever-spring-boot-camp-2026", "Commencis", commencisURL, client)
	if err != nil {
		t.Fatal(err)
	}
	db, repository := openRepository(t)
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "commencis-lever-spring-boot-camp-2026", Company: "Commencis", PriorityGroup: "primary",
		Type: "official_ats_posting", URL: commencisURL, Adapter: "lever", Enabled: true, TrustLevel: "official_ats",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpsertPushSubscription(context.Background(), store.PushSubscriptionInput{
		Endpoint: "https://push.example.test/phase5", P256DH: "fixture-key", Auth: "fixture-auth",
	}); err != nil {
		t.Fatal(err)
	}
	service := &orchestrator.Service{
		Sources: []scraper.Source{source}, Analyzer: suitableAnalyzer{}, Store: repository,
	}
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }
	first, err := service.Run(context.Background(), "scheduled")
	if err != nil || first.Sources[0].New != 1 {
		t.Fatalf("first fixture scan: result=%#v err=%v", first, err)
	}
	sender := &acceptancePushSender{}
	dispatcher, err := push.NewDispatcher(repository, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.DispatchPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	second, err := service.Run(context.Background(), "scheduled")
	if err != nil || second.Sources[0].New != 0 {
		t.Fatalf("second fixture scan: result=%#v err=%v", second, err)
	}
	if err := dispatcher.DispatchPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("expected one push across two scans, got %d", len(sender.messages))
	}
	var payload map[string]string
	if err := json.Unmarshal(sender.messages[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["title"] != "Yeni uygun staj ilanı" || !strings.HasPrefix(payload["url"], "/?listing=") || strings.Contains(payload["body"], "3.97") {
		t.Fatalf("unsafe or incorrect push payload: %#v", payload)
	}
	var events, deliveries int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notification_deliveries WHERE status = 'sent'").Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if events != 1 || deliveries != 1 {
		t.Fatalf("unexpected persisted notification state: events=%d sent=%d", events, deliveries)
	}
}
