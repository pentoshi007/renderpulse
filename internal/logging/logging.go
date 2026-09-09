package logging

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/pentoshi007/renderpulse/internal/config"
)

func New(cfg config.Config) (*slog.Logger, io.Closer, error) {
	if cfg.LogFile == "" {
		return build(os.Stdout, cfg.LogJSON), nopCloser{}, nil
	}
	w, err := newRotatingWriter(cfg.LogFile, int64(cfg.LogMaxMB)<<20)
	if err != nil {
		return nil, nil, err
	}
	return build(w, cfg.LogJSON), w, nil
}

func build(w io.Writer, asJSON bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if asJSON {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

type rotatingWriter struct {
	mu   sync.Mutex
	f    *os.File
	path string
	max  int64
	n    int64
}

func newRotatingWriter(path string, max int64) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &rotatingWriter{f: f, path: path, max: max, n: st.Size()}, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.n+int64(len(p)) > w.max {
		w.rotate()
	}
	n, err := w.f.Write(p)
	w.n += int64(n)
	return n, err
}

// rotate keeps a single previous generation (path.1) — enough for a daemon
// whose logs are only used to confirm keep-alive activity.
func (w *rotatingWriter) rotate() {
	w.f.Close()
	_ = os.Rename(w.path, w.path+".1")
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	w.f = f
	w.n = 0
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
