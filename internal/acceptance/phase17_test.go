package acceptance_test

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
)

type phase17Research struct {
	Phase      string `json:"phase"`
	VerifiedAt string `json:"verified_at"`
	Parks      []struct {
		Name                 string `json:"name"`
		OfficialDirectoryURL string `json:"official_directory_url"`
		Candidates           []struct {
			Company               string `json:"company"`
			OfficialDomain        string `json:"official_domain"`
			Focus                 string `json:"focus"`
			TechnoparkEvidenceURL string `json:"technopark_evidence_url"`
			OpportunitySignalURL  string `json:"opportunity_signal_url"`
			OpportunitySignal     string `json:"opportunity_signal"`
			SourceType            string `json:"source_type"`
			Access                string `json:"access"`
			Status                string `json:"status"`
			Rationale             string `json:"rationale"`
		} `json:"candidates"`
	} `json:"parks"`
}

func TestPhase17ResearchCatalogIsFiniteEvidenceBackedAndRuntimeIsolated(t *testing.T) {
	contents, err := os.ReadFile("../../docs/research/phase-17-candidates-2026-08-11.json")
	if err != nil {
		t.Fatalf("read Phase 17 research catalog: %v", err)
	}
	var research phase17Research
	if err := json.Unmarshal(contents, &research); err != nil {
		t.Fatalf("decode Phase 17 research catalog: %v", err)
	}
	if research.Phase != "17" || research.VerifiedAt == "" {
		t.Fatalf("invalid research identity: %#v", research)
	}
	wantParks := map[string]bool{"Bilkent CYBERPARK": false, "ODTÜ TEKNOKENT": false, "Hacettepe Teknokent": false}
	seenCompanies := make(map[string]struct{})
	seenDomains := make(map[string]struct{})
	for _, park := range research.Parks {
		if _, ok := wantParks[park.Name]; !ok {
			t.Errorf("unexpected technopark %q", park.Name)
			continue
		}
		wantParks[park.Name] = true
		assertPhase17HTTPSURL(t, park.OfficialDirectoryURL, park.Name+" directory")
		if len(park.Candidates) == 0 || len(park.Candidates) > 20 {
			t.Errorf("%s candidate count=%d, want 1..20", park.Name, len(park.Candidates))
		}
		for _, candidate := range park.Candidates {
			if strings.TrimSpace(candidate.Company) == "" || strings.TrimSpace(candidate.Focus) == "" ||
				strings.TrimSpace(candidate.OpportunitySignal) == "" || strings.TrimSpace(candidate.Rationale) == "" {
				t.Errorf("%s has incomplete candidate: %#v", park.Name, candidate)
			}
			if _, duplicate := seenCompanies[candidate.Company]; duplicate {
				t.Errorf("duplicate company %q", candidate.Company)
			}
			seenCompanies[candidate.Company] = struct{}{}
			assertPhase17HTTPSURL(t, candidate.OfficialDomain, candidate.Company+" official domain")
			assertPhase17HTTPSURL(t, candidate.TechnoparkEvidenceURL, candidate.Company+" technopark evidence")
			assertPhase17HTTPSURL(t, candidate.OpportunitySignalURL, candidate.Company+" opportunity signal")
			domainURL, _ := url.Parse(candidate.OfficialDomain)
			domain := strings.ToLower(domainURL.Hostname())
			if _, duplicate := seenDomains[domain]; duplicate {
				t.Errorf("duplicate official domain %q", domain)
			}
			seenDomains[domain] = struct{}{}
			switch candidate.Status {
			case "önerilen", "düşük_sinyal", "kimlik_belirsiz", "erişim_manuel":
			default:
				t.Errorf("%s has invalid status %q", candidate.Company, candidate.Status)
			}
			switch candidate.Access {
			case "public_official", "public_ats", "manual_third_party", "manual_no_source", "unverified":
			default:
				t.Errorf("%s has invalid access %q", candidate.Company, candidate.Access)
			}
		}
	}
	for park, found := range wantParks {
		if !found {
			t.Errorf("required technopark %q is missing", park)
		}
	}

	production, err := os.ReadFile("../../configs/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	phase18Approved := map[string]struct{}{
		"MobileAction": {}, "SİMSOFT": {}, "Netaş": {}, "Bilişim AŞ": {},
	}
	for company := range seenCompanies {
		if _, approved := phase18Approved[company]; approved {
			continue
		}
		if strings.Contains(string(production), `"name": "`+company+`"`) {
			t.Errorf("unapproved Phase 17 company %q leaked into production sources", company)
		}
	}
}

func assertPhase17HTTPSURL(t *testing.T, raw, label string) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		t.Errorf("%s must be an absolute HTTPS URL, got %q", label, raw)
	}
}
