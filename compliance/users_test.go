package compliance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ptr(s string) *string { return &s }

func TestFetchUsersPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			json.NewEncoder(w).Encode(UsersResponse{
				Data: []User{
					{ID: "user_01", FullName: "Alice", EmailAddress: "alice@example.com"},
					{ID: "user_02", FullName: "Bob", EmailAddress: "bob@example.com"},
				},
				HasMore:  true,
				NextPage: ptr("page_abc123"),
			})
		case 2:
			if r.URL.Query().Get("page") != "page_abc123" {
				t.Errorf("expected page=page_abc123, got %q", r.URL.Query().Get("page"))
			}
			json.NewEncoder(w).Encode(UsersResponse{
				Data: []User{
					{ID: "user_03", FullName: "Charlie", EmailAddress: "charlie@example.com"},
				},
				HasMore:  false,
				NextPage: nil,
			})
		default:
			t.Error("unexpected page request")
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	c := NewClient("key", "org-test")
	c.baseURL = srv.URL

	users, err := c.FetchUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
	if users[2].FullName != "Charlie" {
		t.Errorf("expected Charlie, got %q", users[2].FullName)
	}
}

func TestUserEffectiveEmail(t *testing.T) {
	tests := []struct {
		name         string
		emailAddress string
		email        string
		want         string
	}{
		{"prefers email_address", "alice@example.com", "alice-alt@example.com", "alice@example.com"},
		{"falls back to email", "", "bob@example.com", "bob@example.com"},
		{"both empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := User{EmailAddress: tt.emailAddress, Email: tt.email}
			if got := u.EffectiveEmail(); got != tt.want {
				t.Errorf("EffectiveEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}
