package okta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookupUserByEmailHit(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "00uABCDEF123",
					"profile": map[string]string{
						// Real-world AD-mastered case: login on the
						// corporate domain, email on a separate proxy
						// domain. The two localparts differ (no shared
						// root), which is exactly what breaks an
						// email-keyed join.
						"login": "userlogin@corp.example.com",
						"email": "first.last@example.com",
					},
				},
			})
		}))
	defer srv.Close()

	u, err := testClient(srv.URL).LookupUserByEmail(
		context.Background(),
		"first.last@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Fatal("expected hit, got nil")
	}
	if u.ID != "00uABCDEF123" {
		t.Errorf("id: got %q", u.ID)
	}
	if u.Login != "userlogin@corp.example.com" {
		t.Errorf("login: got %q", u.Login)
	}
	if u.Email != "first.last@example.com" {
		t.Errorf("email: got %q", u.Email)
	}
	if gotPath != "/api/v1/users" {
		t.Errorf("path: got %q", gotPath)
	}
	if want := `search=profile.email+eq+%22first.last%40example.com%22`; !strings.Contains(gotQuery, want) {
		t.Errorf("query missing %q in %q", want, gotQuery)
	}
}

func TestLookupUserByEmailMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}))
	defer srv.Close()

	u, err := testClient(srv.URL).LookupUserByEmail(
		context.Background(), "nobody@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Errorf("expected nil miss, got %+v", u)
	}
}

func TestLookupUserByEmailLowercasesProfileFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "00uABC",
					"profile": map[string]string{
						"login": "Alice@Example.COM",
						"email": "Alice@Example.COM",
					},
				},
			})
		}))
	defer srv.Close()

	u, err := testClient(srv.URL).LookupUserByEmail(
		context.Background(), "Alice@Example.COM",
	)
	if err != nil {
		t.Fatal(err)
	}
	if u.Login != "alice@example.com" || u.Email != "alice@example.com" {
		t.Errorf("login/email not lowercased: %+v", u)
	}
}

func TestLookupUserByEmailEmpty(t *testing.T) {
	_, err := testClient("http://unused").LookupUserByEmail(
		context.Background(), "",
	)
	if err == nil {
		t.Fatal("expected error for empty email")
	}
}

