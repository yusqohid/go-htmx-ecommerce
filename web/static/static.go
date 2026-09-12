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

// Handler returns an http.Handler that serves embedded static assets from /static/.
func Handler() http.Handler {
	sub, err := fs.Sub(FS, ".")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}
