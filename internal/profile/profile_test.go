package profile

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestHeadersAreInternallyConsistent(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	for i := 0; i < 100; i++ {
		p := Pick(rng)
		for _, mode := range []Mode{Navigate, FetchAPI} {
			h := p.Headers(mode, rng, "https://svc.example")
			if h.Get("User-Agent") == "" {
				t.Fatal("User-Agent missing")
			}
			if h.Get("Accept-Language") == "" {
				t.Fatal("Accept-Language missing")
			}
			chromium := strings.Contains(p.UserAgent, "Chrome")
			if chromium && h.Get("sec-ch-ua") == "" {
				t.Fatalf("chromium profile %s missing sec-ch-ua", p.Name)
			}
			if !chromium && h.Get("sec-ch-ua") != "" {
				t.Fatalf("non-chromium profile %s must not send sec-ch-ua", p.Name)
			}
			wantMode := "navigate"
			if mode == FetchAPI {
				wantMode = "cors"
			}
			if h.Get("sec-fetch-mode") != wantMode {
				t.Fatalf("sec-fetch-mode = %q, want %q", h.Get("sec-fetch-mode"), wantMode)
			}
			if h.Get("sec-fetch-site") == "none" && h.Get("Referer") != "" {
				t.Fatal("sec-fetch-site none must not carry a Referer")
			}
		}
	}
}

func TestPickCoversAllProfiles(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	got := map[string]bool{}
	for i := 0; i < 200; i++ {
		got[Pick(rng).Name] = true
	}
	for _, p := range pool {
		if !got[p.Name] {
				t.Errorf("profile %s never picked", p.Name)
		}
	}
}
