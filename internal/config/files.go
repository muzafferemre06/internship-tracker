package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

type CandidateProfile struct {
	Education           EducationProfile    `json:"education"`
	FocusAreas          []string            `json:"focus_areas"`
	Experience          []ExperienceProfile `json:"experience"`
	LocationPreferences LocationPreferences `json:"location_preferences"`
}

type EducationProfile struct {
	University string  `json:"university"`
	Department string  `json:"department"`
	ClassYear  int     `json:"class_year"`
	GPA        float64 `json:"gpa"`
}

type ExperienceProfile struct {
	Organization string   `json:"organization"`
	Areas        []string `json:"areas"`
}

type LocationPreferences struct {
	Primary                     []string `json:"primary"`
	SummerOtherCities           bool     `json:"summer_other_cities"`
	TermTimePartTimeOtherCities bool     `json:"term_time_part_time_other_cities"`
}

type SourcesConfig struct {
	Companies []CompanyConfig `json:"companies"`
}

type CompanyConfig struct {
	Name          string         `json:"name"`
	PriorityGroup string         `json:"priority_group"`
	Sources       []SourceConfig `json:"sources"`
}

type SourceConfig struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Adapter  string `json:"adapter"`
	PageName string `json:"page_name,omitempty"`
	Enabled  bool   `json:"enabled"`
}

func LoadCandidateProfile(path string) (CandidateProfile, error) {
	var profile CandidateProfile
	if err := decodeJSONFile(path, &profile); err != nil {
		return CandidateProfile{}, fmt.Errorf("load candidate profile: %w", err)
	}
	if err := profile.validate(); err != nil {
		return CandidateProfile{}, fmt.Errorf("validate candidate profile: %w", err)
	}
	return profile, nil
}

func LoadSources(path string) (SourcesConfig, error) {
	var sources SourcesConfig
	if err := decodeJSONFile(path, &sources); err != nil {
		return SourcesConfig{}, fmt.Errorf("load sources: %w", err)
	}
	if err := sources.validate(); err != nil {
		return SourcesConfig{}, fmt.Errorf("validate sources: %w", err)
	}
	return sources, nil
}

func decodeJSONFile(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func (p CandidateProfile) validate() error {
	if strings.TrimSpace(p.Education.University) == "" {
		return errors.New("education.university is required")
	}
	if strings.TrimSpace(p.Education.Department) == "" {
		return errors.New("education.department is required")
	}
	if p.Education.ClassYear < 1 {
		return errors.New("education.class_year must be positive")
	}
	if p.Education.GPA < 0 || p.Education.GPA > 4 {
		return errors.New("education.gpa must be between 0 and 4")
	}
	if len(p.FocusAreas) == 0 {
		return errors.New("focus_areas must not be empty")
	}
	return nil
}

func (c SourcesConfig) validate() error {
	if len(c.Companies) == 0 {
		return errors.New("companies must not be empty")
	}

	companyNames := make(map[string]struct{}, len(c.Companies))
	sourceIDs := make(map[string]struct{})
	for companyIndex, company := range c.Companies {
		name := strings.TrimSpace(company.Name)
		if name == "" {
			return fmt.Errorf("companies[%d].name is required", companyIndex)
		}
		if _, exists := companyNames[name]; exists {
			return fmt.Errorf("company %q is defined more than once", name)
		}
		companyNames[name] = struct{}{}

		switch company.PriorityGroup {
		case "primary", "secondary", "candidate":
		default:
			return fmt.Errorf("company %q has invalid priority_group %q", name, company.PriorityGroup)
		}

		for sourceIndex, source := range company.Sources {
			if err := source.validate(); err != nil {
				return fmt.Errorf("company %q source %d: %w", name, sourceIndex, err)
			}
			if _, exists := sourceIDs[source.ID]; exists {
				return fmt.Errorf("source id %q is defined more than once", source.ID)
			}
			sourceIDs[source.ID] = struct{}{}
		}
	}
	return nil
}

func (s SourceConfig) validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(s.Type) == "" {
		return errors.New("type is required")
	}
	if strings.TrimSpace(s.Adapter) == "" {
		return errors.New("adapter is required")
	}
	parsedURL, err := url.ParseRequestURI(s.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return errors.New("url must be an absolute HTTP(S) URL")
	}
	return nil
}
