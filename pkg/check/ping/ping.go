// Package ping implements a ping (ICMP echo) check for the check framework.
//
// It shells out to the system ping command, parses the output for
// round-trip time, and returns a check.Result with a latency_us metric.
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

// Desc describes the metrics produced by a ping check.
var Desc = check.Descriptor{
	Metrics: []check.MetricDef{
		{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms", Scale: 1000},
	},
}

// Ping implements check.Check using ICMP echo requests.
type Ping struct {
	target  string
	timeout time.Duration
	count   int
}

// New creates a Ping check with the given target and options.
func New(target string, opts ...Option) (*Ping, error) {
	if target == "" {
		return nil, fmt.Errorf("ping: target must not be empty")
	}

	p := &Ping{
		target:  target,
		timeout: DefaultTimeout,
		count:   DefaultCount,
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

// WithCount sets the number of ping packets to send.
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

// Run executes the ping check and returns a Result.
func (p *Ping) Run(ctx context.Context) check.Result {
	now := time.Now()

	timeoutSec := fmt.Sprintf("%.0f", p.timeout.Seconds())
	count := strconv.Itoa(p.count)

	cmd := exec.CommandContext(ctx, "ping", "-c", count, "-W", timeoutSec, p.target)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("ping %s: %w", p.target, err),
		}
	}

	latency, err := parseOutput(out.String())
	if err != nil {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("ping %s: %w", p.target, err),
		}
	}

	return check.Result{
		Timestamp: now,
		Success:   true,
		Metrics: map[string]int64{
			"latency_us": int64(latency.Microseconds()),
		},
	}
}

// Factory creates a Ping check from a config map.
// Required key: "target" (string).
// Optional keys: "timeout" (string parseable by time.ParseDuration), "count" (float64).
func Factory(config map[string]any) (check.Check, error) {
	target, ok := config["target"]
	if !ok {
		return nil, fmt.Errorf("ping: config missing required key 'target'")
	}
	targetStr, ok := target.(string)
	if !ok {
		return nil, fmt.Errorf("ping: 'target' must be a string, got %T", target)
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

	return New(targetStr, opts...)
}

// parseOutput extracts the round-trip time from ping command output.
func parseOutput(output string) (time.Duration, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "time=") {
			continue
		}

		start := strings.Index(line, "time=") + len("time=")
		end := strings.IndexAny(line[start:], " ")
		if end == -1 {
			end = len(line[start:])
		}
		rttStr := line[start : start+end]

		unitStart := start + end
		unit := strings.TrimSpace(line[unitStart:])

		rtt, err := strconv.ParseFloat(strings.TrimSpace(rttStr), 64)
		if err != nil {
			return 0, fmt.Errorf("could not parse RTT %q: %w", rttStr, err)
		}

		switch {
		case strings.Contains(unit, "ms"):
			return time.Duration(rtt * float64(time.Millisecond)), nil
		case strings.Contains(unit, "us") || strings.Contains(unit, "µs"):
			return time.Duration(rtt * float64(time.Microsecond)), nil
		case unit == "s":
			return time.Duration(rtt * float64(time.Second)), nil
		}

		return 0, fmt.Errorf("could not determine time unit from %q", unit)
	}
	return 0, fmt.Errorf("RTT not found in ping output")
}
