package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

func Handler() http.Handler {
	sub, _ := fs.Sub(staticFiles, "static")
	fs := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// always serve index.html for SPA routes
		if r.URL.Path != "/" {
			// check if file exists
			f, err := staticFiles.Open("static" + r.URL.Path)
			if err != nil {
				r.URL.Path = "/"
			} else {
				f.Close()
			}
		}
		fs.ServeHTTP(w, r)
	})
}
