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

const maxCareerPageBytes = 5 << 20
const maxAccessErrorBodyBytes = 64 << 10

var ErrUnexpectedPage = errors.New("career page structure is not recognized")

type KariyerNetSource struct {
	name       string
	company    string
	pageName   string
	profileURL *url.URL
	client     *http.Client
	now        func() time.Time
}

func NewKariyerNetSource(
	name string,
	company string,
	pageName string,
	profileURL string,
	client *http.Client,
) (*KariyerNetSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	pageName = strings.TrimSpace(pageName)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	if pageName == "" {
		return nil, errors.New("page name is required")
	}

	parsedURL, err := url.Parse(profileURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("profile URL must be an absolute HTTP(S) URL")
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	return &KariyerNetSource{
		name:       name,
		company:    company,
		pageName:   pageName,
		profileURL: parsedURL,
		client:     client,
		now:        time.Now,
	}, nil
}

func (s *KariyerNetSource) Name() string { return s.name }

func (s *KariyerNetSource) AccessPolicy() AccessPolicy {
	scope := strings.ToLower(s.profileURL.Hostname())
	if scope == "kariyer.net" || strings.HasSuffix(scope, ".kariyer.net") {
		scope = "kariyer.net"
	}
	return AccessPolicy{
		Scope:           scope,
		MinimumInterval: 24 * time.Hour,
		BaseCooldown:    6 * time.Hour,
		MaximumCooldown: 24 * time.Hour,
	}
}

func (s *KariyerNetSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.profileURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create kariyer.net request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("Accept-Language", "tr-TR,tr;q=0.9")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch kariyer.net profile: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch kariyer.net profile: %w", accessError(response, body, s.now().UTC()))
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxCareerPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read kariyer.net profile: %w", err)
	}
	if len(body) > maxCareerPageBytes {
		return nil, fmt.Errorf("read kariyer.net profile: response exceeds %d bytes", maxCareerPageBytes)
	}
	if isAccessChallenge(response, body) {
		return nil, fmt.Errorf("fetch kariyer.net profile: %w", accessError(response, body, s.now().UTC()))
	}

	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse kariyer.net profile: %w", err)
	}
	return s.parseListings(root)
}

func accessError(response *http.Response, body []byte, now time.Time) *AccessError {
	return &AccessError{
		StatusCode: response.StatusCode,
		RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"), now),
		Server:     strings.TrimSpace(response.Header.Get("Server")),
		CFRay:      strings.TrimSpace(response.Header.Get("CF-Ray")),
		Challenge:  isAccessChallenge(response, body),
	}
}

func isAccessChallenge(response *http.Response, body []byte) bool {
	if strings.EqualFold(strings.TrimSpace(response.Header.Get("cf-mitigated")), "challenge") {
		return true
	}
	text := strings.ToLower(string(body))
	return strings.Contains(text, "cf-chl-") ||
		strings.Contains(text, "challenge-platform") ||
		strings.Contains(text, "just a moment...")
}

func parseRetryAfter(value string, now time.Time) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if seconds, err := time.ParseDuration(value + "s"); err == nil && seconds >= 0 {
		retryAt := now.Add(seconds).UTC()
		return &retryAt
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		retryAt = retryAt.UTC()
		return &retryAt
	}
	return nil
}

func (s *KariyerNetSource) parseListings(root *html.Node) ([]domain.RawListing, error) {
	if !containsCompanyHeading(root, s.pageName) {
		return nil, ErrUnexpectedPage
	}

	fetchedAt := s.now().UTC()
	listings := make([]domain.RawListing, 0)
	seenURLs := make(map[string]struct{})
	var parseErr error

	walkElements(root, func(node *html.Node) {
		if parseErr != nil || node.Data != "a" {
			return
		}
		href, exists := attribute(node, "href")
		if !exists || !strings.Contains(href, "/is-ilani/") {
			return
		}

		jobURL, err := s.profileURL.Parse(href)
		if err != nil || (jobURL.Scheme != "http" && jobURL.Scheme != "https") || jobURL.Host == "" {
			parseErr = fmt.Errorf("%w: invalid job URL", ErrUnexpectedPage)
			return
		}
		jobURL.Fragment = ""
		if _, exists := seenURLs[jobURL.String()]; exists {
			return
		}

		title := jobTitle(node)
		if title == "" {
			parseErr = fmt.Errorf("%w: job link has no title", ErrUnexpectedPage)
			return
		}

		seenURLs[jobURL.String()] = struct{}{}
		listings = append(listings, domain.RawListing{
			Company:   s.company,
			SourceID:  s.name,
			Title:     title,
			URL:       jobURL.String(),
			RawText:   normalizedText(node),
			FetchedAt: fetchedAt,
		})
	})
	if parseErr != nil {
		return nil, parseErr
	}
	return listings, nil
}

func containsCompanyHeading(root *html.Node, company string) bool {
	found := false
	walkElements(root, func(node *html.Node) {
		if !found && node.Data == "h1" && strings.Contains(
			strings.ToLower(normalizedText(node)),
			strings.ToLower(company),
		) {
			found = true
		}
	})
	return found
}

func jobTitle(anchor *html.Node) string {
	if title, ok := attribute(anchor, "data-job-title"); ok {
		return strings.TrimSpace(title)
	}
	if title := descendantTextByAttribute(anchor, "data-testid", "job-title"); title != "" {
		return title
	}
	if title, ok := attribute(anchor, "aria-label"); ok {
		return strings.TrimSpace(title)
	}
	return normalizedText(anchor)
}

func descendantTextByAttribute(root *html.Node, key string, value string) string {
	result := ""
	walkElements(root, func(node *html.Node) {
		if result == "" {
			if attributeValue, ok := attribute(node, key); ok && attributeValue == value {
				result = normalizedText(node)
			}
		}
	})
	return result
}

func attribute(node *html.Node, key string) (string, bool) {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val, true
		}
	}
	return "", false
}

func walkElements(root *html.Node, visit func(*html.Node)) {
	if root.Type == html.ElementNode {
		visit(root)
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walkElements(child, visit)
	}
}

func normalizedText(root *html.Node) string {
	parts := make([]string, 0)
	var walkText func(*html.Node)
	walkText = func(node *html.Node) {
		if node.Type == html.TextNode {
			if text := strings.TrimSpace(node.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walkText(child)
		}
	}
	walkText(root)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
