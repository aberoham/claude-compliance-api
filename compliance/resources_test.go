package compliance

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testResponse(status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testClient(transport roundTripFunc) *Client {
	client := NewClient("test-key", "org-default")
	client.baseURL = "https://compliance.test"
	client.httpClient = &http.Client{Transport: transport}
	return client
}

func TestFetchOrganizationsFollowsNextPageAcrossEmptyPage(t *testing.T) {
	var calls int32
	client := testClient(func(request *http.Request) (*http.Response, error) {
		call := atomic.AddInt32(&calls, 1)
		if request.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing API key header")
		}
		switch call {
		case 1:
			if got := request.URL.Query().Get("limit"); got != "1000" {
				t.Errorf("limit = %q, want 1000", got)
			}
			return testResponse(200, `{"data":[{"uuid":"one","name":"One"}],"has_more":true,"next_page":"p2"}`, nil), nil
		case 2:
			if got := request.URL.Query().Get("page"); got != "p2" {
				t.Errorf("page = %q, want p2", got)
			}
			return testResponse(200, `{"data":[],"has_more":true,"next_page":"p3"}`, nil), nil
		case 3:
			return testResponse(200, `{"data":[{"uuid":"two","name":"Two"}],"has_more":false}`, nil), nil
		default:
			t.Fatalf("unexpected request %d", call)
			return nil, nil
		}
	})

	organizations, err := client.FetchOrganizations(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(organizations) != 2 || organizations[0].UUID != "one" || organizations[1].UUID != "two" {
		t.Fatalf("unexpected organizations: %#v", organizations)
	}
}

func TestOrganizationPageCallbackCanResume(t *testing.T) {
	var calls int32
	client := testClient(func(request *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		if got := request.URL.Query().Get("page"); got != "saved-cursor" {
			t.Errorf("page = %q, want saved-cursor", got)
		}
		return testResponse(200, `{"data":[{"uuid":"one","name":"One"}],"has_more":true,"next_page":"next-cursor"}`, nil), nil
	})
	var next string
	err := client.ForEachOrganizationPage(context.Background(), ListOptions{Page: "saved-cursor"}, func(values []Organization, cursor CursorPage) error {
		if len(values) != 1 || values[0].UUID != "one" {
			t.Fatalf("values = %#v", values)
		}
		next = cursor.NextCursor
		return context.Canceled
	})
	if err != context.Canceled || next != "next-cursor" || calls != 1 {
		t.Fatalf("err=%v next=%q calls=%d", err, next, calls)
	}
}

func TestDeleteFileUsesDeleteAndReturnsReceipt(t *testing.T) {
	client := testClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", request.Method)
		}
		if request.URL.EscapedPath() != "/v1/compliance/apps/chats/files/claude_file_1" {
			t.Errorf("path = %s", request.URL.EscapedPath())
		}
		return testResponse(200, `{"id":"claude_file_1","type":"claude_file_deleted"}`, nil), nil
	})

	receipt, err := client.DeleteFile(context.Background(), "claude_file_1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Type != "claude_file_deleted" {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
}

func TestDownloadGeneratedFilePreservesHeaders(t *testing.T) {
	headers := http.Header{
		"Content-Disposition": {`attachment; filename="report.pdf"`},
		"Content-Type":        {"application/pdf"},
		"Content-Md5":         {"YWJj"},
	}
	client := testClient(func(request *http.Request) (*http.Response, error) {
		return testResponse(200, "pdf bytes", headers), nil
	})

	download, err := client.DownloadGeneratedFile(context.Background(), "claude_gen_file_1")
	if err != nil {
		t.Fatal(err)
	}
	defer download.Body.Close()
	body, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatal(err)
	}
	if download.Filename != "report.pdf" || download.ContentType != "application/pdf" || download.ContentMD5 != "YWJj" {
		t.Fatalf("unexpected download metadata: %#v", download)
	}
	if string(body) != "pdf bytes" {
		t.Fatalf("body = %q", body)
	}
}

func TestEffectiveSettingsPreserveUnknownValueShape(t *testing.T) {
	client := testClient(func(request *http.Request) (*http.Response, error) {
		return testResponse(200, `{"organization_id":"org-uuid","settings":[{"name":"future_setting","type":"future","value":{"enabled":true}}]}`, nil), nil
	})

	settings, err := client.GetOrganizationSettings(context.Background(), "org-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Settings) != 1 || string(settings.Settings[0].Value) != `{"enabled":true}` {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestRetriesDocumentedTransientStatus(t *testing.T) {
	var calls int32
	client := testClient(func(request *http.Request) (*http.Response, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return testResponse(http.StatusServiceUnavailable, "not ready", http.Header{"Retry-After": {"0"}}), nil
		}
		return testResponse(200, `{"id":"rbac_group_1","name":"Engineering"}`, nil), nil
	})

	group, err := client.GetGroup(context.Background(), "rbac_group_1")
	if err != nil {
		t.Fatal(err)
	}
	if group.Name != "Engineering" || atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("group=%#v calls=%d", group, calls)
	}
}

func TestInternalServerErrorCanDisableRetry(t *testing.T) {
	var calls int32
	client := testClient(func(request *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return testResponse(500, "do not retry", http.Header{
			"X-Should-Retry": {"false"},
			"Request-Id":     {"req_123"},
		}), nil
	})

	_, err := client.GetGroup(context.Background(), "rbac_group_1")
	apiError, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiError.RequestID != "req_123" || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("error=%#v calls=%d", apiError, calls)
	}
}

func TestActivityQuerySupportsCurrentFilters(t *testing.T) {
	client := testClient(func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		checks := map[string]string{
			"created_at.gt":            "2026-07-01T00:00:00Z",
			"created_at.lte":           "2026-07-02T00:00:00Z",
			"user_ids[]":               "user_1",
			"organization_ids[]":       "org_uuid",
			"exclude_activity_types[]": "compliance_api_accessed",
			"order":                    "asc",
		}
		for name, want := range checks {
			if got := query.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		return testResponse(200, `{"data":[{"id":"a1","type":"compliance_api_accessed","created_at":"2026-07-01T00:00:00Z","actor":{"type":"api_actor"}}],"has_more":false}`, nil), nil
	})
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	activities, err := client.FetchActivities(context.Background(), ActivityQuery{
		CreatedAtGT:                  &start,
		CreatedAtLTE:                 &end,
		UserIDs:                      []string{"user_1"},
		OrganizationIDs:              []string{"org_uuid"},
		ExcludeActivityTypes:         []string{"compliance_api_accessed"},
		IncludeComplianceAPIAccessed: true,
		Order:                        "asc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 || activities[0].Type != "compliance_api_accessed" {
		t.Fatalf("unexpected activities: %#v", activities)
	}
}

func TestFetchChatMessagesSupportsToolBlocksAndPagination(t *testing.T) {
	var calls int32
	maxChars := -1
	client := testClient(func(request *http.Request) (*http.Response, error) {
		call := atomic.AddInt32(&calls, 1)
		query := request.URL.Query()
		if query.Get("limit") != "1" || query.Get("tool_result_max_chars") != "-1" {
			t.Errorf("unexpected query: %s", request.URL.RawQuery)
		}
		if call == 1 {
			return testResponse(200, `{"id":"chat_1","name":"Chat","chat_messages":[{"id":"m1","role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"search","input":"{\"q\":\"x\"}"}]}],"has_more":true,"first_id":"first","last_id":"next"}`, nil), nil
		}
		if query.Get("after_id") != "next" {
			t.Errorf("after_id = %q, want next", query.Get("after_id"))
		}
		return testResponse(200, `{"id":"chat_1","chat_messages":[{"id":"m2","role":"assistant","generated_files":[{"id":"gf1","filename":"report.pdf","size_bytes":10}]}],"has_more":false,"first_id":"next","last_id":"last"}`, nil), nil
	})

	chat, err := client.FetchChatMessages(context.Background(), "chat_1", ChatMessageQuery{
		Limit:              1,
		ToolResultMaxChars: &maxChars,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chat.ChatMessages) != 2 || chat.ChatMessages[0].Content[0].Type != "tool_use" || len(chat.ChatMessages[1].GeneratedFiles) != 1 {
		t.Fatalf("unexpected chat: %#v", chat)
	}
}

func TestCodeArtifactFiltersUseArraySyntax(t *testing.T) {
	client := testClient(func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		if query.Get("organization_ids[]") != "org_uuid" || query.Get("user_ids[]") != "user_1" {
			t.Errorf("unexpected query: %s", request.URL.RawQuery)
		}
		return testResponse(200, `{"data":[],"has_more":false}`, nil), nil
	})
	_, err := client.FetchCodeArtifacts(context.Background(), CodeArtifactQuery{
		OrganizationIDs: []string{"org_uuid"},
		UserIDs:         []string{"user_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
}
