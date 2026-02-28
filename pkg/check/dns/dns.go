// Package dns implements a DNS query check that resolves one or more names
// against a specific server and validates each answer against an expected value.
// Supported record types are A, AAAA, and PTR. The check succeeds only when
// every configured query resolves and its answer matches the expected value.
package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/miekg/dns"
)

const (
	// TypeName is the registered name for this check type.
	TypeName = "dns"

	// DefaultTimeout is the default DNS query timeout.
	DefaultTimeout = 3 * time.Second
)

// queryConfig holds the parsed configuration for a single DNS query.
type queryConfig struct {
	name      string // query name as provided (without trailing dot)
	qtype     uint16 // dns.TypeA, dns.TypeAAAA, dns.TypePTR
	expect    string // expected value in the answer (normalized)
	resultKey string // key in Result.Metrics (= name)
	dsName    string // RRD DS name (e.g. "q0", "q1")
}

// Check implements check.Check using DNS queries to a specific server.
type Check struct {
	server  string        // host:port of the DNS server
	timeout time.Duration
	queries []queryConfig
	client  *dns.Client
	desc    check.Descriptor
}

// Option is a functional option for configuring a DNS Check.
type Option func(*Check) error

// WithTimeout sets the DNS query timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Check) error {
		if d <= 0 {
			return fmt.Errorf("timeout must be positive, got %v", d)
		}
		c.timeout = d
		return nil
	}
}

// New creates a DNS Check targeting the given server with the given queries.
func New(server string, queries []queryConfig, opts ...Option) (*Check, error) {
	if server == "" {
		return nil, fmt.Errorf("dns: server must not be empty")
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("dns: at least one query is required")
	}

	c := &Check{
		server:  server,
		timeout: DefaultTimeout,
		queries: queries,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("dns: %w", err)
		}
	}

	c.client = &dns.Client{
		Timeout: c.timeout,
	}

	metrics := make([]check.MetricDef, len(queries))
	for i, q := range queries {
		metrics[i] = check.MetricDef{
			ResultKey: q.resultKey,
			DSName:    q.dsName,
			Label:     q.resultKey,
			Unit:      "ms",
			Scale:     1000,
		}
	}
	c.desc = check.Descriptor{
		Label:   "dns",
		Metrics: metrics,
	}

	return c, nil
}

// Type returns the check type name.
func (c *Check) Type() string {
	return TypeName
}

// Describe returns the Descriptor for this check instance.
func (c *Check) Describe() check.Descriptor {
	return c.desc
}

// Run executes all configured DNS queries against the server and returns a Result.
// Success requires every query to resolve and every answer to match its expected value.
// Each query's RTT is stored in microseconds keyed by the query name.
func (c *Check) Run(ctx context.Context) check.Result {
	metrics := make(map[string]*int64, len(c.queries))
	var lastErr error
	succeeded := 0

	for _, q := range c.queries {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(q.name), q.qtype)
		msg.RecursionDesired = true

		resp, rtt, err := c.client.ExchangeContext(ctx, msg, c.server)
		if err != nil {
			lastErr = fmt.Errorf("dns %s %s: %w", qtypeName(q.qtype), q.name, err)
			metrics[q.resultKey] = nil
			continue
		}

		if resp.Rcode != dns.RcodeSuccess {
			lastErr = fmt.Errorf("dns %s %s: rcode %s", qtypeName(q.qtype), q.name, dns.RcodeToString[resp.Rcode])
			metrics[q.resultKey] = nil
			continue
		}

		if err := validateAnswer(resp.Answer, q.qtype, q.expect); err != nil {
			lastErr = fmt.Errorf("dns %s %s: %w", qtypeName(q.qtype), q.name, err)
			metrics[q.resultKey] = nil
			continue
		}

		v := rtt.Microseconds()
		metrics[q.resultKey] = &v
		succeeded++
	}

	return check.Result{
		Timestamp: time.Now(),
		Success:   succeeded == len(c.queries),
		Err:       lastErr,
		Metrics:   metrics,
	}
}

// validateAnswer checks that at least one RR in the answer section matches
// the expected value for the given query type.
func validateAnswer(rrs []dns.RR, qtype uint16, expect string) error {
	for _, rr := range rrs {
		switch qtype {
		case dns.TypeA:
			if a, ok := rr.(*dns.A); ok {
				if normalizeIP(a.A.String()) == normalizeIP(expect) {
					return nil
				}
			}
		case dns.TypeAAAA:
			if aaaa, ok := rr.(*dns.AAAA); ok {
				if normalizeIP(aaaa.AAAA.String()) == normalizeIP(expect) {
					return nil
				}
			}
		case dns.TypePTR:
			if ptr, ok := rr.(*dns.PTR); ok {
				if normalizeFQDN(ptr.Ptr) == normalizeFQDN(expect) {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("expected %q not found in answer", expect)
}

// normalizeIP parses and re-serializes an IP address string for comparison,
// handling IPv4-in-IPv6 representations and leading zeros.
func normalizeIP(s string) string {
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	return ip.String()
}

// normalizeFQDN strips the trailing dot so that "example.com." and "example.com"
// compare equal.
func normalizeFQDN(s string) string {
	return strings.TrimSuffix(s, ".")
}

// qtypeName returns a human-readable record type name for error messages.
func qtypeName(qtype uint16) string {
	switch qtype {
	case dns.TypeA:
		return "A"
	case dns.TypeAAAA:
		return "AAAA"
	case dns.TypePTR:
		return "PTR"
	default:
		return fmt.Sprintf("TYPE%d", qtype)
	}
}

// parseQType converts a record type string to a miekg/dns type constant.
// Supported values (case-insensitive): A, AAAA, PTR.
func parseQType(s string) (uint16, error) {
	switch strings.ToUpper(s) {
	case "A":
		return dns.TypeA, nil
	case "AAAA":
		return dns.TypeAAAA, nil
	case "PTR":
		return dns.TypePTR, nil
	default:
		return 0, fmt.Errorf("unsupported query type %q (supported: A, AAAA, PTR)", s)
	}
}

// Factory creates a DNS Check from a config map.
// Required keys:
//   - "server" (string) — host:port of the DNS server to query
//   - "queries" (list of objects) — each with "name", "type", and "expect"
//
// Optional keys:
//   - "timeout" (string) — duration string (e.g. "5s"), default "3s"
func Factory(config map[string]any) (check.Check, error) {
	serverRaw, ok := config["server"]
	if !ok {
		return nil, fmt.Errorf("dns: config missing required key 'server'")
	}
	server, ok := serverRaw.(string)
	if !ok {
		return nil, fmt.Errorf("dns: 'server' must be a string, got %T", serverRaw)
	}
	if server == "" {
		return nil, fmt.Errorf("dns: 'server' must not be empty")
	}

	queries, err := extractQueries(config)
	if err != nil {
		return nil, err
	}

	var opts []Option

	if v, ok := config["timeout"]; ok {
		ts, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("dns: 'timeout' must be a string, got %T", v)
		}
		d, err := time.ParseDuration(ts)
		if err != nil {
			return nil, fmt.Errorf("dns: invalid timeout %q: %w", ts, err)
		}
		opts = append(opts, WithTimeout(d))
	}

	return New(server, queries, opts...)
}

// extractQueries parses the "queries" list from the config map.
func extractQueries(config map[string]any) ([]queryConfig, error) {
	raw, ok := config["queries"]
	if !ok {
		return nil, fmt.Errorf("dns: config missing required key 'queries'")
	}

	rawList, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("dns: 'queries' must be a list, got %T", raw)
	}
	if len(rawList) == 0 {
		return nil, fmt.Errorf("dns: 'queries' must not be empty")
	}

	queries := make([]queryConfig, 0, len(rawList))
	for i, item := range rawList {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("dns: query at index %d must be an object, got %T", i, item)
		}

		name, ok := m["name"].(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("dns: query at index %d missing required 'name'", i)
		}

		typeStr, ok := m["type"].(string)
		if !ok || typeStr == "" {
			return nil, fmt.Errorf("dns: query at index %d missing required 'type'", i)
		}

		qtype, err := parseQType(typeStr)
		if err != nil {
			return nil, fmt.Errorf("dns: query at index %d: %w", i, err)
		}

		expect, ok := m["expect"].(string)
		if !ok || expect == "" {
			return nil, fmt.Errorf("dns: query at index %d missing required 'expect'", i)
		}

		queries = append(queries, queryConfig{
			name:      name,
			qtype:     qtype,
			expect:    expect,
			resultKey: name,
			dsName:    fmt.Sprintf("q%d", i),
		})
	}

	return queries, nil
}
