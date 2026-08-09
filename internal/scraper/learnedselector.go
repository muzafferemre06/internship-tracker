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
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"golang.org/x/net/html"
)

const maxRecipeDocumentRunes = 120000

type RecipeStore interface {
	LoadSourceRecipe(context.Context, string) (domain.SourceRecipe, bool, error)
	SaveSourceRecipe(context.Context, domain.SourceRecipe) (domain.SourceRecipe, error)
	UpdateSourceRecipeSnapshot(context.Context, string, int, int, string) error
}

type RecipeLearningRequest struct {
	SourceKey     string
	Company       string
	PageURL       string
	Document      string
	Reason        string
	CurrentRecipe *domain.SourceRecipe
}

type RecipeLearner interface {
	LearnRecipe(context.Context, RecipeLearningRequest) (domain.SourceRecipe, error)
}

// LearnedSelectorSource runs a versioned model-learned recipe deterministically
// on ordinary scans. The learner is called only for initial setup or when a
// guard proves the current recipe no longer describes the page.
type LearnedSelectorSource struct {
	name    string
	company string
	pageURL *url.URL
	store   RecipeStore
	learner RecipeLearner
	client  *http.Client
	now     func() time.Time
}

func NewLearnedSelectorSource(name, company, pageURL string, store RecipeStore, learner RecipeLearner, client *http.Client) (*LearnedSelectorSource, error) {
	name = strings.TrimSpace(name)
	company = strings.TrimSpace(company)
	if name == "" || company == "" {
		return nil, errors.New("source name and company are required")
	}
	parsed, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("page URL must be an absolute HTTP(S) URL")
	}
	if store == nil || learner == nil {
		return nil, errors.New("recipe store and learner are required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &LearnedSelectorSource{name: name, company: company, pageURL: parsed, store: store, learner: learner, client: client, now: time.Now}, nil
}

func (s *LearnedSelectorSource) Name() string { return s.name }

func (s *LearnedSelectorSource) AccessPolicy() AccessPolicy {
	return AccessPolicy{Scope: s.pageURL.Hostname(), MinimumInterval: 2 * time.Second, BaseCooldown: time.Minute, MaximumCooldown: time.Hour}
}

func (s *LearnedSelectorSource) FetchListings(ctx context.Context) ([]domain.RawListing, error) {
	root, err := s.fetch(ctx)
	if err != nil {
		return nil, err
	}
	recipe, found, err := s.store.LoadSourceRecipe(ctx, s.name)
	if err != nil {
		return nil, err
	}
	if !found {
		return s.learnAndRun(ctx, root, "initial_setup", nil)
	}

	listings, runErr := s.runRecipe(root, recipe)
	reason := ""
	switch {
	case runErr != nil:
		reason = recipeFailureReason(runErr)
	case len(listings) == 0 && recipe.GoldenListingCount > 0:
		reason = "historical_nonzero_to_zero"
	}
	if reason != "" {
		return s.learnAndRun(ctx, root, reason, &recipe)
	}
	fingerprint := listingFingerprint(listings)
	if err := s.store.UpdateSourceRecipeSnapshot(ctx, s.name, recipe.Version, len(listings), fingerprint); err != nil {
		return nil, fmt.Errorf("update learned recipe snapshot: %w", err)
	}
	return listings, nil
}

func (s *LearnedSelectorSource) fetch(ctx context.Context) (*html.Node, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.pageURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create learned-selector request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "internship-tracker/0.1 (+personal career monitoring)")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch learned-selector page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxAccessErrorBodyBytes))
		return nil, fmt.Errorf("fetch learned-selector page: %w", accessError(response, body, s.now().UTC()))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLLMGenericPageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read learned-selector page: %w", err)
	}
	if len(body) > maxLLMGenericPageBytes {
		return nil, fmt.Errorf("read learned-selector page: response exceeds %d bytes", maxLLMGenericPageBytes)
	}
	root, err := html.Parse(bytesReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse learned-selector page: %w", err)
	}
	return root, nil
}

func bytesReader(value []byte) io.Reader { return strings.NewReader(string(value)) }

func (s *LearnedSelectorSource) learnAndRun(ctx context.Context, root *html.Node, reason string, current *domain.SourceRecipe) ([]domain.RawListing, error) {
	learned, err := s.learner.LearnRecipe(ctx, RecipeLearningRequest{
		SourceKey: s.name, Company: s.company, PageURL: s.pageURL.String(),
		Document: recipeDocument(root), Reason: reason, CurrentRecipe: current,
	})
	if err != nil {
		return nil, fmt.Errorf("learn extraction recipe (%s): %w", reason, err)
	}
	learned.SourceKey = s.name
	if err := validateRecipe(learned); err != nil {
		return nil, fmt.Errorf("validate learned recipe: %w", err)
	}
	listings, err := s.runRecipe(root, learned)
	if err != nil {
		return nil, fmt.Errorf("run learned recipe: %w", err)
	}
	if len(listings) == 0 {
		return nil, fmt.Errorf("%w: learned recipe returned zero listings", ErrUnexpectedPage)
	}
	learned.GoldenListingCount = len(listings)
	learned.GoldenFingerprint = listingFingerprint(listings)
	if _, err := s.store.SaveSourceRecipe(ctx, learned); err != nil {
		return nil, fmt.Errorf("save learned recipe: %w", err)
	}
	return listings, nil
}

var (
	errRecipeIdentity = errors.New("recipe identity check failed")
	errRecipeSchema   = errors.New("recipe listing schema failed")
)

func recipeFailureReason(err error) string {
	if errors.Is(err, errRecipeIdentity) {
		return "identity_check_failed"
	}
	return "schema_validation_failed"
}

func (s *LearnedSelectorSource) runRecipe(root *html.Node, recipe domain.SourceRecipe) ([]domain.RawListing, error) {
	if err := validateRecipe(recipe); err != nil {
		return nil, err
	}
	identities := selectNodes(root, recipe.IdentitySelector)
	identityOK := false
	for _, node := range identities {
		if strings.Contains(strings.ToLower(normalizedText(node)), strings.ToLower(strings.TrimSpace(recipe.IdentityText))) {
			identityOK = true
			break
		}
	}
	if !identityOK {
		return nil, errRecipeIdentity
	}

	nodes := selectNodes(root, recipe.ListingSelector)
	listings := make([]domain.RawListing, 0, len(nodes))
	seen := make(map[string]struct{})
	for _, node := range nodes {
		titles := selectNodes(node, recipe.TitleSelector)
		links := selectNodes(node, recipe.LinkSelector)
		if len(titles) != 1 || len(links) != 1 {
			return nil, fmt.Errorf("%w: listing must have exactly one title and link", errRecipeSchema)
		}
		title := strings.TrimSpace(normalizedText(titles[0]))
		href, ok := attribute(links[0], "href")
		resolved, err := s.pageURL.Parse(strings.TrimSpace(href))
		if title == "" || !ok || err != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Host == "" {
			return nil, fmt.Errorf("%w: listing has invalid title or URL", errRecipeSchema)
		}
		resolved.Fragment = ""
		canonical := resolved.String()
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		listings = append(listings, domain.RawListing{
			Company: s.company, SourceID: s.name, Title: title, URL: canonical,
			RawText: normalizedText(node), FetchedAt: s.now().UTC(),
		})
	}
	return listings, nil
}

type simpleSelector struct {
	tag     string
	id      string
	classes []string
}

func validateRecipe(recipe domain.SourceRecipe) error {
	if strings.TrimSpace(recipe.SourceKey) == "" {
		return errors.New("recipe source key is required")
	}
	if strings.TrimSpace(recipe.IdentityText) == "" {
		return errors.New("recipe identity text is required")
	}
	for label, selector := range map[string]string{
		"identity": recipe.IdentitySelector, "listing": recipe.ListingSelector,
		"title": recipe.TitleSelector, "link": recipe.LinkSelector,
	} {
		if _, err := parseSelector(selector); err != nil {
			return fmt.Errorf("%s selector: %w", label, err)
		}
	}
	return nil
}

func parseSelector(value string) ([]simpleSelector, error) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 || len(parts) > 6 {
		return nil, errors.New("selector must contain one to six simple descendant components")
	}
	selectors := make([]simpleSelector, 0, len(parts))
	for _, part := range parts {
		parsed := simpleSelector{}
		mode := byte('t')
		var token strings.Builder
		flush := func() bool {
			value := token.String()
			token.Reset()
			if value == "" {
				return false
			}
			switch mode {
			case 't':
				parsed.tag = strings.ToLower(value)
			case '#':
				if parsed.id != "" {
					return false
				}
				parsed.id = value
			case '.':
				parsed.classes = append(parsed.classes, value)
			}
			return true
		}
		for i, r := range part {
			if r == '#' || r == '.' {
				if i > 0 && !flush() {
					return nil, fmt.Errorf("invalid component %q", part)
				}
				mode = byte(r)
				continue
			}
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
				return nil, fmt.Errorf("unsupported character in component %q", part)
			}
			token.WriteRune(r)
		}
		if !flush() || (parsed.tag == "" && parsed.id == "" && len(parsed.classes) == 0) {
			return nil, fmt.Errorf("invalid component %q", part)
		}
		selectors = append(selectors, parsed)
	}
	return selectors, nil
}

func selectNodes(root *html.Node, selector string) []*html.Node {
	parts, err := parseSelector(selector)
	if err != nil {
		return nil
	}
	result := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && matchesSimple(node, parts[len(parts)-1]) && matchesAncestors(node.Parent, parts[:len(parts)-1]) {
			result = append(result, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walk(child)
	}
	return result
}

func matchesAncestors(node *html.Node, selectors []simpleSelector) bool {
	for i := len(selectors) - 1; i >= 0; i-- {
		for node != nil && !matchesSimple(node, selectors[i]) {
			node = node.Parent
		}
		if node == nil {
			return false
		}
		node = node.Parent
	}
	return true
}

func matchesSimple(node *html.Node, selector simpleSelector) bool {
	if node == nil || node.Type != html.ElementNode || (selector.tag != "" && node.Data != selector.tag) {
		return false
	}
	if selector.id != "" {
		id, ok := attribute(node, "id")
		if !ok || id != selector.id {
			return false
		}
	}
	classValue, _ := attribute(node, "class")
	classes := make(map[string]struct{})
	for _, class := range strings.Fields(classValue) {
		classes[class] = struct{}{}
	}
	for _, class := range selector.classes {
		if _, ok := classes[class]; !ok {
			return false
		}
	}
	return true
}

func listingFingerprint(listings []domain.RawListing) string {
	values := make([]string, 0, len(listings))
	for _, listing := range listings {
		values = append(values, strings.TrimSpace(listing.Title)+"\x00"+strings.TrimSpace(listing.URL))
	}
	sort.Strings(values)
	sum := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(sum[:])
}

// recipeDocument removes executable/non-content subtrees and emits a bounded
// structural HTML view. This is the only page representation sent to a learner.
func recipeDocument(root *html.Node) string {
	var builder strings.Builder
	runeCount := 0
	write := func(value string) {
		if runeCount >= maxRecipeDocumentRunes {
			return
		}
		runes := []rune(value)
		remaining := maxRecipeDocumentRunes - runeCount
		if len(runes) > remaining {
			runes = runes[:remaining]
		}
		builder.WriteString(string(runes))
		runeCount += len(runes)
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if runeCount >= maxRecipeDocumentRunes {
			return
		}
		switch node.Type {
		case html.TextNode:
			text := strings.TrimSpace(node.Data)
			if text != "" {
				write(text)
				write(" ")
			}
		case html.ElementNode:
			if _, skip := nonContentTags[node.Data]; skip {
				return
			}
			write("<")
			write(node.Data)
			for _, key := range []string{"id", "class", "href"} {
				if value, ok := attribute(node, key); ok {
					write(" " + key + "=\"")
					write(strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;").Replace(value))
					write("\"")
				}
			}
			write(">")
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if node.Type == html.ElementNode {
			if _, skip := nonContentTags[node.Data]; !skip {
				write("</")
				write(node.Data)
				write(">")
			}
		}
	}
	walk(root)
	return builder.String()
}
