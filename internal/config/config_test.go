package config

import (
	"testing"
	"time"
)

func TestParseShard(t *testing.T) {
	cases := []struct {
		in      string
		wantI   int
		wantN   int
		wantErr bool
	}{
		{"1/1", 1, 1, false},
		{"1/2", 1, 2, false},
		{"2/2", 2, 2, false},
		{"", 1, 1, false},
		{"0/2", 0, 0, true},
		{"3/2", 0, 0, true},
		{"abc", 0, 0, true},
		{"1/", 0, 0, true},
		{"/2", 0, 0, true},
		{"1/2/3", 0, 0, true},
		{"-1/2", 0, 0, true},
	}
	for _, c := range cases {
		got, err := ParseShard(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseShard(%q) = %v, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseShard(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got.Index != c.wantI || got.Count != c.wantN {
			t.Errorf("ParseShard(%q) = %d/%d, want %d/%d", c.in, got.Index, got.Count, c.wantI, c.wantN)
		}
	}
}

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if cfg.IntervalMin != 3*time.Minute || cfg.IntervalMax != 7*time.Minute {
		t.Errorf("default interval = %s..%s, want 3m..7m", cfg.IntervalMin, cfg.IntervalMax)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("default timeout = %s, want 90s", cfg.Timeout)
	}
	if cfg.Shard.Index != 1 || cfg.Shard.Count != 1 {
		t.Errorf("default shard = %s, want 1/1", cfg.Shard)
	}
}

func TestParseRejectsUnsafeIntervals(t *testing.T) {
	if _, err := Parse([]string{"--interval-min", "14m", "--interval-max", "20m"}); err == nil {
		t.Error("interval near the 15-minute spin-down should be rejected")
	}
	if _, err := Parse([]string{"--interval-min", "5m", "--interval-max", "3m"}); err == nil {
		t.Error("inverted interval bounds should be rejected")
	}
	if _, err := Parse([]string{"--shard", "3/2"}); err == nil {
		t.Error("out-of-range shard should be rejected")
	}
}
