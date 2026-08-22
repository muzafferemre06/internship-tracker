package scraper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

var htmlTagRegex = regexp.MustCompile(`(?s)<[^>]*>`)

// RSSFeedSource implements Source and ProtectedSource for RSS 2.0 and Atom feeds.
type RSSFeedSource struct {
	name        string
	company     string
	feedURL     *url.URL
	checkpoints FeedCheckpointStore
	client      *http.Client
}

// NewRSSFeedSource creates and validates a new RSS/Atom feed source.
func NewRSSFeedSource(
	name string,
	company string,
	feedURL string,
	checkpoints FeedCheckpointStore,
	client *http.Client,
) (*RSSFeedSource, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, errors.New("name cannot be empty")
	}

	trimmedCompany := strings.TrimSpace(company)
	if trimmedCompany == "" {
		return nil, errors.New("company cannot be empty")
	}

	if checkpoints == nil {
		return nil, errors.New("checkpoints store cannot be nil")
	}

	parsedURL, err := url.Parse(strings.TrimSpace(feedURL))
	if err != nil || !parsedURL.IsAbs() || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid feed URL %q: must be an absolute http/https URL", feedURL)
	}

	if client == nil {
		client = &http.Client{
			Timeout: 20 * time.Second,
		}
	}

	return &RSSFeedSource{
		name:        trimmedName,
		company:     trimmedCompany,
		feedURL:     parsedURL,
		checkpoints: checkpoints,
		client:      client,
	}, nil
}

// Name returns the identifier of this feed source.
func (s *RSSFeedSource) Name() string {
	return s.name
}

// AccessPolicy returns rate limiting and cooldown policy for this source.
func (s *RSSFeedSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope:           s.feedURL.Hostname(),
		MinimumInterval: time.Second,
		BaseCooldown:    time.Minute,
		MaximumCooldown: time.Hour,
	}
}

// FetchListings polls the feed, handles conditional GET caching, parses items,
// and yields new or modified job listings.
func (s *RSSFeedSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	checkpoint, found, err := s.checkpoints.LoadFeedCheckpoint(ctx, s.name)
	if err != nil {
		return nil, fmt.Errorf("failed to load feed checkpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	if found {
		if checkpoint.ETag != "" {
			req.Header.Set("If-None-Match", checkpoint.ETag)
		}
		if checkpoint.LastModified != "" {
			req.Header.Set("If-Modified-Since", checkpoint.LastModified)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from feed %s", resp.StatusCode, s.feedURL.String())
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read feed body: %w", err)
	}

	items, err := parseFeed(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed from %s: %w", s.feedURL.String(), err)
	}

	listings := make([]domain.RawListing, 0)
	for _, item := range items {
		h := sha256.Sum256([]byte(item.title + "\n" + item.rawText + "\n" + item.link))
		contentHash := hex.EncodeToString(h[:])

		priorHash, seen, err := s.checkpoints.LoadSeenFeedItem(ctx, s.name, item.itemKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check seen feed item %q: %w", item.itemKey, err)
		}

		if !seen || priorHash != contentHash {
			now := time.Now().UTC()
			if err := s.checkpoints.MarkSeenFeedItem(ctx, s.name, item.itemKey, contentHash, now); err != nil {
				return nil, fmt.Errorf("failed to mark seen feed item %q: %w", item.itemKey, err)
			}

			listings = append(listings, domain.RawListing{
				Company:   s.company,
				SourceID:  s.name,
				Title:     item.title,
				URL:       item.link,
				RawText:   item.rawText,
				FetchedAt: now,
			})
		}
	}

	newCheckpoint := domain.FeedCheckpoint{
		SourceKey:    s.name,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	if err := s.checkpoints.SaveFeedCheckpoint(ctx, newCheckpoint); err != nil {
		return nil, fmt.Errorf("failed to save feed checkpoint: %w", err)
	}

	return listings, nil
}

type parsedFeedItem struct {
	title   string
	link    string
	rawText string
	itemKey string
}

func parseFeed(data []byte) ([]parsedFeedItem, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return nil, errors.New("empty XML document")
			}
			return nil, fmt.Errorf("failed to read XML token: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		switch strings.ToLower(start.Name.Local) {
		case "rss":
			var rss rssRoot
			if err := decoder.DecodeElement(&rss, &start); err != nil {
				return nil, fmt.Errorf("failed to decode RSS 2.0 feed: %w", err)
			}
			return parseRSSItems(rss), nil
		case "feed":
			var atom atomFeed
			if err := decoder.DecodeElement(&atom, &start); err != nil {
				return nil, fmt.Errorf("failed to decode Atom feed: %w", err)
			}
			return parseAtomItems(atom), nil
		default:
			return nil, fmt.Errorf("unsupported feed root element <%s>", start.Name.Local)
		}
	}
}

type rssRoot struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title          string `xml:"title"`
	Link           string `xml:"link"`
	Description    string `xml:"description"`
	ContentEncoded string `xml:"encoded"`
	Content        string `xml:"content"`
	GUID           string `xml:"guid"`
}

func parseRSSItems(rss rssRoot) []parsedFeedItem {
	items := make([]parsedFeedItem, 0, len(rss.Channel.Items))
	for _, item := range rss.Channel.Items {
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)

		content := item.ContentEncoded
		if strings.TrimSpace(content) == "" {
			content = item.Content
		}
		if strings.TrimSpace(content) == "" {
			content = item.Description
		}
		rawText := stripHTML(content)

		guid := strings.TrimSpace(item.GUID)
		itemKey := guid
		if itemKey == "" {
			itemKey = link
		}

		if link == "" || itemKey == "" {
			continue
		}

		items = append(items, parsedFeedItem{
			title:   title,
			link:    link,
			rawText: rawText,
			itemKey: itemKey,
		})
	}
	return items
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	ID      string     `xml:"id"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

func parseAtomItems(atom atomFeed) []parsedFeedItem {
	items := make([]parsedFeedItem, 0, len(atom.Entries))
	for _, entry := range atom.Entries {
		title := strings.TrimSpace(entry.Title)
		link := selectAtomLink(entry.Links)

		content := entry.Content
		if strings.TrimSpace(content) == "" {
			content = entry.Summary
		}
		rawText := stripHTML(content)

		id := strings.TrimSpace(entry.ID)
		itemKey := id
		if itemKey == "" {
			itemKey = link
		}

		if link == "" || itemKey == "" {
			continue
		}

		items = append(items, parsedFeedItem{
			title:   title,
			link:    link,
			rawText: rawText,
			itemKey: itemKey,
		})
	}
	return items
}

func selectAtomLink(links []atomLink) string {
	var fallbackEmptyRel string
	var firstAny string

	for _, l := range links {
		href := strings.TrimSpace(l.Href)
		if href == "" {
			continue
		}
		if firstAny == "" {
			firstAny = href
		}

		rel := strings.ToLower(strings.TrimSpace(l.Rel))
		if rel == "alternate" {
			return href
		}
		if rel == "" && fallbackEmptyRel == "" {
			fallbackEmptyRel = href
		}
	}

	if fallbackEmptyRel != "" {
		return fallbackEmptyRel
	}
	return firstAny
}

func stripHTML(input string) string {
	cleaned := htmlTagRegex.ReplaceAllString(input, "")
	unescaped := html.UnescapeString(cleaned)
	return strings.TrimSpace(unescaped)
}