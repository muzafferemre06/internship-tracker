package scraper

import (
	"context"
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

// LeverBoardSource reads the public, server-rendered job board for one Lever
// company. It discovers posting URLs but does not fetch every posting page.
type LeverBoardSource struct {
	name     string
	company  string
	boardURL *url.URL
	slug     string
	client   *http.Client
	now      func() time.Time
}

func NewLeverBoardSource(name, company, boardURL string, client *http.Client) (*LeverBoardSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	parsedURL, err := url.Parse(boardURL)
	if err != nil || parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Hostname(), "jobs.lever.co") {
		return nil, errors.New("board URL must be an absolute jobs.lever.co HTTPS URL")
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 1 || pathParts[0] == "" {
		return nil, errors.New("board URL must identify one Lever company")
	}
	parsedURL.Path = "/" + pathParts[0]
	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &LeverBoardSource{
		name: name, company: company, boardURL: parsedURL, slug: pathParts[0], client: client, now: time.Now,
	}, nil
}

func (s *LeverBoardSource) Name() string { return s.name }

func (s *LeverBoardSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope:           "jobs.lever.co",
		MinimumInterval: time.Second,
		BaseCooldown:    time.Minute,
		MaximumCooldown: time.Hour,
	}
}

func (s *LeverBoardSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.boardURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Lever board request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Lever board: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch Lever board: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLeverPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Lever board: %w", err)
	}
	if len(body) > maxLeverPageBytes {
		return nil, fmt.Errorf("read Lever board: response exceeds %d bytes", maxLeverPageBytes)
	}
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse Lever board: %w", err)
	}
	return s.parseListings(root)
}

func (s *LeverBoardSource) parseListings(root *html.Node) ([]domain.RawListing, error) {
	container := firstElement(root, func(node *html.Node) bool { return hasClass(node, "postings-wrapper") })
	if container == nil {
		return nil, ErrUnexpectedPage
	}
	fetchedAt := s.now().UTC()
	seen := make(map[string]struct{})
	listings := make([]domain.RawListing, 0)
	for _, anchor := range elements(container, func(node *html.Node) bool {
		return node.Data == "a" && hasClass(node, "posting-title")
	}) {
		href, ok := attribute(anchor, "href")
		if !ok {
			continue
		}
		postingURL, err := s.boardURL.Parse(strings.TrimSpace(href))
		if err != nil || postingURL.Scheme != "https" || !strings.EqualFold(postingURL.Hostname(), "jobs.lever.co") {
			continue
		}
		parts := strings.Split(strings.Trim(postingURL.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != s.slug || parts[1] == "" {
			continue
		}
		postingURL.RawQuery = ""
		postingURL.Fragment = ""
		canonicalURL := postingURL.String()
		if _, duplicate := seen[canonicalURL]; duplicate {
			continue
		}
		titleNode := firstElement(anchor, func(node *html.Node) bool { return node.Data == "h5" })
		title := normalizedText(titleNode)
		if title == "" {
			return nil, fmt.Errorf("%w: Lever board posting has no title", ErrUnexpectedPage)
		}
		rawText := normalizedText(anchor)
		if rawText == "" {
			rawText = title
		}
		seen[canonicalURL] = struct{}{}
		listings = append(listings, domain.RawListing{
			Company: s.company, SourceID: s.name, Title: title, URL: canonicalURL,
			RawText: rawText, FetchedAt: fetchedAt,
		})
	}
	return listings, nil
}
