package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCandidateProfile(t *testing.T) {
	profile, err := LoadCandidateProfile("../../configs/candidate-profile.example.json")
	if err != nil {
		t.Fatalf("load example profile: %v", err)
	}
	if profile.Education.ClassYear != 2 || profile.Education.GPA != 3.97 {
		t.Fatalf("unexpected education profile: %#v", profile.Education)
	}
}

func TestLoadCandidateProfileRejectsUnknownFields(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"education": {"university":"Test", "department":"CTIS", "class_year":2, "gpa":3.5},
		"focus_areas":["backend"],
		"experience":[],
		"location_preferences":{"primary":["Ankara"], "summer_other_cities":true, "term_time_part_time_other_cities":false},
		"direct_identity":"must not be accepted"
	}`)

	_, err := LoadCandidateProfile(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadSources(t *testing.T) {
	sources, err := LoadSources("../../configs/sources.example.json")
	if err != nil {
		t.Fatalf("load example sources: %v", err)
	}
	if len(sources.Companies) != 9 || sources.Companies[0].Name != "Commencis" ||
		sources.Companies[0].Sources[0].Adapter != "lever" {
		t.Fatalf("unexpected sources: %#v", sources)
	}
	if len(sources.Companies[2].Sources) != 2 || sources.Companies[2].Sources[1].PageName != "Aselsannet" {
		t.Fatalf("expected ASELSAN multi-profile configuration, got %#v", sources.Companies[2])
	}
}

func TestLoadSourcesInfersStrategyFromAdapter(t *testing.T) {
	sources, err := LoadSources("../../configs/sources.example.json")
	if err != nil {
		t.Fatalf("load example sources: %v", err)
	}
	// Adapter -> inferred strategy when the source omits an explicit "strategy".
	want := map[string]string{
		"lever":       "legacy_html",
		"kariyer_net": "legacy_html",
		"json_ld":     "json_ld",
		"greenhouse":  "ats_api",
		"llm_generic": "llm_generic",
	}
	for _, company := range sources.Companies {
		for _, source := range company.Sources {
			expected, ok := want[source.Adapter]
			if !ok {
				t.Fatalf("example source %q uses undocumented adapter %q", source.ID, source.Adapter)
			}
			if strategy := source.EffectiveStrategy(); strategy != expected {
				t.Fatalf("source %q: expected inferred strategy %q, got %q", source.ID, expected, strategy)
			}
		}
	}
}

func TestLoadSourcesRejectsUnknownStrategy(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"companies":[{
			"name":"Test", "priority_group":"primary",
			"sources":[{"id":"test", "type":"career_page", "url":"https://example.test/careers", "adapter":"kariyer_net", "strategy":"not_a_real_strategy", "enabled":true}]
		}]
	}`)

	_, err := LoadSources(path)
	if err == nil || !strings.Contains(err.Error(), "unknown strategy") {
		t.Fatalf("expected unknown strategy error, got %v", err)
	}
}

func TestLoadSourcesInfersLearnedSelectorStrategy(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"companies":[{
			"name":"Test", "priority_group":"candidate",
			"sources":[{"id":"test", "type":"career_page", "url":"https://example.test/careers", "adapter":"learned_selector", "enabled":true}]
		}]
	}`)
	sources, err := LoadSources(path)
	if err != nil {
		t.Fatalf("load learned selector source: %v", err)
	}
	if got := sources.Companies[0].Sources[0].EffectiveStrategy(); got != "learned_selector" {
		t.Fatalf("expected learned_selector strategy, got %q", got)
	}
}

func TestLoadSourcesInfersActiveTrackingStatus(t *testing.T) {
	sources, err := LoadSources("../../configs/sources.example.json")
	if err != nil {
		t.Fatalf("load example sources: %v", err)
	}
	for _, company := range sources.Companies {
		if status := company.EffectiveTrackingStatus(); status != "active" {
			t.Fatalf("expected company %q to default to active tracking status, got %q", company.Name, status)
		}
	}
}

func TestLoadSourcesAcceptsManualTrackingStatus(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"companies":[{
			"name":"Test", "priority_group":"primary", "tracking_status":"manual",
			"sources":[{"id":"test", "type":"career_page", "url":"https://example.test/careers", "adapter":"manual", "strategy":"manual", "enabled":false}]
		}]
	}`)

	sources, err := LoadSources(path)
	if err != nil {
		t.Fatalf("load manual sources: %v", err)
	}
	if got := sources.Companies[0].EffectiveTrackingStatus(); got != "manual" {
		t.Fatalf("expected manual tracking status, got %q", got)
	}
}

func TestLoadSourcesAcceptsProgramWindow(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"access_policies":[{"domain":"example.test","mode":"manual_only"}],
		"companies":[{
			"name":"Turkcell", "priority_group":"primary", "tracking_status":"manual",
			"sources":[{"id":"turkcell-program","type":"program_page","url":"https://example.test/program","adapter":"manual","strategy":"manual","enabled":false}],
			"programs":[{"id":"gncytnk-staj","name":"GNÇYTNK Staj","type":"internship","url":"https://example.test/apply","status":"closed","opens_at":"2026-01-01T00:00:00Z","closes_at":"2026-03-01T00:00:00Z"}]
		}]
	}`)
	sources, err := LoadSources(path)
	if err != nil {
		t.Fatalf("load program window: %v", err)
	}
	program := sources.Companies[0].Programs[0]
	if program.ID != "gncytnk-staj" || program.Status != "closed" || program.Type != "internship" {
		t.Fatalf("unexpected program window: %#v", program)
	}
}

func TestLoadSourcesRejectsInvalidProgramWindow(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"companies":[{
			"name":"Turkcell", "priority_group":"primary", "tracking_status":"manual",
			"sources":[{"id":"turkcell-program","type":"program_page","url":"https://example.test/program","adapter":"manual","strategy":"manual","enabled":false}],
			"programs":[{"id":"gncytnk-staj","name":"GNÇYTNK Staj","type":"internship","url":"https://example.test/apply","status":"sometimes"}]
		}]
	}`)
	_, err := LoadSources(path)
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("expected invalid program status error, got %v", err)
	}
}

func TestLoadSourcesRejectsInvalidTrackingStatus(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"companies":[{
			"name":"Test", "priority_group":"primary", "tracking_status":"not_a_real_status",
			"sources":[{"id":"test", "type":"career_page", "url":"https://example.test/careers", "adapter":"manual", "strategy":"manual", "enabled":false}]
		}]
	}`)

	_, err := LoadSources(path)
	if err == nil || !strings.Contains(err.Error(), "invalid tracking_status") {
		t.Fatalf("expected invalid tracking_status error, got %v", err)
	}
}

func TestLoadSourcesRejectsInvalidURL(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"companies":[{
			"name":"Test", "priority_group":"primary",
			"sources":[{"id":"test", "type":"career_page", "url":"not-a-url", "adapter":"fixture", "enabled":true}]
		}]
	}`)

	_, err := LoadSources(path)
	if err == nil || !strings.Contains(err.Error(), "absolute HTTP(S) URL") {
		t.Fatalf("expected URL validation error, got %v", err)
	}
}

func TestLoadSourcesResolvesMostSpecificDomainAccessPolicy(t *testing.T) {
	path := writeConfigTestFile(t, `{
		"access_policies":[
			{"domain":"example.test","mode":"robots","minimum_interval_seconds":10,"base_cooldown_seconds":60,"maximum_cooldown_seconds":3600},
			{"domain":"careers.example.test","mode":"robots","minimum_interval_seconds":2,"base_cooldown_seconds":30,"maximum_cooldown_seconds":300}
		],
		"companies":[{
			"name":"Test", "priority_group":"candidate",
			"sources":[{"id":"test","type":"career_page","url":"https://jobs.careers.example.test/openings","adapter":"json_ld","enabled":true}]
		}]
	}`)

	sources, err := LoadSources(path)
	if err != nil {
		t.Fatalf("load domain access policies: %v", err)
	}
	policy, found := sources.ResolveAccessPolicy("https://jobs.careers.example.test/openings")
	if !found || policy.Domain != "careers.example.test" || policy.Mode != "robots" ||
		policy.MinimumIntervalSeconds != 2 || policy.BaseCooldownSeconds != 30 || policy.MaximumCooldownSeconds != 300 {
		t.Fatalf("unexpected resolved policy: found=%v policy=%#v", found, policy)
	}
}

func TestLoadSourcesValidatesManualOnlySocialPolicy(t *testing.T) {
	valid := writeConfigTestFile(t, `{
		"access_policies":[{"domain":"linkedin.com","mode":"manual_only"}],
		"companies":[{
			"name":"Havelsan", "priority_group":"primary", "tracking_status":"manual",
			"sources":[{"id":"havelsan-linkedin","type":"social_profile","url":"https://www.linkedin.com/company/havelsan/jobs/","adapter":"manual","strategy":"manual","enabled":false}]
		}]
	}`)
	if _, err := LoadSources(valid); err != nil {
		t.Fatalf("valid manual-only social source was rejected: %v", err)
	}

	tests := []struct {
		name    string
		company string
		source  string
		want    string
	}{
		{name: "enabled", company: `"tracking_status":"manual",`, source: `"adapter":"manual","strategy":"manual","enabled":true`, want: "must be disabled"},
		{name: "automatic adapter", company: `"tracking_status":"manual",`, source: `"adapter":"json_ld","strategy":"manual","enabled":false`, want: "manual adapter"},
		{name: "active company", company: ``, source: `"adapter":"manual","strategy":"manual","enabled":false`, want: "manual tracking"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfigTestFile(t, `{
				"access_policies":[{"domain":"linkedin.com","mode":"manual_only"}],
				"companies":[{
					"name":"Test", "priority_group":"primary", `+test.company+`
					"sources":[{"id":"social","type":"social_profile","url":"https://linkedin.com/company/test/jobs/",`+test.source+`}]
				}]
			}`)
			_, err := LoadSources(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q validation error, got %v", test.want, err)
			}
		})
	}
}

func TestLoadSourcesRejectsInvalidDomainAccessPolicies(t *testing.T) {
	tests := []struct {
		name     string
		policies string
		want     string
	}{
		{name: "duplicate", policies: `[{"domain":"example.test","mode":"robots","minimum_interval_seconds":1,"base_cooldown_seconds":1,"maximum_cooldown_seconds":2},{"domain":"EXAMPLE.TEST","mode":"robots","minimum_interval_seconds":1,"base_cooldown_seconds":1,"maximum_cooldown_seconds":2}]`, want: "defined more than once"},
		{name: "unknown mode", policies: `[{"domain":"example.test","mode":"bypass","minimum_interval_seconds":1,"base_cooldown_seconds":1,"maximum_cooldown_seconds":2}]`, want: "invalid mode"},
		{name: "bad durations", policies: `[{"domain":"example.test","mode":"robots","minimum_interval_seconds":0,"base_cooldown_seconds":60,"maximum_cooldown_seconds":30}]`, want: "durations"},
		{name: "url instead of domain", policies: `[{"domain":"https://example.test","mode":"manual_only"}]`, want: "domain"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfigTestFile(t, `{
				"access_policies":`+test.policies+`,
				"companies":[{"name":"Test","priority_group":"candidate","sources":[{"id":"test","type":"career_page","url":"https://other.test/jobs","adapter":"json_ld","enabled":false}]}]
			}`)
			_, err := LoadSources(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q validation error, got %v", test.want, err)
			}
		})
	}
}

func writeConfigTestFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
