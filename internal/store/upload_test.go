package store

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// fakeAzureServer creates a test server. responder receives the request index
// (0-based call count) and returns the status code to use.
func fakeAzureServer(t *testing.T, responder func(callN int) int) *httptest.Server {
	t.Helper()
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := responder(n)
		n++
		w.WriteHeader(code)
		if code >= 400 {
			w.Write([]byte(`{"message":"simulated azure error"}`)) //nolint:errcheck
		} else {
			w.Write([]byte(`{"id":"azure-doc-` + strconv.Itoa(n) + `"}`)) //nolint:errcheck
		}
	}))
	return srv
}

// azureClientForTest wires an azure.Client to use the given test server.
func azureClientForTest(srv *httptest.Server) *azure.Client {
	cfg := azure.Config{
		Token:      "test-pat",
		AuthMode:   azure.AuthModeBasic,
		Org:        "TESTORG",
		WorkItemID: 1,
		User:       "Test",
		UserID:     "uid",
		EntryType:  "T",
	}
	c := azure.NewClient(cfg)
	// Replace the internal http.Client transport to redirect calls to the test server.
	c.SetHTTPClient(&http.Client{
		Transport: uploadRedirectToServer(srv.URL),
	})
	return c
}

// uploadRedirectTransport rewrites all request hosts to the given base URL (test server).
type uploadRedirectTransport struct {
	base    http.RoundTripper
	baseURL string
}

func uploadRedirectToServer(baseURL string) http.RoundTripper {
	return &uploadRedirectTransport{base: http.DefaultTransport, baseURL: baseURL}
}

func (rt *uploadRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(rt.baseURL, "http://")
	return rt.base.RoundTrip(r2)
}

func TestUploadActivities_FullSuccess(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	// Create 3 pending activities and approve them.
	var ids []int64
	for i := 0; i < 3; i++ {
		a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	if _, err := s.ApproveActivities(ctx, ids); err != nil {
		t.Fatal(err)
	}

	// All calls succeed.
	srv := fakeAzureServer(t, func(_ int) int { return http.StatusOK })
	defer srv.Close()
	az := azureClientForTest(srv)

	result, err := s.UploadActivities(ctx, "2026-06-12", az)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 3 {
		t.Errorf("expected uploaded_count=3, got %d", result.UploadedCount)
	}
	if len(result.FailedIDs) != 0 {
		t.Errorf("expected no failures, got %v", result.FailedIDs)
	}

	// Verify all rows are now uploaded.
	for _, id := range ids {
		a, err := s.GetActivity(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Status != "uploaded" {
			t.Errorf("expected activity %d status=uploaded, got %q", id, a.Status)
		}
		if a.AzureDocumentID == nil || *a.AzureDocumentID == "" {
			t.Errorf("expected activity %d to store azure_document_id", id)
		}
	}
}

func TestUploadActivities_TwoXXWithoutIDLeavesApproved(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"url":"created-but-no-id"}`)) //nolint:errcheck
	}))
	defer srv.Close()
	az := azureClientForTest(srv)

	result, err := s.UploadActivities(ctx, "2026-06-12", az)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 0 {
		t.Fatalf("expected no uploads, got %d", result.UploadedCount)
	}
	if len(result.FailedIDs) != 1 || result.FailedIDs[0] != a.ID {
		t.Fatalf("expected failed id %d, got %v", a.ID, result.FailedIDs)
	}
	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "approved" {
		t.Fatalf("expected activity to remain approved, got %q", updated.Status)
	}
	if updated.AzureDocumentID != nil {
		t.Fatalf("expected azure_document_id to remain empty, got %v", *updated.AzureDocumentID)
	}
}

func TestUploadActivities_PartialFailure(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	var ids []int64
	for i := 0; i < 3; i++ {
		a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	if _, err := s.ApproveActivities(ctx, ids); err != nil {
		t.Fatal(err)
	}

	// Entry index 1 (second call) fails with 400.
	srv := fakeAzureServer(t, func(n int) int {
		if n == 1 {
			return http.StatusBadRequest
		}
		return http.StatusOK
	})
	defer srv.Close()
	az := azureClientForTest(srv)

	result, err := s.UploadActivities(ctx, "2026-06-12", az)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 2 {
		t.Errorf("expected uploaded_count=2, got %d", result.UploadedCount)
	}
	if len(result.FailedIDs) != 1 {
		t.Errorf("expected 1 failed ID, got %v", result.FailedIDs)
	}

	// The failed one must remain approved.
	failed, err := s.GetActivity(ctx, result.FailedIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "approved" {
		t.Errorf("expected failed activity to remain approved, got %q", failed.Status)
	}
}

func TestUploadActivities_NoApprovedActivities(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	httpCallMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCallMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	az := azureClientForTest(srv)

	result, err := s.UploadActivities(ctx, "2026-06-12", az)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 0 {
		t.Errorf("expected 0 uploads, got %d", result.UploadedCount)
	}
	if httpCallMade {
		t.Error("no HTTP calls should be made when there are no approved activities")
	}
}

// TestUploadActivities_NullActivityResolvesToSeededDefault verifies that a
// daily_activities row with AzureActivityID=nil resolves to whatever
// azure_activities row currently has is_default=1 (the seeded 156263 row on
// a fresh store) and that the resolved work_item_id actually reaches the
// Azure payload, not just cfg.WorkItemID from azureClientForTest (which is
// intentionally set to a different value so this test cannot pass by
// accident).
func TestUploadActivities_NullActivityResolvesToSeededDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}

	var capturedWorkItemID float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		capturedWorkItemID, _ = payload["workItemId"].(float64)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"azure-doc-default"}`)) //nolint:errcheck
	}))
	defer srv.Close()
	// azureClientForTest uses cfg.WorkItemID=1, deliberately different from
	// the seeded default (156263), so this only passes if resolution
	// actually reads from the azure_activities catalog.
	az := azureClientForTest(srv)

	result, err := s.UploadActivities(ctx, "2026-06-12", az)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 1 {
		t.Fatalf("expected 1 upload, got %d (errors: %v)", result.UploadedCount, result.Errors)
	}
	if capturedWorkItemID != 156263 {
		t.Errorf("expected workItemId=156263 (seeded default), got %v", capturedWorkItemID)
	}
}

// TestUploadActivities_ExplicitActivityOverridesDefault seeds a non-default
// azure activity, assigns it to a daily_activities row, and verifies the
// uploaded payload targets that activity's work_item_id rather than the
// default's — exercising per-entry resolution, not just the default path.
func TestUploadActivities_ExplicitActivityOverridesDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	nonDefault, err := s.AddAzureActivity(ctx, "RUNT2PSW", 999111, "QA Activity", "")
	if err != nil {
		t.Fatal(err)
	}
	if nonDefault.IsDefault {
		t.Fatal("expected the second-added activity to NOT be default (seeded default already exists)")
	}

	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetActivityAzureActivity(ctx, a.ID, &nonDefault.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}

	var capturedWorkItemID float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		capturedWorkItemID, _ = payload["workItemId"].(float64)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"azure-doc-explicit"}`)) //nolint:errcheck
	}))
	defer srv.Close()
	az := azureClientForTest(srv)

	result, err := s.UploadActivities(ctx, "2026-06-12", az)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 1 {
		t.Fatalf("expected 1 upload, got %d (errors: %v)", result.UploadedCount, result.Errors)
	}
	if capturedWorkItemID != 999111 {
		t.Errorf("expected workItemId=999111 (explicit non-default activity), got %v", capturedWorkItemID)
	}
}

func TestUploadActivities_NilAzureClient(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	_, err = s.UploadActivities(ctx, "2026-06-12", nil)
	if err == nil {
		t.Error("expected error when azure client is nil")
	}
}
