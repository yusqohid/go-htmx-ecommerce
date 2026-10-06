package view

import (
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

var (
	// youtubeIDRegex matches an 11-character alphanumeric, underscore, or hyphen YouTube video ID.
	youtubeIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	// vimeoIDRegex matches numeric Vimeo video IDs.
	vimeoIDRegex = regexp.MustCompile(`^[0-9]+$`)
)

// EmbedVideoURL extracts the video ID from YouTube or Vimeo URLs and returns a privacy-enhanced embed URL.
// Returns an empty string if the URL is not a recognized or valid video URL.
func EmbedVideoURL(rawURL string) template.URL {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")

	switch {
	// YouTube: youtube.com, m.youtube.com
	case host == "youtube.com" || host == "m.youtube.com":
		// Standard /watch?v=VIDEO_ID
		if u.Path == "/watch" {
			videoID := u.Query().Get("v")
			if youtubeIDRegex.MatchString(videoID) {
				return template.URL(fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s", videoID))
			}
		}
		// Direct embed or shorts: /embed/VIDEO_ID or /shorts/VIDEO_ID
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 && (parts[0] == "embed" || parts[0] == "shorts") {
			if youtubeIDRegex.MatchString(parts[1]) {
				return template.URL(fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s", parts[1]))
			}
		}

	case host == "youtu.be":
		// Short URL: youtu.be/VIDEO_ID
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 1 && youtubeIDRegex.MatchString(parts[0]) {
			return template.URL(fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s", parts[0]))
		}

	case host == "vimeo.com" || host == "player.vimeo.com":
		// Vimeo: vimeo.com/VIDEO_ID or player.vimeo.com/video/VIDEO_ID
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if vimeoIDRegex.MatchString(parts[i]) {
				return template.URL(fmt.Sprintf("https://player.vimeo.com/video/%s", parts[i]))
			}
		}
	}

	return ""
}
