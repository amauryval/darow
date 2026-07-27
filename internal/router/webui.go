package router

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// mountUI serves the embedded SPA under /ui, falling back to index.html for
// any path that isn't a real static asset so vue-router's history-mode
// client routes (e.g. /ui/tables/abc) resolve on a hard refresh.
func mountUI(assets fs.FS) http.Handler {
	fileServer := http.FileServerFS(assets)

	return http.StripPrefix("/ui", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean(req.URL.Path), "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		if _, err := fs.Stat(assets, cleanPath); err != nil {
			// http.FileServer redirects any request ending in "/index.html"
			// to "/", so fall back to the root rather than to index.html
			// directly to avoid a spurious redirect on unknown SPA routes.
			req.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, req)
	}))
}
