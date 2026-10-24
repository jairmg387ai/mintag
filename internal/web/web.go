package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// The SPA is built by `make frontend-build` into static/app, which git
// ignores; release builds run that step first (see .goreleaser.yaml).
// static/.gitkeep keeps the embed pattern valid when the UI was not built.
//
//go:embed all:static
var staticFiles embed.FS

const uiNotBuilt = `<!doctype html>
<html><head><meta charset="utf-8"><title>Mintag</title></head>
<body style="font-family:sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem">
<h1>Mintag web UI is not built</h1>
<p>This binary was compiled without the frontend. Run <code>make frontend-install</code> once,
then <code>make frontend-build</code>, and rebuild the binary (or use <code>make build</code>).</p>
<p>The <code>/api</code> endpoints and <code>mintag mcp</code> work without it.</p>
</body></html>
`

func Handler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static/app")
	if err != nil {
		sub = emptyFS{}
	}
	return handlerFor(sub)
}

func handlerFor(ui fs.FS) http.Handler {
	files := http.FileServer(http.FS(ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(ui, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(uiNotBuilt))
			return
		}
		// always serve index.html for SPA routes
		if r.URL.Path != "/" {
			if _, err := fs.Stat(ui, r.URL.Path[1:]); err != nil {
				r.URL.Path = "/"
			}
		}
		files.ServeHTTP(w, r)
	})
}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }
