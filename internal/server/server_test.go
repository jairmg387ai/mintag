package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newTestServer(t *testing.T) (baseURL string, st *store.Store) {
	t.Helper()
	st, err := store.OpenInMemory()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ts := httptest.NewServer(New(st).Handler())
	t.Cleanup(func() { ts.Close(); st.Close() })
	return ts.URL, st
}

// get performs a GET request and asserts HTTP 200.
func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	mustNoErr(t, err)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("GET %s: expected 200, got %d — %s", url, resp.StatusCode, string(body))
	}
	return resp
}

// postJSON performs a POST with a JSON body and asserts HTTP 200.
func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	mustNoErr(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	mustNoErr(t, err)
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("POST %s: expected 200, got %d — %s", url, resp.StatusCode, string(respBody))
	}
	return resp
}

// decodeJSON decodes a JSON response body into v.
func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	mustNoErr(t, json.NewDecoder(resp.Body).Decode(v))
}

// TestStats_Empty verifies GET /api/stats returns 200 with a Stats object.
func TestStats_Empty(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/stats")
	var stats map[string]any
	decodeJSON(t, resp, &stats)

	if _, ok := stats["total_tasks"]; !ok {
		t.Error("expected 'total_tasks' field in stats response")
	}
}

// TestProjects_CRUD verifies POST /api/projects and GET /api/projects.
func TestProjects_CRUD(t *testing.T) {
	base, _ := newTestServer(t)

	// POST — create a project
	resp := postJSON(t, base+"/api/projects", map[string]any{
		"name":        "Proj",
		"description": "",
		"color":       "#aabbcc",
	})
	var proj map[string]any
	decodeJSON(t, resp, &proj)

	id, ok := proj["id"]
	if !ok {
		t.Fatal("expected 'id' in create project response")
	}
	if id.(float64) <= 0 {
		t.Errorf("expected id > 0, got %v", id)
	}

	// GET — list projects
	resp2 := get(t, base+"/api/projects")
	var projects []map[string]any
	decodeJSON(t, resp2, &projects)

	if projects == nil {
		t.Error("expected non-null projects array")
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

// TestTasks_CRUD verifies POST /api/tasks, GET /api/tasks, PUT /api/tasks/{id},
// and GET /api/tasks/{id}/history.
func TestTasks_CRUD(t *testing.T) {
	base, _ := newTestServer(t)

	// POST — create a task
	resp := postJSON(t, base+"/api/tasks", map[string]any{
		"title":    "Do work",
		"status":   "todo",
		"priority": "medium",
	})
	var task map[string]any
	decodeJSON(t, resp, &task)

	id, ok := task["id"]
	if !ok {
		t.Fatal("expected 'id' in create task response")
	}
	taskID := int(id.(float64))
	if taskID <= 0 {
		t.Errorf("expected id > 0, got %v", taskID)
	}

	// GET — list tasks
	resp2 := get(t, base+"/api/tasks")
	var tasks []map[string]any
	decodeJSON(t, resp2, &tasks)
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	// PUT — update status
	putURL := fmt.Sprintf("%s/api/tasks/%d", base, taskID)
	putReq, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(`{"status":"done","note":"done","author":"Alice"}`))
	mustNoErr(t, err)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	mustNoErr(t, err)
	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		putResp.Body.Close()
		t.Fatalf("PUT %s: expected 200, got %d — %s", putURL, putResp.StatusCode, string(body))
	}
	var updated map[string]any
	decodeJSON(t, putResp, &updated)
	if updated["status"] != "done" {
		t.Errorf("expected status 'done' after update, got %v", updated["status"])
	}

	// GET /api/tasks/{id}/history
	histResp := get(t, fmt.Sprintf("%s/api/tasks/%d/history", base, taskID))
	var history []map[string]any
	decodeJSON(t, histResp, &history)
	if history == nil {
		t.Error("expected non-null history array")
	}
	if len(history) == 0 {
		t.Error("expected at least one history entry")
	}
}

// TestMeetings_Import_And_Get verifies POST /api/meetings/import and
// GET /api/meetings/{id}.
func TestMeetings_Import_And_Get(t *testing.T) {
	base, _ := newTestServer(t)

	// create a temp .txt file
	f, err := os.CreateTemp(t.TempDir(), "*.txt")
	mustNoErr(t, err)
	_, err = f.WriteString("This is meeting content about the project roadmap.")
	mustNoErr(t, err)
	mustNoErr(t, f.Close())

	// POST /api/meetings/import
	resp := postJSON(t, base+"/api/meetings/import", map[string]any{
		"path":    f.Name(),
		"summary": "test summary",
	})
	var meeting map[string]any
	decodeJSON(t, resp, &meeting)

	if _, ok := meeting["id"]; !ok {
		t.Fatal("expected 'id' in import meeting response")
	}
	if meeting["title"] == nil || meeting["title"] == "" {
		t.Error("expected non-empty title in meeting response")
	}
	meetingID := int(meeting["id"].(float64))

	// GET /api/meetings/{id}
	getResp := get(t, fmt.Sprintf("%s/api/meetings/%d", base, meetingID))
	var got map[string]any
	decodeJSON(t, getResp, &got)

	rawContent, _ := got["raw_content"].(string)
	if rawContent == "" {
		t.Error("expected non-empty raw_content in meeting GET response")
	}
}

// TestMeetingRichContent verifies PUT /api/meetings/{id}/rich-content.
func TestMeetingRichContent(t *testing.T) {
	base, st := newTestServer(t)

	m, err := st.CreateMeeting(nil, "f.vtt", "2026-01-01", "Title", "raw", "sum")
	mustNoErr(t, err)

	putURL := fmt.Sprintf("%s/api/meetings/%d/rich-content", base, m.ID)

	// valid request — markdown
	req, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(`{"content":"# Hello","content_type":"markdown"}`))
	mustNoErr(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	mustNoErr(t, err)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("PUT rich-content: expected 200, got %d — %s", resp.StatusCode, string(body))
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["rich_content"] != "# Hello" {
		t.Errorf("expected rich_content='# Hello', got %v", updated["rich_content"])
	}
	if updated["content_type"] != "markdown" {
		t.Errorf("expected content_type='markdown', got %v", updated["content_type"])
	}

	// GET to confirm persistence
	getResp := get(t, fmt.Sprintf("%s/api/meetings/%d", base, m.ID))
	var got map[string]any
	decodeJSON(t, getResp, &got)
	if got["rich_content"] != "# Hello" {
		t.Errorf("GET: expected rich_content='# Hello', got %v", got["rich_content"])
	}
	if got["content_type"] != "markdown" {
		t.Errorf("GET: expected content_type='markdown', got %v", got["content_type"])
	}

	// invalid content_type → 400
	req2, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(`{"content":"x","content_type":"pdf"}`))
	mustNoErr(t, err)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	mustNoErr(t, err)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid content_type, got %d", resp2.StatusCode)
	}

	// unknown id → 404
	req3, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/meetings/99999/rich-content", base), strings.NewReader(`{"content":"x","content_type":"markdown"}`))
	mustNoErr(t, err)
	req3.Header.Set("Content-Type", "application/json")
	resp3, err := http.DefaultClient.Do(req3)
	mustNoErr(t, err)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown meeting, got %d", resp3.StatusCode)
	}
}

// TestSearch_Endpoint verifies GET /api/search?q=... returns matching results.
func TestSearch_Endpoint(t *testing.T) {
	base, st := newTestServer(t)

	// seed data directly through the store
	_, err := st.CreateTask(nil, nil, "zephyrquux endpoint task", "unique", "todo", "medium", "", "")
	mustNoErr(t, err)
	_, err = st.CreateMeeting(nil, "meeting.txt", "2026-01-01", "zephyrquux endpoint meeting", "content", "")
	mustNoErr(t, err)

	resp := get(t, base+"/api/search?q=zephyrquux")
	var results []map[string]any
	decodeJSON(t, resp, &results)

	if results == nil {
		t.Error("expected non-null search results array")
	}
	if len(results) < 1 {
		t.Errorf("expected >= 1 search result for 'zephyrquux', got %d", len(results))
	}
}
