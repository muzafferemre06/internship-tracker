package acceptance_test

import (
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
)

type phase18Diligence struct {
	Phase         string   `json:"phase"`
	VerifiedAt    string   `json:"verified_at"`
	ApprovedBatch []string `json:"approved_batch"`
	Companies     []struct {
		Company                 string   `json:"company"`
		Technopark              string   `json:"technopark"`
		CurrentIdentity         string   `json:"current_identity"`
		OfficialDomain          string   `json:"official_domain"`
		OfficialOpportunityURLs []string `json:"official_opportunity_urls"`
		CurrentOpportunityState string   `json:"current_opportunity_state"`
		StudentSignal           string   `json:"student_signal"`
		AccessObservation       string   `json:"access_observation"`
		AutomationDecision      string   `json:"automation_decision"`
		DecisionRationale       string   `json:"decision_rationale"`
		Phase18Batch            string   `json:"phase_18_batch"`
	} `json:"companies"`
}

func TestPhase18DiligenceCoversEveryPhase17CompanyAndFreezesApprovedBatch(t *testing.T) {
	contents, err := os.ReadFile("../../docs/research/phase-18-company-diligence-2026-08-11.json")
	if err != nil {
		t.Fatalf("read Phase 18 diligence: %v", err)
	}
	var diligence phase18Diligence
	if err := json.Unmarshal(contents, &diligence); err != nil {
		t.Fatalf("decode Phase 18 diligence: %v", err)
	}
	if diligence.Phase != "18" || diligence.VerifiedAt != "2026-08-11" {
		t.Fatalf("invalid diligence identity: phase=%q verified_at=%q", diligence.Phase, diligence.VerifiedAt)
	}

	wantBatch := []string{"Bilişim AŞ", "MobileAction", "Netaş", "SİMSOFT"}
	slices.Sort(diligence.ApprovedBatch)
	if !slices.Equal(diligence.ApprovedBatch, wantBatch) {
		t.Fatalf("approved batch=%v, want %v", diligence.ApprovedBatch, wantBatch)
	}

	wantCompanies := []string{
		"Alictus", "Ankara Bilgi Teknolojileri", "Binalyze", "Bilişim AŞ", "Etiya",
		"Insider", "LOTEC", "MobileAction", "Netaş", "OBSS", "Peaksoft Consulting",
		"SİMSOFT", "T2 Software", "TaleWorlds Entertainment", "Udemy",
	}
	slices.Sort(wantCompanies)
	seen := make([]string, 0, len(diligence.Companies))
	approved := make(map[string]struct{}, len(wantBatch))
	for _, company := range diligence.Companies {
		seen = append(seen, company.Company)
		if strings.TrimSpace(company.Technopark) == "" || strings.TrimSpace(company.CurrentIdentity) == "" ||
			strings.TrimSpace(company.CurrentOpportunityState) == "" || strings.TrimSpace(company.StudentSignal) == "" ||
			strings.TrimSpace(company.AccessObservation) == "" || strings.TrimSpace(company.DecisionRationale) == "" {
			t.Errorf("%s has incomplete diligence: %#v", company.Company, company)
		}
		assertPhase18HTTPSURL(t, company.OfficialDomain, company.Company+" official domain")
		if len(company.OfficialOpportunityURLs) == 0 {
			t.Errorf("%s has no official opportunity evidence", company.Company)
		}
		for _, evidenceURL := range company.OfficialOpportunityURLs {
			assertPhase18HTTPSURL(t, evidenceURL, company.Company+" opportunity evidence")
		}
		switch company.AutomationDecision {
		case "automatic_official_ats", "manual_official", "research_blocked", "defer_unapproved", "retired_identity":
		default:
			t.Errorf("%s has invalid automation decision %q", company.Company, company.AutomationDecision)
		}
		switch company.Phase18Batch {
		case "approved_batch_1":
			approved[company.Company] = struct{}{}
		case "later_batch", "not_eligible":
		default:
			t.Errorf("%s has invalid Phase 18 batch %q", company.Company, company.Phase18Batch)
		}
	}
	slices.Sort(seen)
	if !slices.Equal(seen, wantCompanies) {
		t.Fatalf("researched companies=%v, want %v", seen, wantCompanies)
	}
	for _, company := range wantBatch {
		if _, ok := approved[company]; !ok {
			t.Errorf("approved company %q is not in approved_batch_1", company)
		}
	}
}

func assertPhase18HTTPSURL(t *testing.T, raw, label string) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		t.Errorf("%s must be an absolute HTTPS URL, got %q", label, raw)
	}
}
