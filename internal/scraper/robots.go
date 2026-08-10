package scraper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	robotsProductToken = "internship-tracker"
	robotsUserAgent    = "internship-tracker/0.1 (+personal career monitoring)"
	maxRobotsBytes     = 512 * 1024
	robotsCacheTTL     = 24 * time.Hour
)

type HTTPRobotsChecker struct {
	client *http.Client
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]robotsCacheEntry
}

type robotsCacheEntry struct {
	fetchedAt time.Time
	policy    robotsPolicy
}

type robotsPolicy struct {
	groups   []robotsGroup
	allowAll bool
	denyAll  bool
	reason   string
}

type robotsGroup struct {
	agents []string
	rules  []robotsRule
}

type robotsRule struct {
	allow       bool
	pattern     *regexp.Regexp
	specificity int
}

func NewHTTPRobotsChecker(client *http.Client, now func() time.Time) *HTTPRobotsChecker {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if now == nil {
		now = time.Now
	}
	return &HTTPRobotsChecker{client: client, now: now, cache: make(map[string]robotsCacheEntry)}
}

func (c *HTTPRobotsChecker) Check(ctx context.Context, access AccessPolicy) (RobotsDecision, error) {
	target, err := url.Parse(strings.TrimSpace(access.TargetURL))
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return RobotsDecision{Reason: "robots.txt target URL is invalid"}, errors.New("robots.txt target URL must be absolute HTTP or HTTPS")
	}
	robotsURL := (&url.URL{Scheme: target.Scheme, Host: target.Host, Path: "/robots.txt"}).String()
	policy, found := c.cached(robotsURL)
	if !found {
		policy, err = c.fetch(ctx, robotsURL)
		if err != nil {
			return RobotsDecision{Reason: "robots.txt could not be verified; access denied"}, err
		}
		c.store(robotsURL, policy)
	}
	if policy.allowAll {
		return RobotsDecision{Allowed: true, Reason: policy.reason}, nil
	}
	if policy.denyAll {
		return RobotsDecision{Reason: policy.reason}, nil
	}

	targetPath := target.EscapedPath()
	if targetPath == "" {
		targetPath = "/"
	}
	if target.RawQuery != "" {
		targetPath += "?" + target.RawQuery
	}
	allowed := policy.allowed(robotsProductToken, targetPath)
	reason := "robots.txt disallows target path"
	if allowed {
		reason = "robots.txt allows target path"
	}
	return RobotsDecision{Allowed: allowed, Reason: reason}, nil
}

func (c *HTTPRobotsChecker) cached(key string) (robotsPolicy, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok || !c.now().Before(entry.fetchedAt.Add(robotsCacheTTL)) {
		if ok {
			delete(c.cache, key)
		}
		return robotsPolicy{}, false
	}
	return entry.policy, true
}

func (c *HTTPRobotsChecker) store(key string, policy robotsPolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = robotsCacheEntry{fetchedAt: c.now(), policy: policy}
}

func (c *HTTPRobotsChecker) fetch(ctx context.Context, robotsURL string) (robotsPolicy, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return robotsPolicy{}, fmt.Errorf("create robots.txt request: %w", err)
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", robotsUserAgent)
	response, err := c.client.Do(request)
	if err != nil {
		return robotsPolicy{}, fmt.Errorf("fetch robots.txt: %w", err)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		body, err := io.ReadAll(io.LimitReader(response.Body, maxRobotsBytes+1))
		if err != nil {
			return robotsPolicy{}, fmt.Errorf("read robots.txt: %w", err)
		}
		if len(body) > maxRobotsBytes {
			return robotsPolicy{}, fmt.Errorf("read robots.txt: response exceeds %d bytes", maxRobotsBytes)
		}
		policy, err := parseRobots(body)
		if err != nil {
			return robotsPolicy{}, fmt.Errorf("parse robots.txt: %w", err)
		}
		return policy, nil
	case response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone:
		return robotsPolicy{allowAll: true, reason: fmt.Sprintf("robots.txt unavailable (HTTP %d)", response.StatusCode)}, nil
	case response.StatusCode >= 400 && response.StatusCode < 500:
		return robotsPolicy{denyAll: true, reason: fmt.Sprintf("robots.txt returned HTTP %d; access denied", response.StatusCode)}, nil
	default:
		return robotsPolicy{}, fmt.Errorf("fetch robots.txt: HTTP status %d", response.StatusCode)
	}
}

func parseRobots(body []byte) (robotsPolicy, error) {
	groups := make([]robotsGroup, 0)
	var current *robotsGroup
	hasRules := false
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(body), "\ufeff")))
	scanner.Buffer(make([]byte, 4096), maxRobotsBytes+1)
	for scanner.Scan() {
		line := scanner.Text()
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		field = strings.ToLower(strings.TrimSpace(field))
		value = strings.TrimSpace(value)
		switch field {
		case "user-agent":
			if value == "" {
				continue
			}
			if current == nil || hasRules {
				groups = append(groups, robotsGroup{})
				current = &groups[len(groups)-1]
				hasRules = false
			}
			current.agents = append(current.agents, strings.ToLower(value))
		case "allow", "disallow":
			if current == nil || value == "" {
				continue
			}
			current.rules = append(current.rules, compileRobotsRule(field == "allow", value))
			hasRules = true
		}
	}
	if err := scanner.Err(); err != nil {
		return robotsPolicy{}, err
	}
	return robotsPolicy{groups: groups}, nil
}

func compileRobotsRule(allow bool, value string) robotsRule {
	anchored := strings.HasSuffix(value, "$")
	patternValue := strings.TrimSuffix(value, "$")
	var expression strings.Builder
	expression.WriteByte('^')
	parts := strings.Split(patternValue, "*")
	for index, part := range parts {
		if index > 0 {
			expression.WriteString(".*")
		}
		expression.WriteString(regexp.QuoteMeta(part))
	}
	if anchored {
		expression.WriteByte('$')
	}
	return robotsRule{
		allow: allow, pattern: regexp.MustCompile(expression.String()),
		specificity: len(strings.ReplaceAll(patternValue, "*", "")),
	}
}

func (p robotsPolicy) allowed(productToken string, targetPath string) bool {
	groups := p.matchingGroups(strings.ToLower(productToken))
	bestSpecificity := -1
	allowed := true
	for _, group := range groups {
		for _, rule := range group.rules {
			if !rule.pattern.MatchString(targetPath) || rule.specificity < bestSpecificity {
				continue
			}
			if rule.specificity > bestSpecificity || rule.allow {
				bestSpecificity = rule.specificity
				allowed = rule.allow
			}
		}
	}
	return allowed
}

func (p robotsPolicy) matchingGroups(productToken string) []robotsGroup {
	exact := make([]robotsGroup, 0)
	wildcard := make([]robotsGroup, 0)
	for _, group := range p.groups {
		for _, agent := range group.agents {
			switch agent {
			case productToken:
				exact = append(exact, group)
			case "*":
				wildcard = append(wildcard, group)
			}
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return wildcard
}
