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
)

const maxAshbyBodyBytes = 8 << 20

// AshbyBoardSource reads one company's public Ashby job-board JSON endpoint.
// It keeps only listed postings belonging to the configured board and never
// visits application URLs.
type AshbyBoardSource struct {
	name     string
	company  string
	endpoint *url.URL
	slug     string
	client   *http.Client
	now      func() time.Time
}

func NewAshbyBoardSource(name, company, boardURL string, client *http.Client) (*AshbyBoardSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	parsedURL, err := url.Parse(boardURL)
	if err != nil || parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Hostname(), "api.ashbyhq.com") {
		return nil, errors.New("board URL must be an absolute api.ashbyhq.com HTTPS URL")
	}
	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "posting-api" || parts[1] != "job-board" || parts[2] == "" {
		return nil, errors.New("board URL must be /posting-api/job-board/{board}")
	}
	parsedURL.Path = "/posting-api/job-board/" + parts[2]
	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &AshbyBoardSource{
		name: name, company: company, endpoint: parsedURL, slug: parts[2], client: client, now: time.Now,
	}, nil
}

func (s *AshbyBoardSource) Name() string { return s.name }

func (s *AshbyBoardSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope:           "api.ashbyhq.com",
		MinimumInterval: time.Second,
		BaseCooldown:    time.Minute,
		MaximumCooldown: time.Hour,
	}
}

type ashbyBoardResponse struct {
	Jobs *[]ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	Title              string `json:"title"`
	Department         string `json:"department"`
	Team               string `json:"team"`
	EmploymentType     string `json:"employmentType"`
	Location           string `json:"location"`
	PublishedAt        string `json:"publishedAt"`
	IsListed           bool   `json:"isListed"`
	IsRemote           bool   `json:"isRemote"`
	WorkplaceType      string `json:"workplaceType"`
	JobURL             string `json:"jobUrl"`
	DescriptionPlain   string `json:"descriptionPlain"`
	SecondaryLocations []struct {
		Location string `json:"location"`
	} `json:"secondaryLocations"`
}

func (s *AshbyBoardSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Ashby board request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Ashby board: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch Ashby board: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAshbyBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Ashby board: %w", err)
	}
	if len(body) > maxAshbyBodyBytes {
		return nil, fmt.Errorf("read Ashby board: response exceeds %d bytes", maxAshbyBodyBytes)
	}

	var payload ashbyBoardResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: malformed Ashby payload: %v", ErrUnexpectedPage, err)
	}
	if payload.Jobs == nil {
		return nil, fmt.Errorf("%w: Ashby payload has no jobs array", ErrUnexpectedPage)
	}

	fetchedAt := s.now().UTC()
	seen := make(map[string]struct{})
	listings := make([]domain.RawListing, 0, len(*payload.Jobs))
	for _, job := range *payload.Jobs {
		if !job.IsListed {
			continue
		}
		title := strings.TrimSpace(job.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: Ashby job has no title", ErrUnexpectedPage)
		}
		postingURL, err := url.Parse(strings.TrimSpace(job.JobURL))
		if err != nil || postingURL.Scheme != "https" || !strings.EqualFold(postingURL.Hostname(), "jobs.ashbyhq.com") {
			return nil, fmt.Errorf("%w: Ashby job %q has no valid jobUrl", ErrUnexpectedPage, title)
		}
		parts := strings.Split(strings.Trim(postingURL.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != s.slug || parts[1] == "" {
			return nil, fmt.Errorf("%w: Ashby job %q does not belong to board %q", ErrUnexpectedPage, title, s.slug)
		}
		postingURL.RawQuery = ""
		postingURL.Fragment = ""
		canonicalURL := postingURL.String()
		if _, duplicate := seen[canonicalURL]; duplicate {
			continue
		}
		seen[canonicalURL] = struct{}{}

		rawParts := []string{title}
		for _, value := range []string{job.Department, job.Team, job.EmploymentType, job.Location} {
			if value = strings.TrimSpace(value); value != "" {
				rawParts = append(rawParts, value)
			}
		}
		for _, location := range job.SecondaryLocations {
			if value := strings.TrimSpace(location.Location); value != "" {
				rawParts = append(rawParts, value)
			}
		}
		if value := strings.TrimSpace(job.WorkplaceType); value != "" {
			rawParts = append(rawParts, value)
		} else if job.IsRemote {
			rawParts = append(rawParts, "Remote")
		}
		if value := strings.TrimSpace(job.PublishedAt); value != "" {
			rawParts = append(rawParts, "Published: "+value)
		}
		if value := strings.TrimSpace(job.DescriptionPlain); value != "" {
			rawParts = append(rawParts, value)
		}
		listings = append(listings, domain.RawListing{
			Company: s.company, SourceID: s.name, Title: title, URL: canonicalURL,
			RawText: strings.Join(rawParts, "\n"), FetchedAt: fetchedAt,
		})
	}
	return listings, nil
}
