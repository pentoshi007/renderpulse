package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Shard selects services i/n so two machines can split the fleet without
// ever sending duplicate traffic for the same service.
type Shard struct {
	Index int
	Count int
}

func (s Shard) String() string { return fmt.Sprintf("%d/%d", s.Index, s.Count) }

type Config struct {
	IntervalMin   time.Duration
	IntervalMax   time.Duration
	Timeout       time.Duration
	StartupJitter time.Duration
	ThinkMin      time.Duration
	ThinkMax      time.Duration
	Shard         Shard
	Once          bool
	DryRun        bool
	LogFile       string
	LogJSON       bool
	LogMaxMB      int
	Seed          string
	ShowVersion   bool
	ListServices  bool
}

const (
	DefaultIntervalMin   = 3 * time.Minute
	DefaultIntervalMax   = 7 * time.Minute
	DefaultTimeout       = 90 * time.Second
	DefaultStartupJitter = 45 * time.Second
	DefaultThinkMin      = 800 * time.Millisecond
	DefaultThinkMax      = 6 * time.Second
	DefaultLogMaxMB      = 10
)

func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("renderpulse", flag.ContinueOnError)
	cfg := &Config{}
	fs.DurationVar(&cfg.IntervalMin, "interval-min", DefaultIntervalMin, "minimum delay between visits to the same service")
	fs.DurationVar(&cfg.IntervalMax, "interval-max", DefaultIntervalMax, "maximum delay between visits to the same service")
	fs.DurationVar(&cfg.Timeout, "timeout", DefaultTimeout, "per-request timeout (kept large to cover Render cold starts)")
	fs.DurationVar(&cfg.StartupJitter, "startup-jitter", DefaultStartupJitter, "random delay before the first visit, spreading instances apart")
	fs.DurationVar(&cfg.ThinkMin, "think-min", DefaultThinkMin, "minimum pause between requests inside one browsing session")
	fs.DurationVar(&cfg.ThinkMax, "think-max", DefaultThinkMax, "maximum pause between requests inside one browsing session")
	shard := fs.String("shard", "1/1", "only handle services i/n, e.g. --shard 1/2 on machine one, --shard 2/2 on machine two")
	fs.BoolVar(&cfg.Once, "once", false, "visit every assigned service once, then exit")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "log the requests that would be sent without sending them")
	fs.StringVar(&cfg.LogFile, "log-file", "", "append logs to this file (rotated at --log-max-mb); default: stdout")
	fs.BoolVar(&cfg.LogJSON, "log-json", false, "emit JSON log lines")
	fs.IntVar(&cfg.LogMaxMB, "log-max-mb", DefaultLogMaxMB, "rotate the log file once it exceeds this many megabytes")
	fs.StringVar(&cfg.Seed, "seed", "", "override the randomness seed (default: derived from this machine's identity)")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	fs.BoolVar(&cfg.ListServices, "list-services", false, "print the built-in services and the shard assignment, then exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "renderpulse keeps Render free-tier web services awake by sending\nrealistic, randomized requests well inside the 15-minute spin-down window.\n\nUsage: renderpulse [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	parsed, err := ParseShard(*shard)
	if err != nil {
		return nil, err
	}
	cfg.Shard = parsed
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ParseShard(s string) (Shard, error) {
	if s == "" {
		return Shard{Index: 1, Count: 1}, nil
	}
	parts := strings.SplitN(strings.TrimSpace(s), "/", 2)
	if len(parts) != 2 {
		return Shard{}, fmt.Errorf("--shard must look like 1/2, got %q", s)
	}
	i, errI := strconv.Atoi(parts[0])
	n, errN := strconv.Atoi(parts[1])
	if errI != nil || errN != nil {
		return Shard{}, fmt.Errorf("--shard must look like 1/2, got %q", s)
	}
	if n < 1 || i < 1 || i > n {
		return Shard{}, fmt.Errorf("--shard index must be within 1..%d, got %q", n, s)
	}
	return Shard{Index: i, Count: n}, nil
}

func (c *Config) validate() error {
	if c.IntervalMin <= 0 {
		return fmt.Errorf("--interval-min must be positive")
	}
	if c.IntervalMax < c.IntervalMin {
		return fmt.Errorf("--interval-max (%s) must be >= --interval-min (%s)", c.IntervalMax, c.IntervalMin)
	}
	if c.Timeout < 5*time.Second {
		return fmt.Errorf("--timeout must be at least 5s so Render cold starts can finish")
	}
	if c.ThinkMax < c.ThinkMin {
		return fmt.Errorf("--think-max must be >= --think-min")
	}
	if c.LogMaxMB < 1 {
		return fmt.Errorf("--log-max-mb must be at least 1")
	}
	if c.IntervalMax >= 14*time.Minute {
		return fmt.Errorf("--interval-max (%s) leaves no safety margin under Render's 15-minute spin-down", c.IntervalMax)
	}
	return nil
}
