package profile

import (
	"math/rand/v2"
	"net/http"
)

type Mode int

const (
	Navigate Mode = iota // user opens or refreshes a page
	FetchAPI             // the page's JavaScript calls an API
)

type Profile struct {
	Name       string
	UserAgent  string
	SecCHUA    string // empty for non-Chromium browsers
	Mobile     bool
	Platform   string
	AcceptHTML string
	Languages  []string
}

var pool = []Profile{
	{
		Name:       "chrome-windows",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		SecCHUA:    `"Chromium";v="131", "Not_A Brand";v="24", "Google Chrome";v="131"`,
		Platform:   `"Windows"`,
		AcceptHTML: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		Languages:  []string{"en-US,en;q=0.9", "en-GB,en;q=0.9", "en-US,en;q=0.9,es;q=0.8"},
	},
	{
		Name:       "chrome-android",
		UserAgent:  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
		SecCHUA:    `"Chromium";v="131", "Not_A Brand";v="24", "Google Chrome";v="131"`,
		Mobile:     true,
		Platform:   `"Android"`,
		AcceptHTML: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		Languages:  []string{"en-US,en;q=0.9", "en-IN,en;q=0.9,hi;q=0.8"},
	},
	{
		Name:       "firefox-macos",
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0",
		AcceptHTML: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		Languages:  []string{"en-US,en;q=0.5", "en-GB,en;q=0.9"},
	},
	{
		Name:       "safari-macos",
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
		AcceptHTML: "text/html,application/xhtml+xml,application/xml;q=0.9",
		Languages:  []string{"en-US,en;q=0.9", "en-GB,en;q=0.9"},
	},
}

// Pick returns a random browser profile. One profile is reused for every
// request of a visit so a session looks like a single browser.
func Pick(rng *rand.Rand) Profile { return pool[rng.IntN(len(pool))] }

func (p Profile) AcceptLanguage(rng *rand.Rand) string {
	return p.Languages[rng.IntN(len(p.Languages))]
}

// Headers builds an internally consistent set: Chromium client hints only on
// Chromium, sec-fetch values matching the request mode, and a Referer only
// where a real navigation would carry one.
func (p Profile) Headers(mode Mode, rng *rand.Rand, baseURL string) http.Header {
	h := http.Header{}
	h.Set("User-Agent", p.UserAgent)
	h.Set("Accept-Language", p.AcceptLanguage(rng))
	if p.SecCHUA != "" {
		h.Set("sec-ch-ua", p.SecCHUA)
		h.Set("sec-ch-ua-mobile", boolStr(p.Mobile))
		h.Set("sec-ch-ua-platform", p.Platform)
	}
	switch mode {
	case FetchAPI:
		h.Set("Accept", "application/json, text/plain, */*")
		h.Set("sec-fetch-mode", "cors")
		h.Set("sec-fetch-dest", "empty")
		if rng.Float64() < 0.6 {
			h.Set("sec-fetch-site", "cross-site")
		} else {
			h.Set("sec-fetch-site", "same-site")
		}
	default:
		h.Set("Accept", p.AcceptHTML)
		h.Set("sec-fetch-mode", "navigate")
		h.Set("sec-fetch-dest", "document")
		h.Set("Upgrade-Insecure-Requests", "1")
		switch r := rng.Float64(); {
		case r < 0.6: // typed in the address bar or opened from bookmarks
			h.Set("sec-fetch-site", "none")
		case r < 0.9: // followed a search-engine result
			h.Set("sec-fetch-site", "cross-site")
			h.Set("Referer", "https://www.google.com/")
		default: // link from elsewhere in the same deployment
			h.Set("sec-fetch-site", "same-site")
			h.Set("Referer", baseURL+"/")
		}
		if rng.Float64() < 0.3 {
			h.Set("Cache-Control", "no-cache")
		}
	}
	return h
}

func boolStr(b bool) string {
	if b {
		return "?1"
	}
	return "?0"
}
