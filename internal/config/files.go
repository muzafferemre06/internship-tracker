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
	AccessPolicies []DomainAccessPolicy `json:"access_policies,omitempty"`
	Companies      []CompanyConfig      `json:"companies"`
}

type DomainAccessPolicy struct {
	Domain                 string `json:"domain"`
	Mode                   string `json:"mode"`
	MinimumIntervalSeconds int    `json:"minimum_interval_seconds,omitempty"`
	BaseCooldownSeconds    int    `json:"base_cooldown_seconds,omitempty"`
	MaximumCooldownSeconds int    `json:"maximum_cooldown_seconds,omitempty"`
}

func (c SourcesConfig) ResolveAccessPolicy(rawURL string) (DomainAccessPolicy, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return DomainAccessPolicy{}, false
	}
	hostname := normalizePolicyDomain(parsed.Hostname())
	bestIndex := -1
	bestLength := -1
	for index, policy := range c.AccessPolicies {
		domain := normalizePolicyDomain(policy.Domain)
		if hostname != domain && !strings.HasSuffix(hostname, "."+domain) {
			continue
		}
		if len(domain) > bestLength {
			bestIndex = index
			bestLength = len(domain)
		}
	}
	if bestIndex < 0 {
		return DomainAccessPolicy{}, false
	}
	policy := c.AccessPolicies[bestIndex]
	policy.Domain = normalizePolicyDomain(policy.Domain)
	policy.Mode = strings.ToLower(strings.TrimSpace(policy.Mode))
	return policy, true
}

type CompanyConfig struct {
	Name           string         `json:"name"`
	PriorityGroup  string         `json:"priority_group"`
	TrackingStatus string         `json:"tracking_status,omitempty"`
	Sources        []SourceConfig `json:"sources"`
}

// EffectiveTrackingStatus returns the company's declared tracking status, or
// "active" when unset.
func (c CompanyConfig) EffectiveTrackingStatus() string {
	if status := strings.TrimSpace(c.TrackingStatus); status != "" {
		return status
	}
	return "active"
}

type SourceConfig struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Adapter  string `json:"adapter"`
	Strategy string `json:"strategy,omitempty"`
	PageName string `json:"page_name,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// legacyHTMLAdapters lists the pre-Faz-9 hand-written adapters that default
// to the "legacy_html" strategy when a source does not declare one explicitly.
var legacyHTMLAdapters = map[string]struct{}{
	"kariyer_net": {},
	"lever":       {},
}

// adapterDefaultStrategy maps Faz 10+ adapters to their tier strategy so a
// source only has to declare its adapter; the strategy is inferred (see
// staj-takip-spec-v2.md §16). Explicit "strategy" in config still wins.
var adapterDefaultStrategy = map[string]string{
	"json_ld":          "json_ld",
	"greenhouse":       "ats_api",
	"llm_generic":      "llm_generic",
	"learned_selector": "learned_selector",
}

// validSourceStrategies are the source-strategy tiers defined by Faz 9-12
// (see staj-takip-spec-v2.md §16). "legacy_html" is not part of the target
// enum but is kept as the explicit, documented default for adapters that
// predate the strategy abstraction.
var validSourceStrategies = map[string]struct{}{
	"legacy_html":      {},
	"json_ld":          {},
	"ats_api":          {},
	"learned_selector": {},
	"llm_generic":      {},
	"manual":           {},
}

// EffectiveStrategy returns the source's declared strategy, or an inferred
// default for adapters registered before the strategy field existed.
func (s SourceConfig) EffectiveStrategy() string {
	if strategy := strings.TrimSpace(s.Strategy); strategy != "" {
		return strategy
	}
	if _, ok := legacyHTMLAdapters[s.Adapter]; ok {
		return "legacy_html"
	}
	if strategy, ok := adapterDefaultStrategy[s.Adapter]; ok {
		return strategy
	}
	return ""
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
	policyDomains := make(map[string]struct{}, len(c.AccessPolicies))
	for index, policy := range c.AccessPolicies {
		domain := normalizePolicyDomain(policy.Domain)
		if !validPolicyDomain(policy.Domain, domain) {
			return fmt.Errorf("access_policies[%d] has invalid domain %q", index, policy.Domain)
		}
		if _, exists := policyDomains[domain]; exists {
			return fmt.Errorf("access policy domain %q is defined more than once", domain)
		}
		policyDomains[domain] = struct{}{}
		mode := strings.ToLower(strings.TrimSpace(policy.Mode))
		switch mode {
		case "robots", "public_api":
			if policy.MinimumIntervalSeconds <= 0 || policy.BaseCooldownSeconds <= 0 ||
				policy.MaximumCooldownSeconds < policy.BaseCooldownSeconds {
				return fmt.Errorf("access policy %q durations are invalid", domain)
			}
		case "manual_only":
			if policy.MinimumIntervalSeconds != 0 || policy.BaseCooldownSeconds != 0 || policy.MaximumCooldownSeconds != 0 {
				return fmt.Errorf("manual_only access policy %q durations must be zero", domain)
			}
		default:
			return fmt.Errorf("access policy %q has invalid mode %q", domain, policy.Mode)
		}
	}

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

		switch company.EffectiveTrackingStatus() {
		case "active", "manual", "paused":
		default:
			return fmt.Errorf("company %q has invalid tracking_status %q", name, company.TrackingStatus)
		}

		for sourceIndex, source := range company.Sources {
			if err := source.validate(); err != nil {
				return fmt.Errorf("company %q source %d: %w", name, sourceIndex, err)
			}
			if _, exists := sourceIDs[source.ID]; exists {
				return fmt.Errorf("source id %q is defined more than once", source.ID)
			}
			sourceIDs[source.ID] = struct{}{}
			if policy, found := c.ResolveAccessPolicy(source.URL); found && policy.Mode == "manual_only" {
				if source.Enabled {
					return fmt.Errorf("company %q source %q with manual_only access policy must be disabled", name, source.ID)
				}
				if source.Adapter != "manual" || source.EffectiveStrategy() != "manual" {
					return fmt.Errorf("company %q source %q with manual_only access policy must use the manual adapter and strategy", name, source.ID)
				}
				if company.EffectiveTrackingStatus() != "manual" {
					return fmt.Errorf("company %q source %q with manual_only access policy requires manual tracking", name, source.ID)
				}
			}
		}
	}
	return nil
}

func normalizePolicyDomain(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}

func validPolicyDomain(raw, normalized string) bool {
	if normalized == "" || raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "/:@") {
		return false
	}
	parsed, err := url.Parse("https://" + normalized)
	return err == nil && parsed.Hostname() == normalized && parsed.Port() == "" && parsed.Path == ""
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
	strategy := s.EffectiveStrategy()
	if strategy == "" {
		return fmt.Errorf("strategy is required for adapter %q", s.Adapter)
	}
	if _, ok := validSourceStrategies[strategy]; !ok {
		return fmt.Errorf("unknown strategy %q", strategy)
	}
	return nil
}
