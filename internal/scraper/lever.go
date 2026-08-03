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

const maxLeverPageBytes = 2 << 20

type LeverSource struct {
	name       string
	company    string
	postingURL *url.URL
	client     *http.Client
	now        func() time.Time
}

func NewLeverSource(name string, company string, postingURL string, client *http.Client) (*LeverSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	parsedURL, err := url.Parse(postingURL)
	if err != nil || parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Hostname(), "jobs.lever.co") {
		return nil, errors.New("posting URL must be an absolute jobs.lever.co HTTPS URL")
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] == "" {
		return nil, errors.New("posting URL must identify one Lever company and posting")
	}
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &LeverSource{name: name, company: company, postingURL: parsedURL, client: client, now: time.Now}, nil
}

func (s *LeverSource) Name() string { return s.name }

func (s *LeverSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.postingURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Lever request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Lever posting: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch Lever posting: unexpected HTTP status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLeverPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Lever posting: %w", err)
	}
	if len(body) > maxLeverPageBytes {
		return nil, fmt.Errorf("read Lever posting: response exceeds %d bytes", maxLeverPageBytes)
	}
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse Lever posting: %w", err)
	}
	listing, err := s.parseListing(root)
	if err != nil {
		return nil, err
	}
	return []domain.RawListing{listing}, nil
}

func (s *LeverSource) parseListing(root *html.Node) (domain.RawListing, error) {
	postingPage := firstElement(root, func(node *html.Node) bool {
		return hasClass(node, "posting-page")
	})
	if postingPage == nil {
		return domain.RawListing{}, ErrUnexpectedPage
	}
	headline := firstElement(postingPage, func(node *html.Node) bool { return hasClass(node, "posting-headline") })
	titleNode := firstElement(headline, func(node *html.Node) bool { return node.Data == "h2" })
	title := normalizedText(titleNode)
	if title == "" {
		return domain.RawListing{}, fmt.Errorf("%w: Lever posting has no title", ErrUnexpectedPage)
	}

	applyPath := strings.TrimRight(s.postingURL.Path, "/") + "/apply"
	applyFound := firstElement(postingPage, func(node *html.Node) bool {
		if node.Data != "a" {
			return false
		}
		href, ok := attribute(node, "href")
		if !ok {
			return false
		}
		applyURL, err := s.postingURL.Parse(href)
		return err == nil && strings.EqualFold(applyURL.Hostname(), s.postingURL.Hostname()) && applyURL.Path == applyPath
	})
	if applyFound == nil {
		return domain.RawListing{}, fmt.Errorf("%w: Lever posting has no active application link", ErrUnexpectedPage)
	}

	contentParts := []string{title}
	for _, node := range elements(postingPage, func(node *html.Node) bool {
		if hasClass(node, "posting-categories") || hasAttribute(node, "data-qa", "job-description") ||
			hasAttribute(node, "data-qa", "closing-description") {
			return true
		}
		return hasClass(node, "posting-requirements")
	}) {
		if text := normalizedText(node); text != "" {
			contentParts = append(contentParts, text)
		}
	}
	return domain.RawListing{
		Company: s.company, SourceID: s.name, Title: title, URL: s.postingURL.String(),
		RawText: strings.Join(contentParts, "\n"), FetchedAt: s.now().UTC(),
	}, nil
}

func firstElement(root *html.Node, matches func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	var result *html.Node
	walkElements(root, func(node *html.Node) {
		if result == nil && matches(node) {
			result = node
		}
	})
	return result
}

func elements(root *html.Node, matches func(*html.Node) bool) []*html.Node {
	result := make([]*html.Node, 0)
	walkElements(root, func(node *html.Node) {
		if matches(node) {
			result = append(result, node)
		}
	})
	return result
}

func hasClass(node *html.Node, expected string) bool {
	classes, ok := attribute(node, "class")
	if !ok {
		return false
	}
	for _, class := range strings.Fields(classes) {
		if class == expected {
			return true
		}
	}
	return false
}

func hasAttribute(node *html.Node, key string, expected string) bool {
	value, ok := attribute(node, key)
	return ok && value == expected
}
