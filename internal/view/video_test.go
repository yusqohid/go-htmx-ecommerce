package view

import (
	"bytes"
	"html/template"
	"testing"
)

func TestEmbedVideoURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected template.URL
	}{
		// YouTube tests
		{
			name:     "YouTube watch standard",
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},
		{
			name:     "YouTube watch with extra query parameters",
			input:    "https://youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},
		{
			name:     "YouTube mobile watch",
			input:    "https://m.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},
		{
			name:     "YouTube short URL youtu.be",
			input:    "https://youtu.be/dQw4w9WgXcQ",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},
		{
			name:     "YouTube short URL with query param",
			input:    "https://youtu.be/dQw4w9WgXcQ?si=12345",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},
		{
			name:     "YouTube embed URL",
			input:    "https://www.youtube.com/embed/dQw4w9WgXcQ",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},
		{
			name:     "YouTube shorts URL",
			input:    "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			expected: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		},

		// Vimeo tests
		{
			name:     "Vimeo standard URL",
			input:    "https://vimeo.com/76979871",
			expected: "https://player.vimeo.com/video/76979871",
		},
		{
			name:     "Vimeo player direct URL",
			input:    "https://player.vimeo.com/video/76979871",
			expected: "https://player.vimeo.com/video/76979871",
		},

		// Invalid / unsupported tests
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   \t\n  ",
			expected: "",
		},
		{
			name:     "Invalid URL format",
			input:    "://invalid-url",
			expected: "",
		},
		{
			name:     "Other domain",
			input:    "https://dailymotion.com/video/x7tgad0",
			expected: "",
		},
		{
			name:     "Malicious scheme",
			input:    "javascript:alert(1)",
			expected: "",
		},
		{
			name:     "Invalid YouTube video ID length",
			input:    "https://www.youtube.com/watch?v=too_short",
			expected: "",
		},
		{
			name:     "Invalid characters in YouTube ID",
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXc!",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EmbedVideoURL(tc.input)
			if got != tc.expected {
				t.Errorf("EmbedVideoURL(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestEmbedVideoURL_TemplateExecution(t *testing.T) {
	tmplText := `{{ $embed := embedVideoURL .URL }}{{ if $embed }}<iframe src="{{ $embed }}"></iframe>{{ else }}<no-video>{{ end }}`
	tmpl, err := template.New("test").Funcs(FuncMap()).Parse(tmplText)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	data := map[string]string{
		"URL": "https://youtu.be/dQw4w9WgXcQ",
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	expected := `<iframe src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"></iframe>`
	if buf.String() != expected {
		t.Errorf("got %q, expected %q", buf.String(), expected)
	}

	// Test empty/invalid
	buf.Reset()
	dataEmpty := map[string]string{
		"URL": "https://example.com/invalid",
	}
	if err := tmpl.Execute(&buf, dataEmpty); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}
	if buf.String() != "<no-video>" {
		t.Errorf("got %q, expected <no-video>", buf.String())
	}
}
