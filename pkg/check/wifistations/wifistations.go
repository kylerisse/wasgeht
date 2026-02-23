// Package wifistations implements a check that scrapes a Prometheus metrics
// endpoint for wifi_stations gauge values, reporting connected client counts
// per radio interface.
package wifistations

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

const (
	// TypeName is the registered name for this check type.
	TypeName = "wifi_stations"

	// DefaultTimeout is the default HTTP scrape timeout.
	DefaultTimeout = 5 * time.Second

	// DefaultPort is the default port for the metrics endpoint.
	DefaultPort = "9100"

	// DefaultPath is the default path for the metrics endpoint.
	DefaultPath = "/metrics"

	// MetricName is the Prometheus metric name to look for.
	MetricName = "wifi_stations"
)

// WifiStations implements check.Check by scraping a Prometheus metrics endpoint.
type WifiStations struct {
	url     string
	radios  []radioConfig
	timeout time.Duration
	label   string
	client  *http.Client
	desc    check.Descriptor
}

// radioConfig maps a Prometheus ifname label to an RRD data source.
type radioConfig struct {
	ifname    string // Prometheus label value (e.g. "phy0-ap0")
	resultKey string // key in Result.Metrics (e.g. "phy0-ap0")
	dsName    string // RRD DS name (e.g. "radio0")
	label     string // human-readable label (e.g. "phy0-ap0")
}

// New creates a WifiStations check.
func New(url string, radios []radioConfig, label string, opts ...Option) (*WifiStations, error) {
	if url == "" {
		return nil, fmt.Errorf("wifi_stations: url must not be empty")
	}
	if len(radios) == 0 {
		return nil, fmt.Errorf("wifi_stations: at least one radio is required")
	}
	if label == "" {
		return nil, fmt.Errorf("wifi_stations: label must not be empty")
	}

	w := &WifiStations{
		url:     url,
		radios:  radios,
		label:   label,
		timeout: DefaultTimeout,
	}

	for _, opt := range opts {
		if err := opt(w); err != nil {
			return nil, fmt.Errorf("wifi_stations: %w", err)
		}
	}

	w.client = &http.Client{Timeout: w.timeout}

	metrics := make([]check.MetricDef, len(radios))
	for i, r := range radios {
		metrics[i] = check.MetricDef{
			ResultKey: r.resultKey,
			DSName:    r.dsName,
			Label:     r.label,
			Unit:      "clients",
			Scale:     0,
		}
	}
	w.desc = check.Descriptor{
		Label:   label,
		Metrics: metrics,
	}

	return w, nil
}

// Option is a functional option for configuring a WifiStations check.
type Option func(*WifiStations) error

// WithTimeout sets the HTTP scrape timeout.
func WithTimeout(d time.Duration) Option {
	return func(w *WifiStations) error {
		if d <= 0 {
			return fmt.Errorf("timeout must be positive, got %v", d)
		}
		w.timeout = d
		return nil
	}
}

// Type returns the check type name.
func (w *WifiStations) Type() string {
	return TypeName
}

// Describe returns the Descriptor for this check instance.
func (w *WifiStations) Describe() check.Descriptor {
	return w.desc
}

// Run scrapes the Prometheus endpoint and returns a Result.
func (w *WifiStations) Run(ctx context.Context) check.Result {
	now := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url, nil)
	if err != nil {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("wifi_stations: failed to create request: %w", err),
		}
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("wifi_stations: scrape failed: %w", err),
		}
	}
	defer resp.Body.Close()

	metrics, err := parseMetrics(resp.Body, w.radios)
	if err != nil {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("wifi_stations: %w", err),
		}
	}

	return check.Result{
		Timestamp: now,
		Success:   true,
		Metrics:   metrics,
	}
}

// Factory creates a WifiStations check from a config map.
// Required keys: "target" (string), "radios" (list), "label" (string).
// Optional keys: "url" (string), "timeout" (string).
func Factory(config map[string]any) (check.Check, error) {
	// Resolve the scrape URL
	var url string
	if v, ok := config["url"]; ok {
		urlStr, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("wifi_stations: 'url' must be a string, got %T", v)
		}
		url = urlStr
	} else {
		target, ok := config["target"]
		if !ok {
			return nil, fmt.Errorf("wifi_stations: config missing 'target' or 'url'")
		}
		targetStr, ok := target.(string)
		if !ok {
			return nil, fmt.Errorf("wifi_stations: 'target' must be a string, got %T", target)
		}
		url = fmt.Sprintf("http://%s:%s%s", targetStr, DefaultPort, DefaultPath)
	}

	// Parse radios config
	radiosRaw, ok := config["radios"]
	if !ok {
		return nil, fmt.Errorf("wifi_stations: config missing required key 'radios'")
	}

	radios, err := parseRadiosConfig(radiosRaw)
	if err != nil {
		return nil, err
	}

	// Require explicit label
	labelVal, ok := config["label"]
	if !ok {
		return nil, fmt.Errorf("wifi_stations: config missing required key 'label'")
	}
	labelStr, ok := labelVal.(string)
	if !ok {
		return nil, fmt.Errorf("wifi_stations: 'label' must be a string, got %T", labelVal)
	}
	if labelStr == "" {
		return nil, fmt.Errorf("wifi_stations: 'label' must not be empty")
	}

	var opts []Option

	if v, ok := config["timeout"]; ok {
		switch t := v.(type) {
		case string:
			d, err := time.ParseDuration(t)
			if err != nil {
				return nil, fmt.Errorf("wifi_stations: invalid timeout %q: %w", t, err)
			}
			opts = append(opts, WithTimeout(d))
		default:
			return nil, fmt.Errorf("wifi_stations: 'timeout' must be a duration string, got %T", v)
		}
	}

	return New(url, radios, labelStr, opts...)
}

// parseRadiosConfig converts the raw radios config value into radioConfig slices.
func parseRadiosConfig(raw any) ([]radioConfig, error) {
	var ifnames []string

	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("wifi_stations: each radio must be a string, got %T", item)
			}
			ifnames = append(ifnames, s)
		}
	case []string:
		ifnames = v
	default:
		return nil, fmt.Errorf("wifi_stations: 'radios' must be a list of strings, got %T", raw)
	}

	if len(ifnames) == 0 {
		return nil, fmt.Errorf("wifi_stations: 'radios' must not be empty")
	}

	radios := make([]radioConfig, len(ifnames))
	for i, ifname := range ifnames {
		radios[i] = radioConfig{
			ifname:    ifname,
			resultKey: ifname,
			dsName:    fmt.Sprintf("radio%d", i),
			label:     ifname,
		}
	}

	return radios, nil
}

func parseMetrics(r io.Reader, radios []radioConfig) (map[string]int64, error) {
	lookup := make(map[string]radioConfig, len(radios))
	for _, radio := range radios {
		lookup[radio.ifname] = radio
	}

	found := make(map[string]int64)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, MetricName+"{") {
			continue
		}

		ifname, value, err := parseLine(line)
		if err != nil {
			continue
		}

		if radio, ok := lookup[ifname]; ok {
			found[radio.resultKey] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading metrics: %w", err)
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no wifi_stations metrics found for configured radios")
	}

	return found, nil
}

func parseLine(line string) (string, int64, error) {
	ifnameStart := strings.Index(line, `ifname="`)
	if ifnameStart == -1 {
		return "", 0, fmt.Errorf("no ifname label found")
	}
	ifnameStart += len(`ifname="`)

	ifnameEnd := strings.Index(line[ifnameStart:], `"`)
	if ifnameEnd == -1 {
		return "", 0, fmt.Errorf("unterminated ifname label")
	}
	ifname := line[ifnameStart : ifnameStart+ifnameEnd]

	braceEnd := strings.Index(line, "}")
	if braceEnd == -1 {
		return "", 0, fmt.Errorf("no closing brace found")
	}

	valueStr := strings.TrimSpace(line[braceEnd+1:])
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid value %q: %w", valueStr, err)
	}

	return ifname, int64(value), nil
}
