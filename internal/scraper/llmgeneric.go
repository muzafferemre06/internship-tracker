package scraper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

const (
	maxLLMGenericPageBytes = 4 << 20
	minCandidateBlockRunes = 12
	maxCandidateBlockRunes = 2000
)

// candidateKeywords are the low-cost signals the reduce stage uses to window a
// chaotic page down to blocks that plausibly describe an opening, before any
// model is consulted (see staj-takip-spec-v2.md §16, Faz 11).
var candidateKeywords = []string{
	"staj", "intern", "başvur", "basvur", "apply", "new grad", "yeni mezun",
	"açık pozisyon", "acik pozisyon", "open position", "pozisyon", "ilan",
}

// nonContentTags are stripped during reduce; their text is never job content.
var nonContentTags = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {}, "template": {}, "svg": {},
	"nav": {}, "header": {}, "footer": {}, "aside": {}, "form": {},
}

// candidateBlockTags are the container elements a listing is usually rendered in.
var candidateBlockTags = map[string]struct{}{
	"li": {}, "article": {}, "tr": {}, "section": {}, "div": {},
}

// ExtractionBlock is one reduced candidate region of a page handed to the model.
type ExtractionBlock struct {
	Index int
	Text  string
}

// ExtractionRequest is the minimized, reduced input sent to the extractor. The
// full page is never sent; only windowed candidate blocks are.
type ExtractionRequest struct {
	Company string
	PageURL string
	Blocks  []ExtractionBlock
}

// ExtractedListing is one opening the model claims to have found in a block.
type ExtractedListing struct {
	SourceBlock int
	Title       string
	URL         string
	Summary     string
	Confidence  float64
}

// ExtractionResult is the model's structured output for one request.
type ExtractionResult struct {
	Listings []ExtractedListing
}

// ListingExtractor is the injected port that turns reduced blocks into
// structured listings. Normal tests use a deterministic fake; production wires
// a Gemini-backed implementation (internal/extractor).
type ListingExtractor interface {
	Name() string
	Extract(ctx context.Context, request ExtractionRequest) (ExtractionResult, error)
}

type ExtractionBlockStore interface {
	LoadExtractionBlocks(context.Context, string, []string) (map[string][]domain.RawListing, error)
	SaveExtractionBlocks(context.Context, string, map[string][]domain.RawListing) error
}

// LLMGenericSource implements the Faz 11 "llm_generic" strategy for chaotic,
// bespoke career pages that offer neither an API nor stable structure. It keeps
// the model off the hot path: a deterministic reduce stage windows candidate
// blocks, a content-hash gate skips the model entirely on unchanged rescans, and
// only new/changed blocks are ever sent for extraction.
type LLMGenericSource struct {
	name       string
	company    string
	pageURL    *url.URL
	client     *http.Client
	extractor  ListingExtractor
	now        func() time.Time
	cache      map[string][]domain.RawListing // block hash -> listings extracted from it
	blockCache ExtractionBlockStore
}

func NewLLMGenericSource(name, company, pageURL string, extractor ListingExtractor, client *http.Client, persistentCache ...ExtractionBlockStore) (*LLMGenericSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" {
		return nil, errors.New("source name is required")
	}
	if company == "" {
		return nil, errors.New("company name is required")
	}
	if extractor == nil {
		return nil, errors.New("listing extractor is required")
	}
	parsedURL, err := url.Parse(pageURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("page URL must be an absolute HTTP(S) URL")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	source := &LLMGenericSource{
		name: name, company: company, pageURL: parsedURL, client: client,
		extractor: extractor, now: time.Now, cache: make(map[string][]domain.RawListing),
	}
	if len(persistentCache) > 0 {
		source.blockCache = persistentCache[0]
	}
	return source, nil
}

func (s *LLMGenericSource) Name() string { return s.name }

func (s *LLMGenericSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{
		Scope:           s.pageURL.Hostname(),
		MinimumInterval: 2 * time.Second,
		BaseCooldown:    time.Minute,
		MaximumCooldown: time.Hour,
	}
}

func (s *LLMGenericSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.pageURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create generic request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch generic page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch generic page: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLLMGenericPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read generic page: %w", err)
	}
	if len(body) > maxLLMGenericPageBytes {
		return nil, fmt.Errorf("read generic page: response exceeds %d bytes", maxLLMGenericPageBytes)
	}
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse generic page: %w", err)
	}

	blocks := s.reduce(root)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%w: reduce found no job-like blocks on the page", ErrUnexpectedPage)
	}
	return s.extract(ctx, blocks)
}

// reduce windows a page down to deterministic candidate blocks: non-content
// subtrees are dropped, and the innermost container elements whose text mentions
// a job keyword (within length bounds) are kept, with any application links
// appended so the model can cite a concrete URL.
func (s *LLMGenericSource) reduce(root *html.Node) []ExtractionBlock {
	matched := make([]*html.Node, 0)
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if _, skip := nonContentTags[node.Data]; skip {
				return
			}
			if _, ok := candidateBlockTags[node.Data]; ok && s.isCandidateBlock(node) {
				matched = append(matched, node)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)

	blocks := make([]ExtractionBlock, 0)
	seen := make(map[string]struct{})
	for _, node := range innermost(matched) {
		text := s.blockText(node)
		if text == "" {
			continue
		}
		if _, dup := seen[text]; dup {
			continue
		}
		seen[text] = struct{}{}
		blocks = append(blocks, ExtractionBlock{Index: len(blocks), Text: text})
	}
	return blocks
}

func (s *LLMGenericSource) isCandidateBlock(node *html.Node) bool {
	text := strings.ToLower(normalizedText(node))
	runes := len([]rune(text))
	if runes < minCandidateBlockRunes || runes > maxCandidateBlockRunes {
		return false
	}
	for _, keyword := range candidateKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// blockText renders a candidate block to line-structured text (block-level
// elements become separate lines, so the heading is distinguishable from the
// body) plus one "LINK:" line per resolved application anchor, giving the model
// concrete URLs to cite.
func (s *LLMGenericSource) blockText(node *html.Node) string {
	parts := []string{structuredText(node)}
	seen := make(map[string]struct{})
	for _, anchor := range elements(node, func(n *html.Node) bool { return n.Data == "a" }) {
		href, ok := attribute(anchor, "href")
		if !ok {
			continue
		}
		resolved, err := s.pageURL.Parse(strings.TrimSpace(href))
		if err != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") {
			continue
		}
		resolved.Fragment = ""
		link := resolved.String()
		if _, dup := seen[link]; dup {
			continue
		}
		seen[link] = struct{}{}
		parts = append(parts, "LINK: "+link)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// extract applies the content-hash gate and per-block diff: unchanged blocks are
// served from cache with no model call; only new/changed blocks are sent to the
// extractor. Results are attributed back to blocks by index and cached by hash.
func (s *LLMGenericSource) extract(ctx context.Context, blocks []ExtractionBlock) ([]domain.RawListing, error) {
	hashes := make([]string, len(blocks))
	for i, block := range blocks {
		hash := blockHash(block.Text)
		hashes[i] = hash
	}
	if s.blockCache != nil {
		persisted, err := s.blockCache.LoadExtractionBlocks(ctx, s.name, hashes)
		if err != nil {
			return nil, fmt.Errorf("load persistent extraction blocks: %w", err)
		}
		for hash, listings := range persisted {
			s.cache[hash] = listings
		}
	}

	changed := make([]ExtractionBlock, 0)
	for i, block := range blocks {
		hash := hashes[i]
		if _, cached := s.cache[hash]; !cached {
			changed = append(changed, ExtractionBlock{Index: i, Text: block.Text})
		}
	}

	fresh := make(map[int][]domain.RawListing)
	if len(changed) > 0 {
		result, err := s.extractor.Extract(ctx, ExtractionRequest{
			Company: s.company, PageURL: s.pageURL.String(), Blocks: changed,
		})
		if err != nil {
			return nil, fmt.Errorf("extract listings: %w", err)
		}
		fetchedAt := s.now().UTC()
		sentIndices := make(map[int]struct{}, len(changed))
		for _, block := range changed {
			sentIndices[block.Index] = struct{}{}
		}
		for _, extracted := range result.Listings {
			if _, ok := sentIndices[extracted.SourceBlock]; !ok {
				continue // model cited a block we did not send; ignore rather than trust
			}
			listing, ok := s.normalizeExtracted(extracted, fetchedAt)
			if !ok {
				continue
			}
			fresh[extracted.SourceBlock] = append(fresh[extracted.SourceBlock], listing)
		}
		// Cache every changed block, including those that yielded nothing, so an
		// unchanged rescan never re-consults the model for them.
		persist := make(map[string][]domain.RawListing, len(changed))
		for _, block := range changed {
			hash := blockHash(block.Text)
			s.cache[hash] = fresh[block.Index]
			persist[hash] = fresh[block.Index]
		}
		if s.blockCache != nil {
			if err := s.blockCache.SaveExtractionBlocks(ctx, s.name, persist); err != nil {
				return nil, fmt.Errorf("save persistent extraction blocks: %w", err)
			}
		}
	}

	listings := make([]domain.RawListing, 0)
	seenURL := make(map[string]struct{})
	fetchedAt := s.now().UTC()
	for _, hash := range hashes {
		for _, listing := range s.cache[hash] {
			if _, dup := seenURL[listing.URL]; dup {
				continue
			}
			seenURL[listing.URL] = struct{}{}
			listing.FetchedAt = fetchedAt
			listings = append(listings, listing)
		}
	}
	return listings, nil
}

// normalizeExtracted strictly validates one model-claimed listing: it must carry
// a title and an absolute HTTP(S) URL resolvable against the page. Incomplete
// output is dropped rather than trusted.
func (s *LLMGenericSource) normalizeExtracted(extracted ExtractedListing, fetchedAt time.Time) (domain.RawListing, bool) {
	title := strings.TrimSpace(extracted.Title)
	if title == "" {
		return domain.RawListing{}, false
	}
	resolved, err := s.pageURL.Parse(strings.TrimSpace(extracted.URL))
	if err != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Host == "" {
		return domain.RawListing{}, false
	}
	resolved.Fragment = ""
	rawText := strings.TrimSpace(extracted.Summary)
	if rawText == "" {
		rawText = title
	}
	return domain.RawListing{
		Company:   s.company,
		SourceID:  s.name,
		Title:     title,
		URL:       resolved.String(),
		RawText:   rawText,
		FetchedAt: fetchedAt,
	}, true
}

// innermost keeps only matched nodes that do not contain another matched node,
// so a listing card is captured once at its tightest boundary.
func innermost(matched []*html.Node) []*html.Node {
	set := make(map[*html.Node]struct{}, len(matched))
	for _, node := range matched {
		set[node] = struct{}{}
	}
	result := make([]*html.Node, 0, len(matched))
	for _, node := range matched {
		if !containsMatchedDescendant(node, set) {
			result = append(result, node)
		}
	}
	return result
}

func containsMatchedDescendant(node *html.Node, set map[*html.Node]struct{}) bool {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if _, ok := set[child]; ok {
			return true
		}
		if containsMatchedDescendant(child, set) {
			return true
		}
	}
	return false
}

// blockLevelTags separate lines in structuredText so a heading, its description
// and its call-to-action do not collapse into one run of words.
var blockLevelTags = map[string]struct{}{
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
	"p": {}, "li": {}, "div": {}, "section": {}, "article": {},
	"tr": {}, "br": {}, "ul": {}, "ol": {}, "table": {},
}

// structuredText renders an element to text with a line break at every
// block-level boundary, dropping non-content subtrees.
func structuredText(root *html.Node) string {
	lines := make([]string, 0)
	var current strings.Builder
	flush := func() {
		if text := strings.TrimSpace(current.String()); text != "" {
			lines = append(lines, text)
		}
		current.Reset()
	}
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		switch node.Type {
		case html.TextNode:
			if text := strings.TrimSpace(node.Data); text != "" {
				if current.Len() > 0 {
					current.WriteByte(' ')
				}
				current.WriteString(text)
			}
		case html.ElementNode:
			if _, skip := nonContentTags[node.Data]; skip {
				return
			}
			_, isBlock := blockLevelTags[node.Data]
			if isBlock {
				flush()
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
			if isBlock {
				flush()
			}
		}
	}
	walk(root)
	flush()
	return strings.Join(lines, "\n")
}

func blockHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
