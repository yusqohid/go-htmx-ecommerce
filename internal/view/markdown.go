package view

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
)

var (
	boldRegex       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRegex     = regexp.MustCompile(`\*([^*]+)\*`)
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	linkRegex       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// RenderMarkdown safely converts a markdown string into sanitised HTML.
func RenderMarkdown(input string) template.HTML {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	var out strings.Builder

	inList := false
	inOrderedList := false

	closeLists := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
		}
		if inOrderedList {
			out.WriteString("</ol>\n")
			inOrderedList = false
		}
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			closeLists()
			continue
		}

		// Headings (# H1, ## H2, ### H3, #### H4)
		if strings.HasPrefix(trimmed, "#### ") {
			closeLists()
			out.WriteString(fmt.Sprintf("<h6 class=\"fw-bold mt-4 mb-2 text-dark\">%s</h6>\n", parseInline(trimmed[5:])))
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			closeLists()
			out.WriteString(fmt.Sprintf("<h5 class=\"fw-bold mt-4 mb-2 text-dark\">%s</h5>\n", parseInline(trimmed[4:])))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			closeLists()
			out.WriteString(fmt.Sprintf("<h4 class=\"fw-bold mt-4 mb-2 text-dark\">%s</h4>\n", parseInline(trimmed[3:])))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			closeLists()
			out.WriteString(fmt.Sprintf("<h3 class=\"fw-bold mt-4 mb-3 text-dark\">%s</h3>\n", parseInline(trimmed[2:])))
			continue
		}

		// Blockquote (> Quote)
		if strings.HasPrefix(trimmed, "> ") {
			closeLists()
			out.WriteString(fmt.Sprintf("<blockquote class=\"border-start border-3 border-success ps-3 my-3 text-muted fst-italic\">%s</blockquote>\n", parseInline(trimmed[2:])))
			continue
		}

		// Unordered list (- item, * item, + item)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			if inOrderedList {
				closeLists()
			}
			if !inList {
				out.WriteString("<ul class=\"mb-3 ps-3\">\n")
				inList = true
			}
			out.WriteString(fmt.Sprintf("  <li class=\"mb-1\">%s</li>\n", parseInline(trimmed[2:])))
			continue
		}

		// Ordered list (1. item, 2. item, etc.)
		if len(trimmed) > 3 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:4], ". ") {
			dotIdx := strings.Index(trimmed, ". ")
			if dotIdx != -1 {
				if inList {
					closeLists()
				}
				if !inOrderedList {
					out.WriteString("<ol class=\"mb-3 ps-3\">\n")
					inOrderedList = true
				}
				out.WriteString(fmt.Sprintf("  <li class=\"mb-1\">%s</li>\n", parseInline(trimmed[dotIdx+2:])))
				continue
			}
		}

		// Standard Paragraph
		closeLists()
		out.WriteString(fmt.Sprintf("<p class=\"mb-3\">%s</p>\n", parseInline(trimmed)))
	}

	closeLists()
	return template.HTML(out.String())
}

func parseInline(text string) string {
	// First: strictly escape HTML to neutralize any <script>, <iframe>, or rogue tags
	escaped := html.EscapeString(text)

	// Bold: **text**
	escaped = boldRegex.ReplaceAllString(escaped, "<strong>$1</strong>")

	// Italic: *text*
	escaped = italicRegex.ReplaceAllString(escaped, "<em>$1</em>")

	// Inline Code: `code`
	escaped = inlineCodeRegex.ReplaceAllString(escaped, `<code class="bg-light border px-1.5 py-0.5 rounded text-dark font-monospace small">$1</code>`)

	// Links: [text](url) - strictly sanitize scheme to prevent javascript: or data: URIs
	escaped = linkRegex.ReplaceAllStringFunc(escaped, func(m string) string {
		sub := linkRegex.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		label := sub[1]
		rawURL := strings.TrimSpace(sub[2])
		// Allowed schemes: http, https, relative paths (/ or #)
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "/") || strings.HasPrefix(rawURL, "#") {
			return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer" class="text-success text-decoration-none fw-medium">%s</a>`, rawURL, label)
		}
		return label // Fallback to label if invalid/malicious scheme
	})

	return escaped
}
