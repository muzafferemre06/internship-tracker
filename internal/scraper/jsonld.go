package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"golang.org/x/net/html"
)

const maxJSONLDPageBytes = 4 << 20

// JSONLDSource implements the Faz 10 deterministic, AI-free "json_ld" strategy:
// it reads schema.org JobPosting objects embedded as
// <script type="application/ld+json"> on a career page. Google for Jobs pushes
// many sites to publish this standard structure, so it is portable and resilient
// to layout changes (see staj-takip-spec-v2.md §16, Faz 10).
type JSONLDSource struct {
	name    string
	company string
	pageURL *url.URL
	client  *http.Client
	now     func() time.Time
}

func NewJSONLDSource(name string, company string, pageURL string, client *http.Client) (*JSONLDSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	parsedURL, err := url.Parse(pageURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("page URL must be an absolute HTTP(S) URL")
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &JSONLDSource{name: name, company: company, pageURL: parsedURL, client: client, now: time.Now}, nil
}

func (s *JSONLDSource) Name() string { return s.name }

func (s *JSONLDSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope:           s.pageURL.Hostname(),
		MinimumInterval: time.Second,
		BaseCooldown:    time.Minute,
		MaximumCooldown: time.Hour,
	}
}

func (s *JSONLDSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.pageURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create JSON-LD request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch JSON-LD page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch JSON-LD page: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxJSONLDPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JSON-LD page: %w", err)
	}
	if len(body) > maxJSONLDPageBytes {
		return nil, fmt.Errorf("read JSON-LD page: response exceeds %d bytes", maxJSONLDPageBytes)
	}
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse JSON-LD page: %w", err)
	}
	return s.parseListings(root)
}

func (s *JSONLDSource) parseListings(root *html.Node) ([]domain.RawListing, error) {
	fetchedAt := s.now().UTC()
	listings := make([]domain.RawListing, 0)
	seen := make(map[string]struct{})

	blocks := jsonLDScriptContents(root)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%w: page has no application/ld+json blocks", ErrUnexpectedPage)
	}

	for _, block := range blocks {
		var value any
		if err := json.Unmarshal([]byte(block), &value); err != nil {
			return nil, fmt.Errorf("%w: malformed application/ld+json block: %v", ErrUnexpectedPage, err)
		}
		for _, node := range flattenJSONLD(value) {
			if !isJobPosting(node) {
				continue
			}
			listing, err := s.normalizeJobPosting(node, fetchedAt)
			if err != nil {
				return nil, err
			}
			if _, exists := seen[listing.URL]; exists {
				continue
			}
			seen[listing.URL] = struct{}{}
			listings = append(listings, listing)
		}
	}

	if len(listings) == 0 {
		return nil, fmt.Errorf("%w: no schema.org JobPosting found", ErrUnexpectedPage)
	}
	return listings, nil
}

func (s *JSONLDSource) normalizeJobPosting(node map[string]any, fetchedAt time.Time) (domain.RawListing, error) {
	title := strings.TrimSpace(jsonLDString(node["title"]))
	if title == "" {
		return domain.RawListing{}, fmt.Errorf("%w: JobPosting has no title", ErrUnexpectedPage)
	}

	listingURL := s.pageURL.String()
	if raw := strings.TrimSpace(jsonLDString(node["url"])); raw != "" {
		if parsed, err := s.pageURL.Parse(raw); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			parsed.Fragment = ""
			parsed.RawQuery = ""
			listingURL = parsed.String()
		}
	}

	parts := []string{title}
	if employment := jsonLDStrings(node["employmentType"]); len(employment) > 0 {
		parts = append(parts, strings.Join(employment, ", "))
	}
	if location := jobPostingLocality(node["jobLocation"]); location != "" {
		parts = append(parts, location)
	}
	if posted := strings.TrimSpace(jsonLDString(node["datePosted"])); posted != "" {
		parts = append(parts, "Posted: "+posted)
	}
	if valid := strings.TrimSpace(jsonLDString(node["validThrough"])); valid != "" {
		parts = append(parts, "Valid through: "+valid)
	}
	if description := htmlToText(jsonLDString(node["description"])); description != "" {
		parts = append(parts, description)
	}

	return domain.RawListing{
		Company:   s.company,
		SourceID:  s.name,
		Title:     title,
		URL:       listingURL,
		RawText:   strings.Join(parts, "\n"),
		FetchedAt: fetchedAt,
	}, nil
}

// jsonLDScriptContents returns the raw text of every
// <script type="application/ld+json"> element in document order.
func jsonLDScriptContents(root *html.Node) []string {
	blocks := make([]string, 0)
	for _, node := range elements(root, func(node *html.Node) bool {
		return node.Data == "script" && strings.EqualFold(scriptType(node), "application/ld+json")
	}) {
		if text := scriptRawText(node); strings.TrimSpace(text) != "" {
			blocks = append(blocks, text)
		}
	}
	return blocks
}

func scriptType(node *html.Node) string {
	value, _ := attribute(node, "type")
	return strings.TrimSpace(value)
}

func scriptRawText(node *html.Node) string {
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			builder.WriteString(child.Data)
		}
	}
	return builder.String()
}

// flattenJSONLD walks a decoded JSON-LD value, expanding arrays and @graph
// containers, and returns every object node it contains.
func flattenJSONLD(value any) []map[string]any {
	nodes := make([]map[string]any, 0)
	switch typed := value.(type) {
	case map[string]any:
		nodes = append(nodes, typed)
		if graph, ok := typed["@graph"]; ok {
			nodes = append(nodes, flattenJSONLD(graph)...)
		}
	case []any:
		for _, item := range typed {
			nodes = append(nodes, flattenJSONLD(item)...)
		}
	}
	return nodes
}

func isJobPosting(node map[string]any) bool {
	for _, t := range jsonLDStrings(node["@type"]) {
		if strings.EqualFold(t, "JobPosting") {
			return true
		}
	}
	return false
}

// jsonLDString coerces a JSON-LD scalar (string or number) to a trimmed string.
func jsonLDString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%v", typed)
	}
	return ""
}

// jsonLDStrings coerces a JSON-LD field that may be a scalar or an array of
// scalars into a slice of non-empty strings.
func jsonLDStrings(value any) []string {
	result := make([]string, 0)
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if s := strings.TrimSpace(jsonLDString(item)); s != "" {
				result = append(result, s)
			}
		}
	default:
		if s := strings.TrimSpace(jsonLDString(value)); s != "" {
			result = append(result, s)
		}
	}
	return result
}

// jobPostingLocality extracts a human-readable locality from a JobPosting
// jobLocation, which may be a single Place or an array of them.
func jobPostingLocality(value any) string {
	localities := make([]string, 0)
	for _, place := range flattenPlaces(value) {
		address, ok := place["address"].(map[string]any)
		if !ok {
			continue
		}
		if locality := strings.TrimSpace(jsonLDString(address["addressLocality"])); locality != "" {
			localities = append(localities, locality)
		}
	}
	return strings.Join(localities, ", ")
}

func flattenPlaces(value any) []map[string]any {
	places := make([]map[string]any, 0)
	switch typed := value.(type) {
	case map[string]any:
		places = append(places, typed)
	case []any:
		for _, item := range typed {
			if place, ok := item.(map[string]any); ok {
				places = append(places, place)
			}
		}
	}
	return places
}
