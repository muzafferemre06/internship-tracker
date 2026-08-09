package opportunity

import "testing"

func TestNormalizeTitleHandlesTurkishAndInternshipVariants(t *testing.T) {
	tests := map[string]string{
		"Yazılım Geliştirme Stajyeri":   "yazilim gelistirme staj",
		"  YAZILIM—GELİŞTİRME / STAJI ": "yazilim gelistirme staj",
		"Backend Engineer Internship":   "backend engineer staj",
	}
	for input, want := range tests {
		if got := NormalizeTitle(input); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEvaluateUsesConservativeTitleAndLocationThresholds(t *testing.T) {
	tests := []struct {
		name       string
		incoming   Identity
		existing   Identity
		want       Outcome
		wantReason string
	}{
		{
			name:     "normalized exact and same location merges",
			incoming: Identity{Title: "Yazılım Geliştirme Stajyeri", Location: "İstanbul, Türkiye"},
			existing: Identity{Title: "YAZILIM GELISTIRME STAJI", Location: "Istanbul"},
			want:     AutoMerge, wantReason: "exact_title_location_match",
		},
		{
			name:     "exact title with both locations absent merges",
			incoming: Identity{Title: "Backend Stajı"},
			existing: Identity{Title: "Backend Stajyeri"},
			want:     AutoMerge, wantReason: "exact_title_locations_absent",
		},
		{
			name:       "high confidence fuzzy title and same location merges",
			incoming:   Identity{Title: "Yazılım Geliştirme Stajj", Location: "Ankara"},
			existing:   Identity{Title: "Yazılım Geliştirme Staj", Location: "Ankara"},
			want:       AutoMerge,
			wantReason: "fuzzy_title_location_match",
		},
		{
			name:       "middle confidence fuzzy title stays ambiguous",
			incoming:   Identity{Title: "Yazılım Geliştirme Staj", Location: "Ankara"},
			existing:   Identity{Title: "Yazılım Gelişme Staj", Location: "Ankara"},
			want:       Ambiguous,
			wantReason: "title_similarity_uncertain",
		},
		{
			name:     "one missing location stays ambiguous",
			incoming: Identity{Title: "Backend Stajı", Location: "Ankara"},
			existing: Identity{Title: "Backend Stajyeri"},
			want:     Ambiguous, wantReason: "location_missing",
		},
		{
			name:     "conflicting locations never merge",
			incoming: Identity{Title: "Backend Stajı", Location: "Ankara"},
			existing: Identity{Title: "Backend Stajyeri", Location: "İstanbul"},
			want:     Separate, wantReason: "location_conflict",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Evaluate(test.incoming, test.existing)
			if got.Outcome != test.want || got.Reason != test.wantReason {
				t.Fatalf("unexpected decision: %#v", got)
			}
		})
	}
}

func TestClassifyScoreBoundariesKeepUncertainMatchesSeparate(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		location LocationRelation
		want     Outcome
	}{
		{name: "auto threshold", score: 0.92, location: LocationCompatible, want: AutoMerge},
		{name: "below auto", score: 0.9199, location: LocationCompatible, want: Ambiguous},
		{name: "ambiguous threshold", score: 0.80, location: LocationCompatible, want: Ambiguous},
		{name: "below ambiguous", score: 0.7999, location: LocationCompatible, want: Separate},
		{name: "high score missing location", score: 0.99, location: LocationIncomplete, want: Ambiguous},
		{name: "high score conflicting location", score: 0.99, location: LocationConflict, want: Separate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, _ := classify(test.score, false, test.location); got != test.want {
				t.Fatalf("classify(%v, %q) = %q, want %q", test.score, test.location, got, test.want)
			}
		})
	}
}
