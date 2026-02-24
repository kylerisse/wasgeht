// Package wifistations implements a check that scrapes a Prometheus metrics
// endpoint for wifi_stations gauge values, reporting connected client counts
// per radio interface plus a derived total.
//
// The check expects the target to expose metrics in Prometheus text format
// with lines like:
//
//	wifi_stations{ifname="phy0-ap0"} 3
//	wifi_stations{ifname="phy1-ap0"} 7
//
// Each radio becomes a separate RRD data source. A "total" data source is
// also stored, computed as the sum of all configured radios.
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

	// TotalResultKey is the key used in Result.Metrics for the derived total.
	TotalResultKey = "total"

	// TotalDSName is the RRD data source name for the derived total.
	TotalDSName = "total"
)

// WifiStations implements check.Check by scraping a Prometheus metrics endpoint.
type WifiStations struct {
	url     string
	radios  []radioConfig
	timeout time.Duration
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
// The descriptor includes one metric per radio plus a derived "total" metric.
func New(address string, radios []radioConfig, opts ...Option) (*WifiStations, error) {
	if address == "" {
		return nil, fmt.Errorf("wifi_stations: address must not be empty")
	}
	if len(radios) == 0 {
		return nil, fmt.Errorf("wifi_stations: at least one radio is required")
	}

	url := fmt.Sprintf("http://%s:%s%s", address, DefaultPort, DefaultPath)

	w := &WifiStations{
		url:     url,
		radios:  radios,
		timeout: DefaultTimeout,
	}

	for _, opt := range opts {
		if err := opt(w); err != nil {
			return nil, fmt.Errorf("wifi_stations: %w", err)
		}
	}

	w.client = &http.Client{Timeout: w.timeout}

	// Build descriptor: one metric per radio, plus total as the last entry.
	metrics := make([]check.MetricDef, 0, len(radios)+1)
	for i, r := range radios {
		metrics = append(metrics, check.MetricDef{
			ResultKey: r.resultKey,
			DSName:    fmt.Sprintf("radio%d", i),
			Label:     r.label,
			Unit:      "clients",
			Scale:     0,
		})
	}
	metrics = append(metrics, check.MetricDef{
		ResultKey: TotalResultKey,
		DSName:    TotalDSName,
		Label:     "total",
		Unit:      "clients",
		Scale:     0,
	})

	w.desc = check.Descriptor{
		Label:   "wifi stations",
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
// Metrics include per-radio client counts and a derived "total".
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

	if resp.StatusCode != http.StatusOK {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("wifi_stations: unexpected status %d", resp.StatusCode),
		}
	}

	radioMetrics, err := parseMetrics(resp.Body, w.radios)
	if err != nil {
		return check.Result{
			Timestamp: now,
			Success:   false,
			Err:       fmt.Errorf("wifi_stations: %w", err),
		}
	}

	// Compute derived total from all found radio values.
	var total int64
	for _, v := range radioMetrics {
		total += v
	}
	radioMetrics[TotalResultKey] = total

	return check.Result{
		Timestamp: now,
		Success:   true,
		Metrics:   radioMetrics,
	}
}

// Factory creates a WifiStations check from a config map.
// Required keys: "address" (string), "radios" (list).
// Optional keys: "timeout" (string).
func Factory(config map[string]any) (check.Check, error) {
	addressVal, ok := config["address"]
	if !ok {
		return nil, fmt.Errorf("wifi_stations: config missing required key 'address'")
	}
	addressStr, ok := addressVal.(string)
	if !ok {
		return nil, fmt.Errorf("wifi_stations: 'address' must be a string, got %T", addressVal)
	}
	if addressStr == "" {
		return nil, fmt.Errorf("wifi_stations: 'address' must not be empty")
	}

	radiosRaw, ok := config["radios"]
	if !ok {
		return nil, fmt.Errorf("wifi_stations: config missing required key 'radios'")
	}
	radios, err := parseRadiosConfig(radiosRaw)
	if err != nil {
		return nil, err
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

	return New(addressStr, radios, opts...)
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
