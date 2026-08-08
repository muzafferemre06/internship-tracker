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
	if len(sources.Companies) != 6 || sources.Companies[0].Name != "Commencis" ||
		sources.Companies[0].Sources[0].Adapter != "lever" {
		t.Fatalf("unexpected sources: %#v", sources)
	}
	if len(sources.Companies[2].Sources) != 2 || sources.Companies[2].Sources[1].PageName != "Aselsannet" {
		t.Fatalf("expected ASELSAN multi-profile configuration, got %#v", sources.Companies[2])
	}
}

func TestLoadSourcesInfersLegacyHTMLStrategy(t *testing.T) {
	sources, err := LoadSources("../../configs/sources.example.json")
	if err != nil {
		t.Fatalf("load example sources: %v", err)
	}
	for _, company := range sources.Companies {
		for _, source := range company.Sources {
			if strategy := source.EffectiveStrategy(); strategy != "legacy_html" {
				t.Fatalf("expected source %q to infer legacy_html strategy, got %q", source.ID, strategy)
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

func writeConfigTestFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
