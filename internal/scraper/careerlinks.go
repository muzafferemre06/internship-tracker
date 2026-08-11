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

const maxCareerLinksPageBytes = 4 << 20

// CareerLinksSource extracts same-origin listing cards from an open career
// index. Configuration narrows discovery to a listing URL path and, when
// necessary, a stable container id so navigation links are never candidates.
type CareerLinksSource struct {
	name               string
	company            string
	pageURL            *url.URL
	listingContainerID string
	listingPathPrefix  string
	client             *http.Client
	now                func() time.Time
}

func NewCareerLinksSource(name, company, pageURL, listingContainerID, listingPathPrefix string, client *http.Client) (*CareerLinksSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	listingContainerID = strings.TrimSpace(listingContainerID)
	listingPathPrefix = strings.TrimSpace(listingPathPrefix)
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
	if !strings.HasPrefix(listingPathPrefix, "/") {
		return nil, errors.New("listing path prefix must start with /")
	}
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &CareerLinksSource{
		name: name, company: company, pageURL: parsedURL,
		listingContainerID: listingContainerID, listingPathPrefix: listingPathPrefix,
		client: client, now: time.Now,
	}, nil
}

func (s *CareerLinksSource) Name() string { return s.name }

func (s *CareerLinksSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope: s.pageURL.Hostname(), MinimumInterval: 24 * time.Hour,
		BaseCooldown: time.Hour, MaximumCooldown: 24 * time.Hour,
	}
}

func (s *CareerLinksSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.pageURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create career links request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch career links page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch career links page: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxCareerLinksPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read career links page: %w", err)
	}
	if len(body) > maxCareerLinksPageBytes {
		return nil, fmt.Errorf("read career links page: response exceeds %d bytes", maxCareerLinksPageBytes)
	}
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse career links page: %w", err)
	}
	return s.parseListings(root)
}

func (s *CareerLinksSource) parseListings(root *html.Node) ([]domain.RawListing, error) {
	scope := root
	if s.listingContainerID != "" {
		scope = firstElement(root, func(node *html.Node) bool {
			value, ok := attribute(node, "id")
			return ok && value == s.listingContainerID
		})
		if scope == nil {
			return nil, fmt.Errorf("%w: listing container %q not found", ErrUnexpectedPage, s.listingContainerID)
		}
	}
	fetchedAt := s.now().UTC()
	seen := make(map[string]struct{})
	listings := make([]domain.RawListing, 0)
	for _, anchor := range elements(scope, func(node *html.Node) bool { return node.Data == "a" }) {
		href, ok := attribute(anchor, "href")
		if !ok {
			continue
		}
		listingURL, err := s.pageURL.Parse(strings.TrimSpace(href))
		if err != nil || (listingURL.Scheme != "http" && listingURL.Scheme != "https") ||
			!strings.EqualFold(listingURL.Hostname(), s.pageURL.Hostname()) ||
			!strings.HasPrefix(listingURL.Path, s.listingPathPrefix) || listingURL.Path == s.pageURL.Path {
			continue
		}
		listingURL.RawQuery = ""
		listingURL.Fragment = ""
		canonicalURL := listingURL.String()
		if _, duplicate := seen[canonicalURL]; duplicate {
			continue
		}
		title, card := careerLinkTitleAndCard(anchor, scope)
		if title == "" {
			continue
		}
		seen[canonicalURL] = struct{}{}
		listings = append(listings, domain.RawListing{
			Company: s.company, SourceID: s.name, Title: title, URL: canonicalURL,
			RawText: normalizedText(card), FetchedAt: fetchedAt,
		})
	}
	if len(listings) == 0 {
		return nil, fmt.Errorf("%w: no listing links matched %q", ErrUnexpectedPage, s.listingPathPrefix)
	}
	return listings, nil
}

func careerLinkTitleAndCard(anchor, scope *html.Node) (string, *html.Node) {
	for candidate := anchor; candidate != nil && candidate != scope; candidate = candidate.Parent {
		heading := firstElement(candidate, func(node *html.Node) bool {
			return len(node.Data) == 2 && node.Data[0] == 'h' && node.Data[1] >= '1' && node.Data[1] <= '6'
		})
		if heading != nil {
			if title := normalizedText(heading); title != "" {
				return title, candidate
			}
		}
	}
	text := firstCareerLinkLabel(anchor)
	switch strings.ToLower(text) {
	case "", "detay", "başvur", "başvurun", "şimdi başvur", "read more", "apply", "apply now":
		return "", anchor
	default:
		return text, anchor
	}
}

func firstCareerLinkLabel(anchor *html.Node) string {
	for child := anchor.FirstChild; child != nil; child = child.NextSibling {
		if text := normalizedText(child); text != "" {
			return text
		}
	}
	return ""
}
