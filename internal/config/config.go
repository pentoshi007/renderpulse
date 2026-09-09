package config

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pentoshi007/renderpulse/internal/services"
)

// Version is stamped by main before Parse so it shows in --help output.
var Version = "dev"

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
	Services      services.AddOptions
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
	fs.BoolVar(&cfg.ListServices, "list-services", false, "view the active services and the shard assignment, then exit")
	fs.Var(stringSlice{&cfg.Services.Adds}, "add", "add (or replace) a service as name=url, repeatable, e.g. --add blog=https://blog.onrender.com")
	fs.Var(stringSlice{&cfg.Services.Removes}, "remove", "remove a service by name, repeatable, e.g. --remove rider (applied after --add)")
	fs.StringVar(&cfg.Services.File, "services-file", "", "load extra services (with optional routes) from a JSON file")
	fs.BoolVar(&cfg.Services.NoBuiltin, "no-builtin", false, "drop the built-in tomato fleet; keep only --add / --services-file services")
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, `renderpulse %s — keep Render free-tier services awake with realistic, randomized traffic.

Render spins down a Free web service after 15 minutes without inbound traffic
(https://render.com/docs/free). renderpulse visits every service on its own
independent, randomized schedule (default every 3-7 minutes) so the idle timer
never expires. All requests are GETs with human-looking browser headers.

Usage:
  renderpulse [flags]

Managing services (view / add / remove):
  --list-services                     view the active fleet and shard assignment
  --add name=url                      add or replace a service (repeatable);
                                      a bare https://host also works
  --remove name                       remove a service by name (repeatable)
  --services-file FILE                add services from JSON, e.g.:
        [
          {"name":"blog","url":"https://blog.onrender.com",
           "routes":[{"path":"/","weight":10},{"path":"/api/posts","weight":5}]}
        ]
                                      services without routes get a generic pool:
                                      /, /api, /api/health, /health, /api/status,
                                      /status, /favicon.ico
  --no-builtin                        use only your own services

Precedence: --services-file overrides built-ins, --add overrides both,
--remove is applied last. Use --list-services to view the result.

Examples:
  renderpulse                                        # keep all 6 tomato services awake
  renderpulse --list-services                        # view the active fleet
  renderpulse --dry-run --once                       # preview one visit per service
  renderpulse --add blog=https://blog.onrender.com   # add a new URL
  renderpulse --remove rider                         # stop pinging one service
  renderpulse --no-builtin --services-file mine.json # fully custom fleet
  renderpulse --shard 1/2                            # machine 1 of 2 (services 1,3,5..)
  renderpulse --shard 2/2                            # machine 2 of 2 (services 2,4,6..)
  renderpulse --log-file /var/log/renderpulse.log --log-json

Flags:
`, Version)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, flag.ErrHelp // usage already printed; caller exits 0
		}
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

// stringSlice lets a flag be repeated and collect every occurrence.
type stringSlice struct{ p *[]string }

func (s stringSlice) String() string {
	if s.p == nil {
		return "" // flag pkg calls the zero value to render defaults
	}
	return strings.Join(*s.p, ",")
}

func (s stringSlice) Set(v string) error {
	*s.p = append(*s.p, v)
	return nil
}
