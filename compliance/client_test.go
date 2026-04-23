package compliance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAuthHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := NewClient("sk-test-key", "org-123")
	c.baseURL = srv.URL

	var resp ActivitiesResponse
	if err := c.get(context.Background(), "/v1/compliance/activities", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if gotKey != "sk-test-key" {
		t.Errorf("expected x-api-key=sk-test-key, got %q", gotKey)
	}
}

func TestRetryOnRateLimit(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`rate limited`))
			return
		}
		w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := NewClient("key", "org")
	c.baseURL = srv.URL

	var resp ActivitiesResponse
	if err := c.get(context.Background(), "/test", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewClient("key", "org")
	c.baseURL = srv.URL

	var resp ActivitiesResponse
	err := c.get(context.Background(), "/test", nil, &resp)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}
