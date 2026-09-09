package pulse

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pentoshi007/renderpulse/internal/config"
	"github.com/pentoshi007/renderpulse/internal/services"
)

func testConfig() *config.Config {
	return &config.Config{
		IntervalMin:   time.Hour,
		IntervalMax:   time.Hour,
		Timeout:       5 * time.Second,
		StartupJitter: 50 * time.Millisecond,
		ThinkMin:      time.Millisecond,
		ThinkMax:      2 * time.Millisecond,
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRandRangeBounds(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	if got := randRange(rng, 0, 0); got != 0 {
		t.Errorf("randRange(0,0) = %v", got)
	}
	lo, hi := 3*time.Minute, 7*time.Minute
	for i := 0; i < 500; i++ {
		got := randRange(rng, lo, hi)
		if got < lo || got > hi {
				t.Fatalf("randRange out of bounds: %v", got)
		}
	}
}

func TestDecorateDeterministicAndWellFormed(t *testing.T) {
	one := decorate("/api/menu", rand.New(rand.NewPCG(21, 22)))
	two := decorate("/api/menu", rand.New(rand.NewPCG(21, 22)))
	if one != two {
		t.Errorf("decorate not deterministic per seed: %q vs %q", one, two)
	}
	for i := 0; i < 200; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 99))
		for _, path := range []string{"/", "/api/health", "/socket.io/?EIO=4&transport=polling"} {
			out := decorate(path, rng)
			if strings.Count(out, "?") > 1 {
				t.Errorf("decorate produced multiple '?': %q", out)
			}
			if !strings.HasPrefix(out, path) {
				t.Errorf("decorate changed the path: %q -> %q", path, out)
			}
		}
	}
	// engine.io habit: handshake always gains a cache-busting t= parameter
	rng := rand.New(rand.NewPCG(1, 1))
	out := decorate("/socket.io/?EIO=4&transport=polling", rng)
	if !strings.Contains(out, "&t=") {
		t.Errorf("socket.io path not decorated with t=: %q", out)
	}
}

func TestOnceVisitsEveryAssignedService(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svcs := []services.Service{
		{Name: "a", BaseURL: srv.URL, Routes: []services.Route{{Path: "/a1", Weight: 1}, {Path: "/a2", Weight: 1}}},
		{Name: "b", BaseURL: srv.URL, Routes: []services.Route{{Path: "/b1", Weight: 1}}},
	}
	cfg := testConfig()
	cfg.Once = true
	p := &Pulsar{Cfg: cfg, Log: quietLogger(), Client: srv.Client()}
	if err := p.Run(context.Background(), svcs, 101, 202); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hits["/a1"]+hits["/a2"] < 1 || hits["/b1"] < 1 {
		t.Errorf("expected at least one request per service, got %v", hits)
	}
}

func TestDryRunNeverSends(t *testing.T) {
	var fired bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fired = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svcs := []services.Service{
		{Name: "a", BaseURL: srv.URL, Routes: []services.Route{{Path: "/x", Weight: 1}}},
	}
	cfg := testConfig()
	cfg.Once = true
	cfg.DryRun = true
	p := &Pulsar{Cfg: cfg, Log: quietLogger(), Client: srv.Client()}
	if err := p.Run(context.Background(), svcs, 1, 2); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fired {
		t.Error("dry-run must not send any request")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svcs := []services.Service{
		{Name: "a", BaseURL: srv.URL, Routes: []services.Route{{Path: "/x", Weight: 1}}},
	}
	cfg := testConfig() // interval 1h: after the first visit it just sleeps
	p := &Pulsar{Cfg: cfg, Log: quietLogger(), Client: srv.Client()}

	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- p.Run(ctx, svcs, 3, 4) }()
	time.Sleep(300 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("Run returned before cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}
