package acceptance_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/config"
)

func TestPhase18Batch4CompletesApprovedCatalogAndExcludesRetiredAlictusIdentity(t *testing.T) {
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	for _, companyName := range []string{"Ankara Bilgi Teknolojileri", "Peaksoft Consulting"} {
		company := phase18Company(t, configured, companyName)
		if company.PriorityGroup != "secondary" || company.EffectiveTrackingStatus() != "manual" || len(company.Sources) != 1 {
			t.Fatalf("%s must be one manual secondary source: %#v", companyName, company)
		}
		source := company.Sources[0]
		if source.Adapter != "manual" || source.EffectiveStrategy() != "manual" || source.EffectiveCoverageStatus() != "manual" ||
			source.Enabled || source.EffectiveTrustLevel() != "official_company" || source.CoverageReason == "" ||
			source.CoverageReasonCode == "" || source.LastVerifiedAt == "" {
			t.Errorf("%s source lacks manual evidence: %#v", companyName, source)
		}
		policy, found := configured.ResolveAccessPolicy(source.URL)
		if !found || policy.Mode != "manual_only" {
			t.Errorf("%s policy=%#v found=%v, want manual_only", companyName, policy, found)
		}
	}
	for _, company := range configured.Companies {
		if company.Name == "Alictus" {
			t.Fatal("retired Alictus identity must not be added to production")
		}
	}

	researchBytes, err := os.ReadFile("../../docs/research/phase-18-company-diligence-2026-08-11.json")
	if err != nil {
		t.Fatal(err)
	}
	var research struct {
		Companies []struct {
			Company string `json:"company"`
			Batch   string `json:"phase_18_batch"`
		} `json:"companies"`
	}
	if err := json.Unmarshal(researchBytes, &research); err != nil {
		t.Fatal(err)
	}
	accounted := 0
	for _, candidate := range research.Companies {
		if candidate.Company == "Alictus" {
			if candidate.Batch != "not_eligible" {
				t.Fatalf("Alictus decision=%q, want not_eligible", candidate.Batch)
			}
			accounted++
			continue
		}
		phase18Company(t, configured, candidate.Company)
		accounted++
	}
	if accounted != 15 {
		t.Fatalf("accounted Phase 18 candidates=%d, want 15", accounted)
	}
}
