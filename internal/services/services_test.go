package services

import (
	"math/rand/v2"
	"testing"
)

func TestValidateBuiltInFleet(t *testing.T) {
	if len(All) != 6 {
		t.Fatalf("built-in fleet has %d services, want 6", len(All))
	}
	if err := Validate(All); err != nil {
		t.Fatalf("Validate(All): %v", err)
	}
	for _, s := range All {
		if s.Name == "" || s.BaseURL == "" {
			t.Errorf("service %+v has empty name or URL", s)
		}
	}
}

func TestForShardPartitionsExactlyOnce(t *testing.T) {
	seen := map[string]int{}
	for idx := 1; idx <= 3; idx++ {
		for _, s := range ForShard(All, idx, 3) {
			seen[s.Name]++
		}
	}
	if len(seen) != len(All) {
		t.Fatalf("3 shards covered %d distinct services, want %d", len(seen), len(All))
	}
	for name, n := range seen {
		if n != 1 {
			t.Errorf("service %s covered %d times across shards, want exactly 1", name, n)
		}
	}
	if got := len(ForShard(All, 2, 2)); got != 3 {
		t.Errorf("shard 2/2 of 6 services holds %d services, want 3", got)
	}
}

func TestPickRouteReachesEveryRoute(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, s := range All {
		got := map[string]int{}
		for i := 0; i < 1000; i++ {
			got[s.PickRoute(rng).Path]++
		}
		for _, r := range s.Routes {
			if got[r.Path] == 0 {
				t.Errorf("service %s: route %s never picked in 1000 draws", s.Name, r.Path)
			}
		}
	}
}

func TestPickRouteHonoursWeights(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	svc := Service{
		Name: "t", BaseURL: "https://t.example",
		Routes: []Route{{"/heavy", 9}, {"/light", 1}},
	}
	heavy := 0
	for i := 0; i < 10000; i++ {
		if svc.PickRoute(rng).Path == "/heavy" {
			heavy++
		}
	}
	if heavy < 8500 || heavy > 9500 {
		t.Errorf("heavy route picked %d/10000 times, want ~9000", heavy)
	}
}
