package static

import (
	"embed"
	"io/fs"
	"net/http"
)

// FS embeds all static assets (CSS, JS, fonts, images).
//
//go:embed all:*
var FS embed.FS

// Handler returns an http.Handler that serves embedded static assets from /static/ with caching headers.
func Handler() http.Handler {
	sub, err := fs.Sub(FS, ".")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours caching
		fileServer.ServeHTTP(w, r)
	}))
}
