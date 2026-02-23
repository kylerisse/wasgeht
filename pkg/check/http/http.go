package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

const (
	// TypeName is the registered name for this check type.
	TypeName = "http"

	// DefaultTimeout is the default HTTP request timeout.
	DefaultTimeout = 10 * time.Second
)

// Check implements check.Check using HTTP GET requests to one or more URLs.
type Check struct {
	urls       []string
	timeout    time.Duration
	skipVerify bool
	client     *http.Client
	desc       check.Descriptor
}

// Option is a functional option for configuring an HTTP Check.
type Option func(*Check) error

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Check) error {
		if d <= 0 {
			return fmt.Errorf("timeout must be positive, got %v", d)
		}
		c.timeout = d
		return nil
	}
}

// WithSkipVerify sets whether to skip TLS certificate verification.
func WithSkipVerify(skip bool) Option {
	return func(c *Check) error {
		c.skipVerify = skip
		return nil
	}
}

// New creates an HTTP Check for the given URLs.
func New(urls []string, opts ...Option) (*Check, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("http: at least one URL is required")
	}

	c := &Check{
		urls:       urls,
		timeout:    DefaultTimeout,
		skipVerify: true,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("http: %w", err)
		}
	}

	c.client = &http.Client{
		Timeout: c.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.skipVerify},
		},
	}

	// Build descriptor: one metric per URL
	metrics := make([]check.MetricDef, len(urls))
	for i, url := range urls {
		metrics[i] = check.MetricDef{
			ResultKey: url,
			DSName:    fmt.Sprintf("url%d", i),
			Label:     url,
			Unit:      "ms",
			Scale:     1000,
		}
	}
	c.desc = check.Descriptor{
		Label:   "response time",
		Metrics: metrics,
	}

	return c, nil
}

// Type returns the check type name.
func (c *Check) Type() string {
	return TypeName
}

// Describe returns the Descriptor for this check instance.
// The metrics vary based on the configured URLs.
func (c *Check) Describe() check.Descriptor {
	return c.desc
}

// Run executes HTTP GET requests to all configured URLs and returns
// a Result. The check succeeds if ALL URLs return a response (any
// HTTP status code counts as reachable). Each URL's response time
// is reported as a separate metric in microseconds.
func (c *Check) Run(ctx context.Context) check.Result {
	result := check.Result{
		Timestamp: time.Now(),
		Metrics:   make(map[string]int64),
	}

	var lastErr error

	for _, url := range c.urls {
		start := time.Now()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request for %s: %w", url, err)
			continue
		}

		resp, err := c.client.Do(req)
		elapsed := time.Since(start)

		if err != nil {
			lastErr = fmt.Errorf("request to %s failed: %w", url, err)
			continue
		}
		resp.Body.Close()

		// Store response time in microseconds (keyed by URL)
		result.Metrics[url] = elapsed.Microseconds()
	}

	// Success only if ALL URLs responded
	if len(result.Metrics) == len(c.urls) {
		result.Success = true
	} else {
		result.Success = false
		result.Err = lastErr
	}

	return result
}

// Factory creates an HTTP Check from a config map.
//
// Required key: "urls" — list of URL strings.
// Optional keys:
//   - "timeout" (string) — duration string (e.g. "10s")
//   - "skip_verify" (bool) — skip TLS cert verification (default: true)
//
// The "target" key injected by the worker is ignored; HTTP checks
// get their targets from the "urls" list.
func Factory(config map[string]any) (check.Check, error) {
	urls, err := extractURLs(config)
	if err != nil {
		return nil, err
	}

	var opts []Option

	if t, ok := config["timeout"]; ok {
		ts, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("http: 'timeout' must be a string, got %T", t)
		}
		d, err := time.ParseDuration(ts)
		if err != nil {
			return nil, fmt.Errorf("http: invalid timeout %q: %w", ts, err)
		}
		opts = append(opts, WithTimeout(d))
	}

	if v, ok := config["skip_verify"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("http: 'skip_verify' must be a bool, got %T", v)
		}
		opts = append(opts, WithSkipVerify(b))
	}

	return New(urls, opts...)
}

// extractURLs pulls the URL list from the config map.
func extractURLs(config map[string]any) ([]string, error) {
	raw, ok := config["urls"]
	if !ok {
		return nil, fmt.Errorf("http: config missing required key 'urls'")
	}

	switch v := raw.(type) {
	case []string:
		if len(v) == 0 {
			return nil, fmt.Errorf("http: 'urls' must not be empty")
		}
		return v, nil
	case []any:
		urls := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("http: 'urls' items must be strings, got %T", item)
			}
			urls = append(urls, s)
		}
		if len(urls) == 0 {
			return nil, fmt.Errorf("http: 'urls' must not be empty")
		}
		return urls, nil
	default:
		return nil, fmt.Errorf("http: 'urls' must be a list, got %T", raw)
	}
}
