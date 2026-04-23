package okta

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSSWSAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Write([]byte(`[]`))
		}))
	defer srv.Close()

	c := NewClient("xoxb-test-token", "test.okta.com")
	_, _, err := c.doRequest(context.Background(), srv.URL+"/api/v1/logs")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "SSWS xoxb-test-token" {
		t.Errorf("expected SSWS auth header, got %q", gotAuth)
	}
}

func TestRateLimitReset(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n == 1 {
				resetAt := time.Now().Add(1 * time.Second).Unix()
				w.Header().Set("X-Rate-Limit-Reset",
					fmt.Sprintf("%d", resetAt))
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`rate limited`))
				return
			}
			w.Write([]byte(`[]`))
		}))
	defer srv.Close()

	c := NewClient("token", "test.okta.com")
	_, _, err := c.doRequest(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestLinkHeaderPagination(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "standard next link",
			link:     `<https://test.okta.com/api/v1/logs?after=abc>; rel="next"`,
			expected: "https://test.okta.com/api/v1/logs?after=abc",
		},
		{
			name:     "multiple links",
			link:     `<https://test.okta.com/self>; rel="self", <https://test.okta.com/next>; rel="next"`,
			expected: "https://test.okta.com/next",
		},
		{
			name:     "no next link",
			link:     `<https://test.okta.com/self>; rel="self"`,
			expected: "",
		},
		{
			name:     "empty link",
			link:     "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkNext(tt.link)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
		}))
	defer srv.Close()

	c := NewClient("token", "test.okta.com")
	_, _, err := c.doRequest(context.Background(), srv.URL+"/test")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestRateLimitWaitFallback(t *testing.T) {
	h := http.Header{}
	wait := rateLimitWait(h)
	if wait != 5*time.Second {
		t.Errorf("expected 5s fallback, got %v", wait)
	}
}
