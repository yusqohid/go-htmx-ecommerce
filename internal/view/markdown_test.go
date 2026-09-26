package view_test

import (
	"strings"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func TestRenderMarkdown_Formatting(t *testing.T) {
	input := `# Main Title
## Subtitle
### Section Header
#### Subsection

Here is a paragraph with **bold text** and *italicized text* and ` + "`code snippet`" + `.

> Important note about digital downloads.

- Feature Alpha
- Feature Beta
* Feature Gamma

1. First step
2. Second step

Visit our [Documentation](https://sellora.local/docs) or [Catalog](/products).
`
	html := string(view.RenderMarkdown(input))

	// Verify headings
	if !strings.Contains(html, "<h3 class=\"fw-bold mt-4 mb-3 text-dark\">Main Title</h3>") {
		t.Errorf("missing H1 rendering, got: %s", html)
	}
	if !strings.Contains(html, "<h4 class=\"fw-bold mt-4 mb-2 text-dark\">Subtitle</h4>") {
		t.Errorf("missing H2 rendering, got: %s", html)
	}
	if !strings.Contains(html, "<h5 class=\"fw-bold mt-4 mb-2 text-dark\">Section Header</h5>") {
		t.Errorf("missing H3 rendering, got: %s", html)
	}

	// Verify inline styles
	if !strings.Contains(html, "<strong>bold text</strong>") {
		t.Errorf("missing bold rendering")
	}
	if !strings.Contains(html, "<em>italicized text</em>") {
		t.Errorf("missing italic rendering")
	}
	if !strings.Contains(html, `<code class="bg-light border px-1.5 py-0.5 rounded text-dark font-monospace small">code snippet</code>`) {
		t.Errorf("missing inline code rendering")
	}

	// Verify blockquote
	if !strings.Contains(html, "<blockquote class=\"border-start border-3 border-success ps-3 my-3 text-muted fst-italic\">Important note about digital downloads.</blockquote>") {
		t.Errorf("missing blockquote rendering")
	}

	// Verify lists
	if !strings.Contains(html, "<ul class=\"mb-3 ps-3\">") || !strings.Contains(html, "<li class=\"mb-1\">Feature Alpha</li>") {
		t.Errorf("missing unordered list rendering")
	}
	if !strings.Contains(html, "<ol class=\"mb-3 ps-3\">") || !strings.Contains(html, "<li class=\"mb-1\">First step</li>") {
		t.Errorf("missing ordered list rendering")
	}

	// Verify safe links
	if !strings.Contains(html, `<a href="https://sellora.local/docs" target="_blank" rel="noopener noreferrer" class="text-success text-decoration-none fw-medium">Documentation</a>`) {
		t.Errorf("missing safe https link rendering")
	}
	if !strings.Contains(html, `<a href="/products" target="_blank" rel="noopener noreferrer" class="text-success text-decoration-none fw-medium">Catalog</a>`) {
		t.Errorf("missing safe relative link rendering")
	}
}

func TestRenderMarkdown_XSSPrevention(t *testing.T) {
	// 1. Raw script injection
	scriptInput := `<script>alert('xss')</script> <img src=x onerror=alert(1)>`
	html := string(view.RenderMarkdown(scriptInput))

	if strings.Contains(html, "<script>") || strings.Contains(html, "<img") {
		t.Errorf("raw tags should be escaped, got: %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag, got: %s", html)
	}

	// 2. Malicious URI schemes in markdown links
	maliciousLink := `[Click here](javascript:alert('xss')) and [Data](data:text/html,<script>alert(1)</script>)`
	htmlLink := string(view.RenderMarkdown(maliciousLink))

	if strings.Contains(htmlLink, `href="javascript:`) || strings.Contains(htmlLink, `href="data:`) {
		t.Errorf("malicious schemes must not be transformed into active hrefs, got: %s", htmlLink)
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	if view.RenderMarkdown("") != "" {
		t.Errorf("expected empty string for empty input")
	}
	if view.RenderMarkdown("   \n\r\n  ") != "" {
		t.Errorf("expected empty string for whitespace input")
	}
}
