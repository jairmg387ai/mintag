package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

func builtUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":        {Data: []byte("<html>mintag spa</html>")},
		"assets/index-1.js": {Data: []byte("console.log('app')")},
	}
}

func TestHandler_ServesBuiltAsset(t *testing.T) {
	code, body := get(t, handlerFor(builtUI()), "/assets/index-1.js")
	if code != http.StatusOK || !strings.Contains(body, "console.log") {
		t.Fatalf("want asset, got %d %q", code, body)
	}
}

func TestHandler_FallsBackToIndexForSPARoutes(t *testing.T) {
	code, body := get(t, handlerFor(builtUI()), "/graph/tables")
	if code != http.StatusOK || !strings.Contains(body, "mintag spa") {
		t.Fatalf("want index.html for SPA route, got %d %q", code, body)
	}
}

func TestHandler_ExplainsMissingUIBuild(t *testing.T) {
	for _, path := range []string{"/", "/graph"} {
		code, body := get(t, handlerFor(fstest.MapFS{}), path)
		if code != http.StatusServiceUnavailable || !strings.Contains(body, "make frontend-build") {
			t.Fatalf("%s: want 503 with build hint, got %d %q", path, code, body)
		}
	}
}
