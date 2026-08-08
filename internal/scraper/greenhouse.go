package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

const maxGreenhouseBodyBytes = 8 << 20

// GreenhouseSource implements the Faz 10 "ats_api" strategy for Greenhouse's
// public job board API (boards-api.greenhouse.io). It consumes structured JSON
// rather than scraping HTML, so it is cheap, deterministic and stable across
// site redesigns (see staj-takip-spec-v2.md §16, Faz 10).
type GreenhouseSource struct {
	name     string
	company  string
	endpoint *url.URL
	client   *http.Client
	now      func() time.Time
}

func NewGreenhouseSource(name string, company string, boardURL string, client *http.Client) (*GreenhouseSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	parsedURL, err := url.Parse(boardURL)
	if err != nil || parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Hostname(), "boards-api.greenhouse.io") {
		return nil, errors.New("board URL must be an absolute boards-api.greenhouse.io HTTPS URL")
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 4 || pathParts[0] != "v1" || pathParts[1] != "boards" || pathParts[2] == "" || pathParts[3] != "jobs" {
		return nil, errors.New("board URL must be /v1/boards/{token}/jobs")
	}
	// Always request the full job content so descriptions reach the analyzer.
	query := parsedURL.Query()
	query.Set("content", "true")
	parsedURL.RawQuery = query.Encode()
	parsedURL.Fragment = ""
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &GreenhouseSource{name: name, company: company, endpoint: parsedURL, client: client, now: time.Now}, nil
}

func (s *GreenhouseSource) Name() string { return s.name }

func (s *GreenhouseSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope:           "boards-api.greenhouse.io",
		MinimumInterval: time.Second,
		BaseCooldown:    time.Minute,
		MaximumCooldown: time.Hour,
	}
}

type greenhouseResponse struct {
	Jobs *[]greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	ID          json.Number `json:"id"`
	Title       string      `json:"title"`
	AbsoluteURL string      `json:"absolute_url"`
	UpdatedAt   string      `json:"updated_at"`
	Content     string      `json:"content"`
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
}

func (s *GreenhouseSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Greenhouse request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Greenhouse board: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch Greenhouse board: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGreenhouseBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Greenhouse board: %w", err)
	}
	if len(body) > maxGreenhouseBodyBytes {
		return nil, fmt.Errorf("read Greenhouse board: response exceeds %d bytes", maxGreenhouseBodyBytes)
	}

	var payload greenhouseResponse
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("%w: malformed Greenhouse payload: %v", ErrUnexpectedPage, err)
	}
	if payload.Jobs == nil {
		return nil, fmt.Errorf("%w: Greenhouse payload has no jobs array", ErrUnexpectedPage)
	}

	fetchedAt := s.now().UTC()
	listings := make([]domain.RawListing, 0, len(*payload.Jobs))
	seen := make(map[string]struct{})
	for _, job := range *payload.Jobs {
		title := strings.TrimSpace(job.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: Greenhouse job has no title", ErrUnexpectedPage)
		}
		listingURL := strings.TrimSpace(job.AbsoluteURL)
		parsed, err := url.Parse(listingURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("%w: Greenhouse job %q has no valid absolute_url", ErrUnexpectedPage, title)
		}
		if _, exists := seen[listingURL]; exists {
			continue
		}
		seen[listingURL] = struct{}{}

		parts := []string{title}
		if location := strings.TrimSpace(job.Location.Name); location != "" {
			parts = append(parts, location)
		}
		if job.UpdatedAt != "" {
			parts = append(parts, "Updated: "+strings.TrimSpace(job.UpdatedAt))
		}
		// Greenhouse returns the description as HTML-escaped HTML inside JSON,
		// so unescape once before stripping tags to plain text.
		if content := htmlToText(stdhtml.UnescapeString(job.Content)); content != "" {
			parts = append(parts, content)
		}
		listings = append(listings, domain.RawListing{
			Company:   s.company,
			SourceID:  s.name,
			Title:     title,
			URL:       listingURL,
			RawText:   strings.Join(parts, "\n"),
			FetchedAt: fetchedAt,
		})
	}
	return listings, nil
}
