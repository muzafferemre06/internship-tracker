package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestHTTPRobotsCheckerAppliesRFC9309GroupsAndLongestMatch(t *testing.T) {
	fixture, err := os.ReadFile("testdata/robots/phase14.txt")
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != "https://careers.example.test/robots.txt" {
			t.Fatalf("unexpected robots URL: %s", request.URL)
		}
		if request.Header.Get("User-Agent") != robotsUserAgent {
			t.Fatalf("unexpected user agent: %q", request.Header.Get("User-Agent"))
		}
		return robotsResponse(request, http.StatusOK, fixture), nil
	})}
	checker := NewHTTPRobotsChecker(client, func() time.Time {
		return time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	})

	cases := []struct {
		path    string
		allowed bool
	}{
		{path: "/jobs/internships/backend", allowed: true},
		{path: "/jobs/private/42", allowed: false},
		{path: "/jobs/private/public", allowed: true},
		{path: "/jobs/private/public/child", allowed: false},
		{path: "/jobs/internships/brief.pdf", allowed: false},
		// Exact product groups exist, so the wildcard group's /jobs/ rule is ignored.
		{path: "/jobs/open", allowed: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			decision, err := checker.Check(context.Background(), AccessPolicy{
				Mode: "robots", Scope: "careers.example.test",
				TargetURL: "https://careers.example.test" + testCase.path,
			})
			if err != nil {
				t.Fatalf("check robots: %v", err)
			}
			if decision.Allowed != testCase.allowed {
				t.Fatalf("allowed=%t, want %t; decision=%#v", decision.Allowed, testCase.allowed, decision)
			}
		})
	}
	if requests != 1 {
		t.Fatalf("robots policy must be cached per domain, got %d requests", requests)
	}
}

func TestHTTPRobotsCheckerUsesWildcardGroupWhenProductGroupIsAbsent(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return robotsResponse(request, http.StatusOK, []byte("User-agent: *\nDisallow: /private\nAllow: /private/public\n")), nil
	})}
	checker := NewHTTPRobotsChecker(client, time.Now)
	for _, testCase := range []struct {
		url     string
		allowed bool
	}{
		{url: "https://careers.example.test/private/report", allowed: false},
		{url: "https://careers.example.test/private/public", allowed: true},
	} {
		decision, err := checker.Check(context.Background(), AccessPolicy{Mode: "robots", TargetURL: testCase.url})
		if err != nil || decision.Allowed != testCase.allowed {
			t.Fatalf("check %s: decision=%#v err=%v", testCase.url, decision, err)
		}
	}
}

func TestHTTPRobotsCheckerFailsClosedExceptForUnavailable404(t *testing.T) {
	transportError := errors.New("network unavailable")
	cases := []struct {
		name       string
		statusCode int
		transport  error
		allowed    bool
		wantError  bool
	}{
		{name: "not found", statusCode: http.StatusNotFound, allowed: true},
		{name: "forbidden", statusCode: http.StatusForbidden, allowed: false},
		{name: "server failure", statusCode: http.StatusServiceUnavailable, allowed: false, wantError: true},
		{name: "network failure", transport: transportError, allowed: false, wantError: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if testCase.transport != nil {
					return nil, testCase.transport
				}
				return robotsResponse(request, testCase.statusCode, nil), nil
			})}
			decision, err := NewHTTPRobotsChecker(client, time.Now).Check(context.Background(), AccessPolicy{
				Mode: "robots", TargetURL: "https://careers.example.test/jobs/1",
			})
			if (err != nil) != testCase.wantError {
				t.Fatalf("error=%v, wantError=%t", err, testCase.wantError)
			}
			if decision.Allowed != testCase.allowed {
				t.Fatalf("decision=%#v, want allowed=%t", decision, testCase.allowed)
			}
		})
	}
}

func TestHTTPRobotsCheckerExpiresCacheAfter24Hours(t *testing.T) {
	now := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return robotsResponse(request, http.StatusOK, []byte("User-agent: *\nAllow: /\n")), nil
	})}
	checker := NewHTTPRobotsChecker(client, func() time.Time { return now })
	policy := AccessPolicy{Mode: "robots", TargetURL: "https://careers.example.test/jobs/1"}
	if _, err := checker.Check(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24*time.Hour - time.Second)
	if _, err := checker.Check(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, err := checker.Check(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("expected cache refresh after 24 hours, got %d requests", requests)
	}
}

func TestHTTPRobotsCheckerRejectsInvalidTargetAndOversizedPolicy(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return robotsResponse(request, http.StatusOK, bytes.Repeat([]byte("x"), maxRobotsBytes+1)), nil
	})}
	checker := NewHTTPRobotsChecker(client, time.Now)

	decision, err := checker.Check(context.Background(), AccessPolicy{Mode: "robots", TargetURL: "/relative"})
	if err == nil || decision.Allowed {
		t.Fatalf("invalid target must fail closed: decision=%#v err=%v", decision, err)
	}
	if requests != 0 {
		t.Fatalf("invalid target must not make an HTTP request, got %d", requests)
	}

	decision, err = checker.Check(context.Background(), AccessPolicy{
		Mode: "robots", TargetURL: "https://careers.example.test/jobs",
	})
	if err == nil || decision.Allowed {
		t.Fatalf("oversized robots policy must fail closed: decision=%#v err=%v", decision, err)
	}
}

func robotsResponse(request *http.Request, statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    request,
	}
}
