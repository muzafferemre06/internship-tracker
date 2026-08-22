package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	maxDiscoveryBodySize = 2 << 20 // 2MB limit
	defaultDiscoveryUA   = "internship-tracker/0.1 (+personal career monitoring)"
)

// DiscoverFeedLinks fetches pageURL and parses HTML for RSS/Atom <link> tags.
// Only returns deduplicated absolute URLs belonging to the exact same domain (hostname) as pageURL.
func DiscoverFeedLinks(ctx context.Context, pageURL string, client *http.Client) ([]string, error) {
	parsedPageURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid page URL %q: %w", pageURL, err)
	}
	if !parsedPageURL.IsAbs() || (parsedPageURL.Scheme != "http" && parsedPageURL.Scheme != "https") || parsedPageURL.Hostname() == "" {
		return nil, fmt.Errorf("page URL must be an absolute http or https URL, got %q", pageURL)
	}

	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %q: %w", pageURL, err)
	}
	req.Header.Set("User-Agent", defaultDiscoveryUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %q: %w", pageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d (%s) for %q", resp.StatusCode, resp.Status, pageURL)
	}

	limitReader := io.LimitReader(resp.Body, maxDiscoveryBodySize)
	doc, err := html.Parse(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML from %q: %w", pageURL, err)
	}

	var typedHrefs []string
	var untypedHrefs []string

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "link") {
			var rel, typeAttr, href string
			for _, attr := range n.Attr {
				switch strings.ToLower(attr.Key) {
				case "rel":
					rel = attr.Val
				case "type":
					typeAttr = attr.Val
				case "href":
					href = attr.Val
				}
			}

			if hasAlternateRel(rel) && strings.TrimSpace(href) != "" {
				trimmedType := strings.ToLower(strings.TrimSpace(typeAttr))
				trimmedHref := strings.TrimSpace(href)

				if isFeedType(trimmedType) {
					typedHrefs = append(typedHrefs, trimmedHref)
				} else if trimmedType == "" && isFeedHrefFallback(trimmedHref) {
					untypedHrefs = append(untypedHrefs, trimmedHref)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	resolveAndFilter := func(hrefs []string) []string {
		var valid []string
		seen := make(map[string]bool)
		for _, h := range hrefs {
			ref, err := url.Parse(h)
			if err != nil {
				continue
			}
			resolved := parsedPageURL.ResolveReference(ref)
			if resolved.Scheme != "http" && resolved.Scheme != "https" {
				continue
			}
			if resolved.Hostname() != parsedPageURL.Hostname() {
				continue
			}
			resolvedStr := resolved.String()
			if !seen[resolvedStr] {
				seen[resolvedStr] = true
				valid = append(valid, resolvedStr)
			}
		}
		return valid
	}

	typedValid := resolveAndFilter(typedHrefs)
	if len(typedValid) > 0 {
		return typedValid, nil
	}

	untypedValid := resolveAndFilter(untypedHrefs)
	if len(untypedValid) > 0 {
		return untypedValid, nil
	}

	return []string{}, nil
}

func hasAlternateRel(rel string) bool {
	fields := strings.Fields(strings.ToLower(rel))
	for _, f := range fields {
		if f == "alternate" {
			return true
		}
	}
	return false
}

func isFeedType(t string) bool {
	switch t {
	case "application/rss+xml", "application/atom+xml", "application/x.atom+xml", "application/x-atom+xml":
		return true
	default:
		return false
	}
}

func isFeedHrefFallback(href string) bool {
	u, err := url.Parse(href)
	var p string
	if err == nil && u.Path != "" {
		p = strings.ToLower(u.Path)
	} else {
		p = strings.ToLower(href)
	}
	return strings.HasSuffix(p, ".rss") || strings.HasSuffix(p, ".atom") || strings.HasSuffix(p, ".xml")
}

// NewRSSFeedSourceFromPage discovers RSS/Atom feed links on pageURL and constructs an RSSFeedSource from the first found link.
func NewRSSFeedSourceFromPage(ctx context.Context, name string, company string, pageURL string, checkpoints FeedCheckpointStore, client *http.Client) (*RSSFeedSource, error) {
	links, err := DiscoverFeedLinks(ctx, pageURL, client)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("no RSS/Atom feed link discovered on page %q", pageURL)
	}
	return NewRSSFeedSource(name, company, links[0], checkpoints, client)
}
