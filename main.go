package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/pentoshi007/renderpulse/internal/config"
	"github.com/pentoshi007/renderpulse/internal/logging"
	"github.com/pentoshi007/renderpulse/internal/pulse"
	"github.com/pentoshi007/renderpulse/internal/seed"
	"github.com/pentoshi007/renderpulse/internal/services"
)

var version = "dev"

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "renderpulse:", err)
		os.Exit(2)
	}
	switch {
	case cfg.ShowVersion:
		fmt.Printf("renderpulse %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	case cfg.ListServices:
		listServices(*cfg)
		return
	}
	if err := services.Validate(services.All); err != nil {
		fmt.Fprintln(os.Stderr, "renderpulse:", err)
		os.Exit(1)
	}

	log, closer, err := logging.New(*cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "renderpulse:", err)
		os.Exit(1)
	}
	defer closer.Close()

	svcs := services.ForShard(services.All, cfg.Shard.Index, cfg.Shard.Count)
	if len(svcs) == 0 {
		log.Error("shard assignment is empty", "shard", cfg.Shard.String())
		os.Exit(1)
	}

	seedA, seedB := seed.Derive(cfg.Shard.Index, cfg.Seed)
	log.Info("renderpulse starting",
		"version", version,
		"services", serviceNames(svcs),
		"shard", cfg.Shard.String(),
		"interval", cfg.IntervalMin.String()+".."+cfg.IntervalMax.String(),
		"timeout", cfg.Timeout.String(),
		"dry_run", cfg.DryRun,
		"once", cfg.Once,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	p := &pulse.Pulsar{Cfg: cfg, Log: log, Client: &http.Client{Timeout: cfg.Timeout}}
	if err := p.Run(ctx, svcs, seedA, seedB); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("renderpulse stopped with error", "err", err.Error())
		os.Exit(1)
	}
	log.Info("renderpulse stopped")
}

func listServices(cfg config.Config) {
	for i, s := range services.All {
		mark := " "
		if i%cfg.Shard.Count == cfg.Shard.Index-1 {
			mark = "*"
		}
		fmt.Printf("%s %-12s %s\n", mark, s.Name, s.BaseURL)
	}
	fmt.Printf("(* = handled by shard %s)\n", cfg.Shard.String())
}

func serviceNames(svcs []services.Service) []string {
	names := make([]string, len(svcs))
	for i, s := range svcs {
		names[i] = s.Name
	}
	return names
}
