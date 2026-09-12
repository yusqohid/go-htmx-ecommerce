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
