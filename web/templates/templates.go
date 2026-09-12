package templates

import "embed"

// FS embeds all application HTML templates.
//
//go:embed layouts/*.html pages/*/*.html pages/*/*/*.html
var FS embed.FS
