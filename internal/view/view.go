package view

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// View manages template caching and rendering.
type View struct {
	fsys         fs.FS
	isProduction bool
	cache        map[string]*template.Template
	mu           sync.RWMutex
}

// New creates a new View renderer.
func New(fsys fs.FS, isProduction bool) *View {
	return &View{
		fsys:         fsys,
		isProduction: isProduction,
		cache:        make(map[string]*template.Template),
	}
}

// FuncMap provides utility functions inside HTML templates.
func FuncMap() template.FuncMap {
	p := message.NewPrinter(language.Indonesian)
	return template.FuncMap{
		"formatMoney": func(amount int64) string {
			return "Rp " + p.Sprintf("%d", amount)
		},
		"formatDate": func(t time.Time) string {
			return t.Format("02 Jan 2006, 15:04")
		},
		"formatDateShort": func(t time.Time) string {
			return t.Format("02 Jan 2006")
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"formatBytes": func(b int64) string {
			const unit = 1024
			if b < unit {
				return fmt.Sprintf("%d B", b)
			}
			div, exp := int64(unit), 0
			for n := b / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
		},
		"nl2br": func(text string) template.HTML {
			escaped := template.HTMLEscapeString(text)
			replaced := strings.ReplaceAll(escaped, "\n", "<br>")
			return template.HTML(replaced)
		},
		"renderMarkdown": RenderMarkdown,
	}
}

// Render executes a layout and page template and writes the HTML response.
func (v *View) Render(w http.ResponseWriter, layout string, page string, data any) error {
	tmpl, err := v.getTemplate(layout, page)
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

func (v *View) getTemplate(layout, page string) (*template.Template, error) {
	key := layout + ":" + page

	if v.isProduction {
		v.mu.RLock()
		tmpl, exists := v.cache[key]
		v.mu.RUnlock()
		if exists {
			return tmpl, nil
		}
	}

	layoutPath := fmt.Sprintf("layouts/%s.html", layout)
	pagePath := fmt.Sprintf("pages/%s.html", page)

	tmpl, err := template.New(layout+".html").Funcs(FuncMap()).ParseFS(v.fsys, layoutPath, pagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates [%s, %s]: %w", layoutPath, pagePath, err)
	}

	if v.isProduction {
		v.mu.Lock()
		v.cache[key] = tmpl
		v.mu.Unlock()
	}

	return tmpl, nil
}
