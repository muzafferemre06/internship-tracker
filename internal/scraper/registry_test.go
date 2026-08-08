package scraper

import "testing"

func TestNewSourceDispatchesRegisteredAdapters(t *testing.T) {
	cases := []struct {
		adapter string
		spec    SourceSpec
	}{
		{
			adapter: "kariyer_net",
			spec: SourceSpec{
				ID:      "meteksan-kariyer-net",
				Company: "Meteksan",
				URL:     "https://www.kariyer.net/firma-profil/meteksan",
			},
		},
		{
			adapter: "lever",
			spec: SourceSpec{
				ID:      "commencis-lever",
				Company: "Commencis",
				URL:     "https://jobs.lever.co/commencis/04a5cd98-ab26-4b48-bb64-3397ffe79a55",
			},
		},
		{
			adapter: "json_ld",
			spec: SourceSpec{
				ID:      "northstar-careers",
				Company: "Northstar Robotics",
				URL:     "https://careers.northstar.example/",
			},
		},
		{
			adapter: "greenhouse",
			spec: SourceSpec{
				ID:      "acme-greenhouse",
				Company: "Acme Robotics",
				URL:     "https://boards-api.greenhouse.io/v1/boards/acmerobotics/jobs",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.adapter, func(t *testing.T) {
			if !SupportsAdapter(testCase.adapter) {
				t.Fatalf("expected %q to be a supported adapter", testCase.adapter)
			}
			source, err := NewSource(testCase.adapter, testCase.spec)
			if err != nil {
				t.Fatalf("build source: %v", err)
			}
			if source.Name() != testCase.spec.ID {
				t.Fatalf("expected source name %q, got %q", testCase.spec.ID, source.Name())
			}
		})
	}
}

func TestNewSourceRejectsUnknownAdapter(t *testing.T) {
	if SupportsAdapter("does_not_exist") {
		t.Fatalf("expected unknown adapter to be unsupported")
	}
	if _, err := NewSource("does_not_exist", SourceSpec{ID: "x", Company: "X", URL: "https://example.test"}); err == nil {
		t.Fatalf("expected error for unknown adapter")
	}
}
