package seed

import (
	"fmt"
	"hash/fnv"
	"os"
	"strings"
)

// Derive returns the two 64-bit values that seed the PCG generator. The
// material combines a stable machine identity with the shard index so two
// hosts (or two shards on one host) never produce the same request schedule.
func Derive(shardIndex int, explicit string) (uint64, uint64) {
	material := explicit
	if material == "" {
		material = machineIdentity()
	}
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%d", material, shardIndex)
	a := h.Sum64()
	return a, a ^ 0x9E3779B97F4A7C15
}

func machineIdentity() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				return s
			}
	}
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown-host"
	}
	return host
}
