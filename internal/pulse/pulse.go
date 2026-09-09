package pulse

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pentoshi007/renderpulse/internal/config"
	"github.com/pentoshi007/renderpulse/internal/profile"
	"github.com/pentoshi007/renderpulse/internal/services"
)

// maxQuickRetries bounds consecutive fast retries after failed visits so a
// dead host is never hammered.
const maxQuickRetries = 3

type Pulsar struct {
	Cfg    *config.Config
	Log    *slog.Logger
	Client *http.Client
}

// Run drives one independent loop per service until ctx is cancelled.
// Each loop gets its own PCG stream derived from the machine seed, so the
// schedules of different services (and different machines) never align.
func (p *Pulsar) Run(ctx context.Context, svcs []services.Service, seedA, seedB uint64) error {
	var wg sync.WaitGroup
	for i, svc := range svcs {
		wg.Add(1)
		go func(i int, svc services.Service) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(seedA+uint64(i)*7919, seedB))
			p.loop(ctx, svc, rng)
		}(i, svc)
	}
	wg.Wait()
	return ctx.Err()
}

func (p *Pulsar) loop(ctx context.Context, svc services.Service, rng *rand.Rand) {
	stagger := randRange(rng, 0, p.Cfg.StartupJitter)
	if p.Cfg.Once {
		stagger = randRange(rng, 0, 2*time.Second)
	}
	if !sleep(ctx, stagger) {
		return
	}
	if p.Cfg.Once {
		p.visit(ctx, svc, rng)
		return
	}
	failStreak := 0
	for {
		ok := p.visit(ctx, svc, rng)
		next := randRange(rng, p.Cfg.IntervalMin, p.Cfg.IntervalMax)
		if ok {
			failStreak = 0
		} else if failStreak < maxQuickRetries {
			failStreak++
			next = minDuration(next, randRange(rng, 30*time.Second, 60*time.Second))
		}
		p.Log.Info("next visit scheduled", "service", svc.Name, "in", next.String())
		if !sleep(ctx, next) {
			return
		}
	}
}

// visit simulates a short browsing session against one service: a consistent
// browser profile making one to three related requests with human pauses.
func (p *Pulsar) visit(ctx context.Context, svc services.Service, rng *rand.Rand) bool {
	prof := profile.Pick(rng)
	n := visitLength(rng)
	alive := false
	for i := 0; i < n; i++ {
		route := svc.PickRoute(rng)
		path := decorate(route.Path, rng)
		if p.request(ctx, svc, prof, pickMode(route.Path, rng), path, rng) {
			alive = true
		}
		if i < n-1 && !sleep(ctx, randRange(rng, p.Cfg.ThinkMin, p.Cfg.ThinkMax)) {
			return alive
		}
	}
	return alive
}

// request sends one GET and reports whether the service answered at all; any
// HTTP status, including 404, still counts as inbound traffic for Render.
func (p *Pulsar) request(ctx context.Context, svc services.Service, prof profile.Profile, mode profile.Mode, path string, rng *rand.Rand) bool {
	url := svc.URL(path)
	if p.Cfg.DryRun {
		p.Log.Info("dry-run", "service", svc.Name, "method", "GET", "url", url, "profile", prof.Name, "mode", modeName(mode))
		return true
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		p.Log.Warn("bad request", "service", svc.Name, "url", url, "err", err.Error())
		return false
	}
	for k, vs := range prof.Headers(mode, rng, svc.BaseURL) {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	start := time.Now()
	resp, err := p.Client.Do(req)
	if err != nil {
		p.Log.Warn("request failed", "service", svc.Name, "url", url, "err", err.Error(), "elapsed", time.Since(start).Round(time.Millisecond).String())
		return false
	}
	n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	p.Log.Info("request sent",
		"service", svc.Name,
		"method", "GET",
		"path", path,
		"status", resp.StatusCode,
		"bytes", n,
		"elapsed", time.Since(start).Round(time.Millisecond).String(),
		"profile", prof.Name,
	)
	return true
}

func visitLength(rng *rand.Rand) int {
	switch r := rng.Float64(); {
	case r < 0.6:
		return 1
	case r < 0.85:
		return 2
	default:
		return 3
	}
}

func pickMode(path string, rng *rand.Rand) profile.Mode {
	api := strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/socket.io/")
	if api {
		if rng.Float64() < 0.9 {
			return profile.FetchAPI
		}
		return profile.Navigate
	}
	if rng.Float64() < 0.85 {
		return profile.Navigate
	}
	return profile.FetchAPI
}

// decorate sometimes appends a plausible query string. engine.io clients
// habitually carry a cache-busting t= parameter, which is reproduced here.
func decorate(path string, rng *rand.Rand) string {
	if strings.HasPrefix(path, "/socket.io/") && !strings.Contains(path, "?t=") && !strings.Contains(path, "&t=") {
		return path + "&t=" + fakeEpochMs(rng)
	}
	if rng.Float64() < 0.55 {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + queryParam(rng)
}

var (
	paramKeys = []string{"v", "_", "t", "page", "limit", "ref", "q"}
	refValues = []string{"menu", "cart", "home", "app"}
	qValues   = []string{"pizza", "burger", "salad", "coffee"}
)

func queryParam(rng *rand.Rand) string {
	key := paramKeys[rng.IntN(len(paramKeys))]
	switch key {
	case "v":
		return "v=" + strconv.Itoa(1+rng.IntN(9))
	case "_", "t":
		return key + "=" + fakeEpochMs(rng)
	case "page":
		return "page=" + strconv.Itoa(1+rng.IntN(4))
	case "limit":
		return "limit=" + []string{"10", "20", "50"}[rng.IntN(3)]
	case "ref":
		return "ref=" + refValues[rng.IntN(len(refValues))]
	default:
		return "q=" + qValues[rng.IntN(len(qValues))]
	}
}

// fakeEpochMs yields a deterministic millisecond-looking number instead of
// reading the wall clock, keeping generated URLs reproducible per seed.
func fakeEpochMs(rng *rand.Rand) string {
	return strconv.FormatInt(1_500_000_000_000+rng.Int64N(600_000_000_000), 10)
}

func randRange(rng *rand.Rand, min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	return min + time.Duration(rng.Int64N(int64(max-min)+1))
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func modeName(m profile.Mode) string {
	if m == profile.FetchAPI {
		return "fetch"
	}
	return "navigate"
}
