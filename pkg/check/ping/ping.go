// Package ping implements a ping (ICMP echo) check for the check framework.
//
// It shells out to the system ping command for each configured address,
// reporting per-address latency as separate metrics. The addresses array
// is explicit in the check config; the worker-injected "target" key is ignored.
package ping

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

const (
	// TypeName is the registered name for this check type.
	TypeName = "ping"

	// DefaultTimeout is the default ping timeout.
	DefaultTimeout = 3 * time.Second

	// DefaultCount is the default number of ping packets.
	DefaultCount = 1
)

// addressConfig maps a single ping target to its RRD data source.
type addressConfig struct {
	address   string // IP or hostname to ping
	resultKey string // key in Result.Metrics (same as address)
	dsName    string // RRD DS name (e.g. "addr0")
	label     string // human-readable label (same as address)
}

// Ping implements check.Check using ICMP echo requests to one or more addresses.
type Ping struct {
	addresses []addressConfig
	timeout   time.Duration
	count     int
}

// New creates a Ping check with the given addresses and options.
func New(addresses []string, opts ...Option) (*Ping, error) {
	if len(addresses) == 0 {
		return nil, fmt.Errorf("ping: at least one address is required")
	}

	addrs := make([]addressConfig, len(addresses))
	for i, a := range addresses {
		if a == "" {
			return nil, fmt.Errorf("ping: address at index %d must not be empty", i)
		}
		addrs[i] = addressConfig{
			address:   a,
			resultKey: a,
			dsName:    fmt.Sprintf("addr%d", i),
			label:     a,
		}
	}

	p := &Ping{
		addresses: addrs,
		timeout:   DefaultTimeout,
		count:     DefaultCount,
	}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("ping: %w", err)
		}
	}

	return p, nil
}

// Option is a functional option for configuring a Ping check.
type Option func(*Ping) error

// WithTimeout sets the ping timeout duration.
func WithTimeout(d time.Duration) Option {
	return func(p *Ping) error {
		if d <= 0 {
			return fmt.Errorf("timeout must be positive, got %v", d)
		}
		p.timeout = d
		return nil
	}
}

// WithCount sets the number of ping packets to send per address.
func WithCount(n int) Option {
	return func(p *Ping) error {
		if n < 1 {
			return fmt.Errorf("count must be at least 1, got %d", n)
		}
		p.count = n
		return nil
	}
}

// Type returns the check type name.
func (p *Ping) Type() string {
	return TypeName
}

// Describe returns the Descriptor for this ping check instance.
// One metric is produced per configured address.
func (p *Ping) Describe() check.Descriptor {
	metrics := make([]check.MetricDef, len(p.addresses))
	for i, a := range p.addresses {
		metrics[i] = check.MetricDef{
			ResultKey: a.resultKey,
			DSName:    a.dsName,
			Label:     a.label,
			Unit:      "ms",
			Scale:     1000,
		}
	}
	return check.Descriptor{
		Label:   "ping",
		Metrics: metrics,
	}
}

// Run pings all configured addresses and returns a Result.
// Success requires all addresses to respond. Each address latency
// is stored in microseconds keyed by address string.
func (p *Ping) Run(ctx context.Context) check.Result {
	now := time.Now()
	metrics := make(map[string]*int64, len(p.addresses))
	var lastErr error
	succeeded := 0

	timeoutSec := fmt.Sprintf("%.0f", p.timeout.Seconds())
	count := strconv.Itoa(p.count)

	for _, a := range p.addresses {
		cmd := exec.CommandContext(ctx, "ping", "-c", count, "-W", timeoutSec, a.address)

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("ping %s: %w", a.address, err)
			metrics[a.resultKey] = nil
			continue
		}

		latency, err := parseOutput(out.String())
		if err != nil {
			lastErr = fmt.Errorf("ping %s: %w", a.address, err)
			metrics[a.resultKey] = nil
			continue
		}

		v := int64(latency.Microseconds())
		metrics[a.resultKey] = &v
		succeeded++
	}

	return check.Result{
		Timestamp: now,
		Success:   succeeded == len(p.addresses),
		Err:       lastErr,
		Metrics:   metrics,
	}
}

// Factory creates a Ping check from a config map.
// Required keys: "addresses" (list of strings).
// Optional keys: "timeout" (duration string), "count" (float64).
// The "target" key injected by the worker is ignored.
func Factory(config map[string]any) (check.Check, error) {
	addresses, err := extractAddresses(config)
	if err != nil {
		return nil, err
	}

	var opts []Option

	if v, ok := config["timeout"]; ok {
		switch t := v.(type) {
		case string:
			d, err := time.ParseDuration(t)
			if err != nil {
				return nil, fmt.Errorf("ping: invalid timeout %q: %w", t, err)
			}
			opts = append(opts, WithTimeout(d))
		default:
			return nil, fmt.Errorf("ping: 'timeout' must be a duration string, got %T", v)
		}
	}

	if v, ok := config["count"]; ok {
		switch c := v.(type) {
		case float64:
			opts = append(opts, WithCount(int(c)))
		default:
			return nil, fmt.Errorf("ping: 'count' must be a number, got %T", v)
		}
	}

	return New(addresses, opts...)
}

// extractAddresses pulls the addresses list from the config map.
func extractAddresses(config map[string]any) ([]string, error) {
	raw, ok := config["addresses"]
	if !ok {
		return nil, fmt.Errorf("ping: config missing required key 'addresses'")
	}

	switch v := raw.(type) {
	case []string:
		if len(v) == 0 {
			return nil, fmt.Errorf("ping: 'addresses' must not be empty")
		}
		return v, nil
	case []any:
		addrs := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("ping: 'addresses' items must be strings, got %T", item)
			}
			addrs = append(addrs, s)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("ping: 'addresses' must not be empty")
		}
		return addrs, nil
	default:
		return nil, fmt.Errorf("ping: 'addresses' must be a list, got %T", raw)
	}
}

// parseOutput extracts the round-trip time from ping command output.
func parseOutput(output string) (time.Duration, error) {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "rtt min/avg/max") || strings.Contains(line, "round-trip min/avg/max") {
			parts := strings.Split(line, "=")
			if len(parts) < 2 {
				continue
			}
			stats := strings.TrimSpace(parts[1])
			fields := strings.Split(stats, "/")
			if len(fields) < 2 {
				continue
			}
			avgStr := strings.TrimSpace(fields[1])
			avgMs, err := strconv.ParseFloat(avgStr, 64)
			if err != nil {
				continue
			}
			return time.Duration(avgMs * float64(time.Millisecond)), nil
		}
	}
	return 0, fmt.Errorf("could not parse ping output")
}
