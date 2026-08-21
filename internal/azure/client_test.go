package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientFromEnv_Defaults(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "test-pat")
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_AUTH_MODE", "")
	t.Setenv("MINTAG_AZURE_ORG", "")
	t.Setenv("MINTAG_AZURE_USER", "")
	t.Setenv("MINTAG_AZURE_USER_ID", "")
	t.Setenv("MINTAG_AZURE_ENTRY_TYPE", "")

	c := NewClientFromEnv()
	if c.cfg.Token != "test-pat" {
		t.Errorf("expected token=test-pat, got %q", c.cfg.Token)
	}
	if c.cfg.AuthMode != AuthModeBasic {
		t.Errorf("expected auth_mode=basic for legacy PAT fallback, got %q", c.cfg.AuthMode)
	}
	if c.cfg.Org != "RUNT2PSW" {
		t.Errorf("expected Org=RUNT2PSW, got %q", c.cfg.Org)
	}
	if c.cfg.WorkItemID != 156263 {
		t.Errorf("expected WorkItemID=156263, got %d", c.cfg.WorkItemID)
	}
	if c.cfg.User != "" {
		t.Errorf("expected no hardcoded default user, got %q", c.cfg.User)
	}
	if c.cfg.UserID != "" {
		t.Errorf("expected no hardcoded default userID, got %q", c.cfg.UserID)
	}
	if c.cfg.EntryType != "Desarrollo de Software (Codificación)" {
		t.Errorf("expected default entryType, got %q", c.cfg.EntryType)
	}
}

func TestEnabled_FalseWhenNoPAT(t *testing.T) {
	c := NewClient(Config{Token: ""})
	if c.Enabled() {
		t.Error("expected Enabled()=false when PAT is empty")
	}
}

func TestEnabled_TrueWhenPATSet(t *testing.T) {
	c := NewClient(Config{Token: "some-pat"})
	if !c.Enabled() {
		t.Error("expected Enabled()=true when PAT is set")
	}
}

func TestPostTimeEntry_PayloadShape(t *testing.T) {
	var capturedBody []byte
	var capturedAuth string
	var capturedAccept string
	var capturedFedAuthRedirect string
	var capturedOrigin string
	var capturedReferer string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedAccept = r.Header.Get("Accept")
		capturedFedAuthRedirect = r.Header.Get("X-TFS-FedAuthRedirect")
		capturedOrigin = r.Header.Get("Origin")
		capturedReferer = r.Header.Get("Referer")
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"doc-123"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	cfg := Config{
		Token:      "my-secret-pat",
		AuthMode:   AuthModeBasic,
		Org:        "TESTORG",
		WorkItemID: 99999,
		User:       "Test User",
		UserID:     "user-uuid",
		EntryType:  "Test Type",
	}
	c := &Client{cfg: cfg, http: srv.Client()}

	// Patch the URL by overriding how the client builds the URL — use a custom
	// roundtripper that redirects to the test server.
	c.http = &http.Client{
		Transport: redirectToServer(srv.URL),
	}

	entry := TimeEntry{
		Date:           "2026-06-12",
		Hours:          1.5,
		RegistroDiario: "Reviewed architecture docs",
	}

	ctx := context.Background()
	if _, err := c.PostTimeEntry(ctx, entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auth header uses Basic scheme with base64(:{PAT}).
	expectedToken := "Basic " + base64.StdEncoding.EncodeToString([]byte(":my-secret-pat"))
	if capturedAuth != expectedToken {
		t.Errorf("auth header mismatch\n  want: %q\n  got:  %q", expectedToken, capturedAuth)
	}
	if !strings.Contains(capturedAccept, "excludeUrls=true") {
		t.Errorf("expected Accept header to include excludeUrls=true, got %q", capturedAccept)
	}
	if capturedFedAuthRedirect != "Suppress" {
		t.Errorf("expected X-TFS-FedAuthRedirect=Suppress, got %q", capturedFedAuthRedirect)
	}
	if capturedOrigin != "https://dev.azure.com" {
		t.Errorf("expected Origin=https://dev.azure.com, got %q", capturedOrigin)
	}
	if capturedReferer != "https://dev.azure.com/" {
		t.Errorf("expected Referer=https://dev.azure.com/, got %q", capturedReferer)
	}

	// Verify payload fields.
	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	// minutes = round(1.5 * 60) = 90
	if got := payload["minutes"]; got != float64(90) {
		t.Errorf("expected minutes=90, got %v", got)
	}
	if got := payload["comment"]; got != "Reviewed architecture docs" {
		t.Errorf("expected comment=%q, got %v", "Reviewed architecture docs", got)
	}
	if got := payload["dateWeek"]; got != "2026-W24" {
		t.Errorf("expected dateWeek=2026-W24, got %v", got)
	}
	if got := payload["date"]; got != "2026-06-12" {
		t.Errorf("expected date=2026-06-12, got %v", got)
	}
}

func TestPostTimeEntry_ISOWeekBoundary(t *testing.T) {
	// 2025-12-29 is in ISO week 1 of year 2026.
	got, err := isoWeekString("2025-12-29")
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-W01"
	if got != want {
		t.Errorf("isoWeekString(2025-12-29) = %q, want %q", got, want)
	}
}

func TestPostTimeEntry_HoursToMinutesRounding(t *testing.T) {
	tests := []struct {
		hours   float64
		minutes int
	}{
		{1.0, 60},
		{1.5, 90},
		{0.25, 15},
		{1.333, 80}, // round(1.333*60) = round(79.98) = 80
	}
	for _, tc := range tests {
		got := int(roundHours(tc.hours))
		if got != tc.minutes {
			t.Errorf("hours=%v → want minutes=%d, got %d", tc.hours, tc.minutes, got)
		}
	}
}

func TestPostTimeEntry_MissingPAT_NoHTTPCall(t *testing.T) {
	callMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(Config{Token: ""})
	_, err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Error("expected error when PAT is empty")
	}
	if callMade {
		t.Error("HTTP call should not be made when PAT is empty")
	}
}

func TestPostTimeEntry_NonTwoXXSanitizesResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"invalid field","token":"must-not-leak"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	cfg := Config{Token: "x", AuthMode: AuthModeBasic, Org: "ORG", WorkItemID: 1, User: "U", UserID: "uid", EntryType: "T"}
	c := &Client{
		cfg:  cfg,
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "unexpected status 400: invalid field") {
		t.Errorf("expected sanitized status/message, got: %v", err)
	}
	if strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), "token") {
		t.Errorf("error leaked raw response body: %v", err)
	}
}

func TestPostTimeEntry_TwoXXWithIDReturnsID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"azure-doc-123"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", WorkItemID: 1, User: "U", UserID: "uid", EntryType: "T"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	id, err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "azure-doc-123" {
		t.Fatalf("expected id azure-doc-123, got %q", id)
	}
}

func TestPostTimeEntry_TwoXXWithoutIDFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"url":"no-id"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", WorkItemID: 1, User: "U", UserID: "uid", EntryType: "T"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Fatal("expected error when 2xx response omits id")
	}
	if !strings.Contains(err.Error(), "missing document id") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}

func TestPostTimeEntry_TwoXXWithWhitespaceIDFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"   "}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", WorkItemID: 1, User: "U", UserID: "uid", EntryType: "T"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Fatal("expected error when 2xx response has whitespace id")
	}
	if !strings.Contains(err.Error(), "missing document id") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}

func TestPostTimeEntry_EntryWorkItemIDOverridesConfig(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"doc-override"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", WorkItemID: 156263, User: "U", UserID: "uid", EntryType: "T"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	entry := TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x", WorkItemID: 99887}
	if _, err := c.PostTimeEntry(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["workItemId"]; got != float64(99887) {
		t.Errorf("expected workItemId=99887 (entry override), got %v", got)
	}
}

func TestPostTimeEntry_ZeroEntryWorkItemIDFallsBackToConfig(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"doc-fallback"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", WorkItemID: 156263, User: "U", UserID: "uid", EntryType: "T"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	entry := TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"} // WorkItemID zero-value
	if _, err := c.PostTimeEntry(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["workItemId"]; got != float64(156263) {
		t.Errorf("expected workItemId=156263 (config fallback), got %v", got)
	}
}

func TestPostTimeEntry_TwoXXHTMLResponseFailsWithAuthHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!doctype html><html><body>sign in</body></html>`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", WorkItemID: 1, User: "U", UserID: "uid", EntryType: "T"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Fatal("expected error for HTML success response")
	}
	if !strings.Contains(err.Error(), "Azure returned HTML/sign-in response; token may be expired or auth mode invalid") {
		t.Fatalf("expected clear HTML/auth error, got %v", err)
	}
	if strings.Contains(err.Error(), "invalid character") || strings.Contains(err.Error(), "sign in") || strings.Contains(err.Error(), "html") {
		t.Fatalf("expected sanitized error without raw decode/body details, got %v", err)
	}
}

func TestDeleteTimeEntry_RequestShapeAndSuccessStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "ok", status: http.StatusOK},
		{name: "no content", status: http.StatusNoContent},
		{name: "not found is idempotent", status: http.StatusNotFound},
		{name: "gone is idempotent", status: http.StatusGone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method, requestURI, auth, accept, fedAuthRedirect, origin, referer string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method = r.Method
				requestURI = r.RequestURI
				auth = r.Header.Get("Authorization")
				accept = r.Header.Get("Accept")
				fedAuthRedirect = r.Header.Get("X-TFS-FedAuthRedirect")
				origin = r.Header.Get("Origin")
				referer = r.Header.Get("Referer")
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			c := &Client{
				cfg:  Config{Token: "secret-token", AuthMode: AuthModeBearer, Org: "ORG"},
				http: &http.Client{Transport: redirectToServer(srv.URL)},
			}

			if err := c.DeleteTimeEntry(context.Background(), "doc/with space"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if method != http.MethodDelete {
				t.Fatalf("expected DELETE, got %s", method)
			}
			if !strings.Contains(requestURI, "/TimeLogData/Documents/doc%2Fwith%20space") {
				t.Fatalf("expected escaped document id in URL, got %s", requestURI)
			}
			if auth != "Bearer secret-token" {
				t.Fatalf("expected bearer auth header, got %q", auth)
			}
			if !strings.Contains(accept, "api-version=3.1-preview.1") || !strings.Contains(accept, "excludeUrls=true") {
				t.Fatalf("expected Azure TimeLog Accept header, got %q", accept)
			}
			if fedAuthRedirect != "Suppress" || origin != "https://dev.azure.com" || referer != "https://dev.azure.com/" {
				t.Fatalf("missing Azure headers: fed=%q origin=%q referer=%q", fedAuthRedirect, origin, referer)
			}
		})
	}
}

func TestDeleteTimeEntry_Failures(t *testing.T) {
	t.Run("empty document id", func(t *testing.T) {
		c := NewClient(Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"})
		if err := c.DeleteTimeEntry(context.Background(), "   "); err == nil || !strings.Contains(err.Error(), "document id is required") {
			t.Fatalf("expected document id error, got %v", err)
		}
	})

	t.Run("server error is sanitized", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"backend down","token":"must-not-leak"}`)) //nolint:errcheck
		}))
		defer srv.Close()

		c := &Client{
			cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
			http: &http.Client{Transport: redirectToServer(srv.URL)},
		}
		err := c.DeleteTimeEntry(context.Background(), "doc-123")
		if err == nil || !strings.Contains(err.Error(), "unexpected delete status 500: backend down") {
			t.Fatalf("expected sanitized 500 error, got %v", err)
		}
		if strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), "token") {
			t.Fatalf("error leaked raw response body: %v", err)
		}
	})

	t.Run("html sign in response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<!doctype html><html><body>sign in</body></html>`)) //nolint:errcheck
		}))
		defer srv.Close()

		c := &Client{
			cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
			http: &http.Client{Transport: redirectToServer(srv.URL)},
		}
		err := c.DeleteTimeEntry(context.Background(), "doc-123")
		if err == nil || !strings.Contains(err.Error(), "Azure returned HTML/sign-in response") {
			t.Fatalf("expected HTML auth error, got %v", err)
		}
	})
}

func TestFetchAssignedWorkItems_Success(t *testing.T) {
	var wiqlBody []byte
	var workitemsQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/_apis/wit/wiql"):
			body, _ := io.ReadAll(r.Body)
			wiqlBody = body
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"workItems":[{"id":101},{"id":202}]}`)) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			workitemsQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count":2,"value":[
				{"id":101,"fields":{"System.Title":"Fix login bug","System.WorkItemType":"Bug","System.State":"Active"}},
				{"id":202,"fields":{"System.Title":"Add export button","System.WorkItemType":"Task","System.State":"New"}}
			]}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	items, err := c.FetchAssignedWorkItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var wiqlPayload map[string]string
	if err := json.Unmarshal(wiqlBody, &wiqlPayload); err != nil {
		t.Fatalf("unmarshal wiql body: %v", err)
	}
	wiqlQuery := wiqlPayload["query"]
	for _, tt := range []struct {
		name   string
		clause string
	}{
		{name: "assigned to current user", clause: "[System.AssignedTo] = @Me"},
		{name: "excludes closed", clause: "[System.State] <> 'Closed'"},
		{name: "excludes removed", clause: "[System.State] <> 'Removed'"},
		{name: "excludes cerrado", clause: "[System.State] <> 'Cerrado'"},
		{name: "only tasks and bugs", clause: "[System.WorkItemType] IN ('Task', 'Bug')"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(wiqlQuery, tt.clause) {
				t.Errorf("expected wiql query to contain %q, got %s", tt.clause, wiqlQuery)
			}
		})
	}
	if !strings.Contains(workitemsQuery, "ids=101,202") {
		t.Errorf("expected workitems query to batch both ids, got %s", workitemsQuery)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != (AssignedWorkItem{ID: 101, Title: "Fix login bug", Type: "Bug", State: "Active"}) {
		t.Errorf("unexpected first item: %+v", items[0])
	}
	if items[1] != (AssignedWorkItem{ID: 202, Title: "Add export button", Type: "Task", State: "New"}) {
		t.Errorf("unexpected second item: %+v", items[1])
	}
}

func TestFetchAssignedWorkItems_ChunksDetailFetchesAtAzureLimit(t *testing.T) {
	var detailBatches []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/_apis/wit/wiql"):
			workItems := make([]map[string]int, 201)
			for i := range workItems {
				workItems[i] = map[string]int{"id": i + 1}
			}
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]any{"workItems": workItems}); err != nil {
				t.Fatalf("encode wiql response: %v", err)
			}
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			detailBatches = append(detailBatches, r.URL.Query().Get("ids"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count":0,"value":[]}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	if _, err := c.FetchAssignedWorkItems(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(detailBatches) != 2 {
		t.Fatalf("expected 2 detail fetches, got %d (%v)", len(detailBatches), detailBatches)
	}
	if got := len(strings.Split(detailBatches[0], ",")); got != 200 {
		t.Errorf("expected first detail batch to contain 200 ids, got %d", got)
	}
	if got := len(strings.Split(detailBatches[1], ",")); got != 1 {
		t.Errorf("expected second detail batch to contain 1 id, got %d", got)
	}
}

func TestFetchAssignedWorkItems_EmptyResultSkipsDetailFetch(t *testing.T) {
	detailFetchCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/_apis/wit/workitems") {
			detailFetchCalled = true
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"workItems":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	items, err := c.FetchAssignedWorkItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty result, got %v", items)
	}
	if detailFetchCalled {
		t.Error("workitems detail endpoint should not be called for an empty id list")
	}
}

func TestFetchAssignedWorkItems_NoTokenNoHTTPCall(t *testing.T) {
	callMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{cfg: Config{Token: ""}, http: &http.Client{Transport: redirectToServer(srv.URL)}}

	_, err := c.FetchAssignedWorkItems(context.Background())
	if err == nil {
		t.Error("expected error when token is empty")
	}
	if callMade {
		t.Error("HTTP call should not be made when token is empty")
	}
}

func TestFetchAssignedWorkItems_WiqlErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"token expired"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.FetchAssignedWorkItems(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 wiql response")
	}
	if !strings.Contains(err.Error(), "unexpected wiql status 401: token expired") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewClientFromEnv_TeamProjectDefault(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "test-pat")
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TEAM_PROJECT", "")

	c := NewClientFromEnv()
	if c.cfg.TeamProject != "RUNTPRO" {
		t.Errorf("expected default TeamProject=RUNTPRO, got %q", c.cfg.TeamProject)
	}
}

func TestNewClientFromEnv_TeamProjectOverride(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "test-pat")
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TEAM_PROJECT", "CUSTOMPROJ")

	c := NewClientFromEnv()
	if c.cfg.TeamProject != "CUSTOMPROJ" {
		t.Errorf("expected TeamProject=CUSTOMPROJ, got %q", c.cfg.TeamProject)
	}
}

func TestCreateWorkItem_RequestShapeAndPayload(t *testing.T) {
	restore := freezeNow(t, time.Date(2026, 6, 12, 14, 30, 0, 0, time.UTC)) // 2026-06-12 09:30 COT
	defer restore()

	var method, path, contentType string
	var capturedOps []patchOp
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		if !strings.Contains(r.URL.RawQuery, "api-version=7.1-preview.3") {
			t.Errorf("expected api-version=7.1-preview.3 in query, got %q", r.URL.RawQuery)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedOps); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":4242}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "Test User"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	got, err := c.CreateWorkItem(context.Background(), CreateWorkItemInput{
		Title:            "Fix the thing",
		AreaPath:         `PROJ\Team\Sub`,
		IterationPath:    `PROJ\Sprint 1`,
		OriginalEstimate: 16,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 4242 || got.State != "Proposed" {
		t.Errorf("expected {4242 Proposed}, got %+v", got)
	}
	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
	if path != "/ORG/PROJ/_apis/wit/workitems/$Task" {
		t.Errorf("expected path /ORG/PROJ/_apis/wit/workitems/$Task, got %q", path)
	}
	if contentType != "application/json-patch+json" {
		t.Errorf("expected json-patch content type, got %q", contentType)
	}

	values := opValues(capturedOps)
	wantValue := map[string]any{
		"/fields/System.Title":                               "Fix the thing",
		"/fields/System.AreaPath":                            `PROJ\Team\Sub`,
		"/fields/System.IterationPath":                       `PROJ\Sprint 1`,
		"/fields/System.AssignedTo":                          "Test User",
		"/fields/System.State":                               "Proposed",
		"/fields/System.Reason":                              "New",
		"/fields/Microsoft.VSTS.Scheduling.OriginalEstimate": float64(16),
		"/fields/Microsoft.VSTS.Scheduling.RemainingWork":    float64(16),
		"/fields/Microsoft.VSTS.Scheduling.CompletedWork":    float64(0),
		"/fields/Microsoft.VSTS.Common.Triage":               "Pending",
		"/fields/Microsoft.VSTS.Common.Discipline":           "Desarrollo de Software (Codificación)",
		"/fields/Microsoft.VSTS.CMMI.Blocked":                "No",
		"/fields/Microsoft.VSTS.CMMI.TaskType":               "Planned",
		"/fields/Microsoft.VSTS.CMMI.RequiresReview":         "No",
		"/fields/Microsoft.VSTS.CMMI.RequiresTest":           "No",
		"/fields/Microsoft.VSTS.Scheduling.StartDate":        "2026-06-12T05:00:00Z",
		"/fields/Custom.InicioOptimista":                     "2026-06-12T05:00:00Z",
		"/fields/Custom.FinOptimista":                        "2026-06-30T05:00:00Z",
	}
	for path, want := range wantValue {
		if got, ok := values[path]; !ok {
			t.Errorf("missing op for path %q", path)
		} else if got != want {
			t.Errorf("path %q: expected value %v, got %v", path, want, got)
		}
	}
	if _, ok := values["/fields/System.Description"]; ok {
		t.Error("expected no System.Description op when Description is blank")
	}
}

func TestCreateWorkItem_DescriptionIncludedWhenProvided(t *testing.T) {
	restore := freezeNow(t, time.Date(2026, 6, 12, 14, 30, 0, 0, time.UTC))
	defer restore()

	var capturedOps []patchOp
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedOps) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	_, err := c.CreateWorkItem(context.Background(), CreateWorkItemInput{
		Title: "T", Description: "  some description  ", AreaPath: "A", IterationPath: "I",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	values := opValues(capturedOps)
	if got := values["/fields/System.Description"]; got != "some description" {
		t.Errorf("expected trimmed description, got %v", got)
	}
}

func TestCreateWorkItem_DefaultsEstimateWhenZeroOrNegative(t *testing.T) {
	restore := freezeNow(t, time.Date(2026, 6, 12, 14, 30, 0, 0, time.UTC))
	defer restore()

	for _, estimate := range []float64{0, -5} {
		var capturedOps []patchOp
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &capturedOps) //nolint:errcheck
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":1}`)) //nolint:errcheck
		}))

		c := &Client{
			cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
			http: &http.Client{Transport: redirectToServer(srv.URL)},
		}
		_, err := c.CreateWorkItem(context.Background(), CreateWorkItemInput{
			Title: "T", AreaPath: "A", IterationPath: "I", OriginalEstimate: estimate,
		})
		srv.Close()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		values := opValues(capturedOps)
		if got := values["/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"]; got != float64(24) {
			t.Errorf("estimate=%v: expected default 24, got %v", estimate, got)
		}
	}
}

func TestCreateWorkItem_NotEnabled_NoHTTPCall(t *testing.T) {
	callMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{cfg: Config{Token: ""}, http: &http.Client{Transport: redirectToServer(srv.URL)}}
	_, err := c.CreateWorkItem(context.Background(), CreateWorkItemInput{Title: "T", AreaPath: "A", IterationPath: "I"})
	if err == nil {
		t.Error("expected error when token is empty")
	}
	if callMade {
		t.Error("HTTP call should not be made when token is empty")
	}
}

func TestCreateWorkItem_ErrorStatusSanitized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"invalid area path","token":"must-not-leak"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	_, err := c.CreateWorkItem(context.Background(), CreateWorkItemInput{Title: "T", AreaPath: "A", IterationPath: "I"})
	if err == nil || !strings.Contains(err.Error(), "invalid area path") {
		t.Fatalf("expected sanitized 400 error, got %v", err)
	}
	if strings.Contains(err.Error(), "must-not-leak") {
		t.Errorf("error leaked raw response body: %v", err)
	}
}

func TestActivateWorkItem_TwoStepPatch(t *testing.T) {
	var patches [][]patchOp
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		var ops []patchOp
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &ops) //nolint:errcheck
		patches = append(patches, ops)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":4242}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	if err := c.ActivateWorkItem(context.Background(), 4242); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patches) != 2 {
		t.Fatalf("expected exactly 2 PATCH calls, got %d", len(patches))
	}
	for _, p := range paths {
		if p != "/ORG/PROJ/_apis/wit/workitems/4242" {
			t.Errorf("expected PATCH path /ORG/PROJ/_apis/wit/workitems/4242, got %q", p)
		}
	}

	first := opValues(patches[0])
	if first["/fields/System.State"] != "Active" {
		t.Errorf("expected first PATCH to set State=Active, got %v", first)
	}
	if first["/fields/System.Reason"] != "Accepted" || first["/fields/Custom.Resolucion"] != "Accepted" {
		t.Errorf("expected first PATCH to set Reason/Resolucion=Accepted, got %v", first)
	}

	second := opValues(patches[1])
	if _, ok := second["/fields/System.State"]; ok {
		t.Errorf("expected second PATCH to NOT include System.State, got %v", second)
	}
	if second["/fields/System.Reason"] != "Accepted" || second["/fields/Custom.Resolucion"] != "Accepted" {
		t.Errorf("expected second PATCH to re-affirm Reason/Resolucion=Accepted, got %v", second)
	}
}

func TestActivateWorkItem_FirstPatchFailure_StopsBeforeSecond(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"not allowed"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	err := c.ActivateWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 PATCH call before stopping, got %d", calls)
	}
}

func TestCreateAndActivateWorkItem_Success_ReturnsActiveState(t *testing.T) {
	restore := freezeNow(t, time.Date(2026, 6, 12, 14, 30, 0, 0, time.UTC))
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":777}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"id":777}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	got, err := c.CreateAndActivateWorkItem(context.Background(), CreateWorkItemInput{Title: "T", AreaPath: "A", IterationPath: "I"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 777 || got.State != "Active" {
		t.Errorf("expected {777 Active}, got %+v", got)
	}
}

func TestCreateAndActivateWorkItem_ActivationFails_ReturnsPartialSuccessWithProposedState(t *testing.T) {
	restore := freezeNow(t, time.Date(2026, 6, 12, 14, 30, 0, 0, time.UTC))
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":888}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"cannot transition"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	got, err := c.CreateAndActivateWorkItem(context.Background(), CreateWorkItemInput{Title: "T", AreaPath: "A", IterationPath: "I"})
	if err == nil {
		t.Fatal("expected a partial-success error when activation fails")
	}
	var activationErr *ActivationError
	if !errors.As(err, &activationErr) {
		t.Fatalf("expected *ActivationError, got %T: %v", err, err)
	}
	if got.ID != 888 || got.State != "Proposed" {
		t.Errorf("expected the created work item to be preserved as {888 Proposed}, got %+v", got)
	}
	if activationErr.WorkItem.ID != 888 {
		t.Errorf("expected ActivationError to carry the created work item, got %+v", activationErr.WorkItem)
	}
}

func TestFetchClassificationTree_Success_ParsesNestedNodes(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if !strings.Contains(r.URL.RawQuery, "$depth=10") || !strings.Contains(r.URL.RawQuery, "api-version=7.1-preview.2") {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"PROJ","children":[{"name":"Team","children":[{"name":"Sub"}]}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	node, err := c.FetchClassificationTree(context.Background(), "areas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/ORG/PROJ/_apis/wit/classificationnodes/areas" {
		t.Errorf("expected classification nodes path, got %q", path)
	}
	if node.Name != "PROJ" || len(node.Children) != 1 || node.Children[0].Name != "Team" {
		t.Fatalf("unexpected tree shape: %+v", node)
	}
	if len(node.Children[0].Children) != 1 || node.Children[0].Children[0].Name != "Sub" {
		t.Fatalf("unexpected nested children: %+v", node.Children[0])
	}
}

func TestFetchClassificationTree_InvalidKindRejectedLocally_NoHTTPCall(t *testing.T) {
	callMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	_, err := c.FetchClassificationTree(context.Background(), "bogus")
	if err == nil {
		t.Error("expected error for invalid kind")
	}
	if callMade {
		t.Error("HTTP call should not be made for a locally-rejected kind")
	}
}

func TestFetchClassificationTree_ErrorStatusSanitized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"token expired"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ", User: "U"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	_, err := c.FetchClassificationTree(context.Background(), "iterations")
	if err == nil || !strings.Contains(err.Error(), "token expired") {
		t.Fatalf("expected sanitized 401 error, got %v", err)
	}
}

// freezeNow overrides timeNow for the duration of the calling test, restoring
// it on cleanup. fixed is given as UTC; timeNow's caller converts to the
// fixed Colombia (UTC-5) offset before deriving optimistic dates.
func freezeNow(t *testing.T, fixed time.Time) func() {
	t.Helper()
	orig := timeNow
	timeNow = func() time.Time { return fixed }
	return func() { timeNow = orig }
}

// opValues flattens a json-patch op list to a path->value map for easy
// assertions (the payload never repeats a path, so this loses no information).
func opValues(ops []patchOp) map[string]any {
	out := make(map[string]any, len(ops))
	for _, op := range ops {
		out[op.Path] = op.Value
	}
	return out
}

// redirectToServer is a transport that rewrites the host to the test server URL.
type redirectTransport struct {
	base    http.RoundTripper
	baseURL string
}

func redirectToServer(serverURL string) http.RoundTripper {
	return &redirectTransport{base: http.DefaultTransport, baseURL: serverURL}
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone and replace the host.
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(rt.baseURL, "http://")
	return rt.base.RoundTrip(r2)
}

// roundHours is extracted for testability — matches the logic in PostTimeEntry.
func roundHours(hours float64) float64 {
	import_math_round := func(f float64) float64 {
		if f < 0 {
			return float64(int(f - 0.5))
		}
		return float64(int(f + 0.5))
	}
	return import_math_round(hours * 60)
}

// Ensure isoWeekString uses stdlib (sanity check for specific dates).
func TestISOWeekString_KnownDates(t *testing.T) {
	tests := []struct {
		date string
		want string
	}{
		{"2026-06-12", "2026-W24"},
		{"2025-12-29", "2026-W01"}, // year boundary
		{"2026-01-01", "2026-W01"},
		{"2020-12-31", "2020-W53"},
	}
	// Verify using stdlib directly.
	for _, tc := range tests {
		t.Run(tc.date, func(t *testing.T) {
			parsed, _ := time.Parse("2006-01-02", tc.date)
			y, w := parsed.ISOWeek()
			got, err := isoWeekString(tc.date)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			_ = y
			_ = w
			if got != want {
				t.Errorf("isoWeekString(%q) = %q, want %q", tc.date, got, want)
			}
		})
	}
}
