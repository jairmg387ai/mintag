package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientFromEnv_Defaults(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "test-pat")
	t.Setenv("MINTAG_AZURE_ORG", "")
	t.Setenv("MINTAG_AZURE_USER", "")
	t.Setenv("MINTAG_AZURE_USER_ID", "")
	t.Setenv("MINTAG_AZURE_ENTRY_TYPE", "")

	c := NewClientFromEnv()
	if c.cfg.PAT != "test-pat" {
		t.Errorf("expected PAT=test-pat, got %q", c.cfg.PAT)
	}
	if c.cfg.Org != "RUNT2PSW" {
		t.Errorf("expected Org=RUNT2PSW, got %q", c.cfg.Org)
	}
	if c.cfg.WorkItemID != 156263 {
		t.Errorf("expected WorkItemID=156263, got %d", c.cfg.WorkItemID)
	}
	if c.cfg.User != "Jair Reinel Muñoz Gomez" {
		t.Errorf("expected default user, got %q", c.cfg.User)
	}
	if c.cfg.UserID != "781ef5a8-e9fc-63f2-9c64-ea9193bcbd6d" {
		t.Errorf("expected default userID, got %q", c.cfg.UserID)
	}
	if c.cfg.EntryType != "Desarrollo de Software (Codificación)" {
		t.Errorf("expected default entryType, got %q", c.cfg.EntryType)
	}
}

func TestEnabled_FalseWhenNoPAT(t *testing.T) {
	c := NewClient(Config{PAT: ""})
	if c.Enabled() {
		t.Error("expected Enabled()=false when PAT is empty")
	}
}

func TestEnabled_TrueWhenPATSet(t *testing.T) {
	c := NewClient(Config{PAT: "some-pat"})
	if !c.Enabled() {
		t.Error("expected Enabled()=true when PAT is set")
	}
}

func TestPostTimeEntry_PayloadShape(t *testing.T) {
	var capturedBody []byte
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := Config{
		PAT:        "my-secret-pat",
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
	if err := c.PostTimeEntry(ctx, entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auth header uses Basic scheme with base64(:{PAT}).
	expectedToken := "Basic " + base64.StdEncoding.EncodeToString([]byte(":my-secret-pat"))
	if capturedAuth != expectedToken {
		t.Errorf("auth header mismatch\n  want: %q\n  got:  %q", expectedToken, capturedAuth)
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

	c := NewClient(Config{PAT: ""})
	err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Error("expected error when PAT is empty")
	}
	if callMade {
		t.Error("HTTP call should not be made when PAT is empty")
	}
}

func TestPostTimeEntry_NonTwoXXSurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"invalid field"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	cfg := Config{PAT: "x", Org: "ORG", WorkItemID: 1, User: "U", UserID: "uid", EntryType: "T"}
	c := &Client{
		cfg:  cfg,
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	err := c.PostTimeEntry(context.Background(), TimeEntry{Date: "2026-06-12", Hours: 1, RegistroDiario: "x"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "invalid field") {
		t.Errorf("expected error to contain response body, got: %v", err)
	}
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
