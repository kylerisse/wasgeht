package wifistations

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

const sampleMetrics = `# HELP wifi_stations Number of connected stations
# TYPE wifi_stations gauge
wifi_stations{ifname="phy0-ap0"} 3
wifi_stations{ifname="phy1-ap0"} 7
# HELP node_cpu_seconds_total Total CPU time
# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="0",mode="idle"} 12345.67
`

func testRadios() []radioConfig {
	return []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
		{ifname: "phy1-ap0", resultKey: "phy1-ap0", dsName: "radio1", label: "phy1-ap0"},
	}
}

// --- New() tests ---

func TestNew_Valid(t *testing.T) {
	w, err := New("ap1.local", testRadios())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.url != "http://ap1.local:9100/metrics" {
		t.Errorf("expected url 'http://ap1.local:9100/metrics', got %q", w.url)
	}
	if w.timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %v", w.timeout)
	}
	if len(w.radios) != 2 {
		t.Errorf("expected 2 radios, got %d", len(w.radios))
	}
}

func TestNew_URLBuiltFromAddress(t *testing.T) {
	w, err := New("192.168.1.1", testRadios())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := fmt.Sprintf("http://192.168.1.1:%s%s", DefaultPort, DefaultPath)
	if w.url != expected {
		t.Errorf("expected url %q, got %q", expected, w.url)
	}
}

func TestNew_EmptyAddress(t *testing.T) {
	_, err := New("", testRadios())
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestNew_EmptyRadios(t *testing.T) {
	_, err := New("ap1.local", []radioConfig{})
	if err == nil {
		t.Error("expected error for empty radios")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	w, err := New("ap1.local", testRadios(), WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", w.timeout)
	}
}

func TestWithTimeout_Invalid(t *testing.T) {
	_, err := New("ap1.local", testRadios(), WithTimeout(-1*time.Second))
	if err == nil {
		t.Error("expected error for negative timeout")
	}
}

// --- Describe() tests ---

func TestDescribe_Label(t *testing.T) {
	w, _ := New("ap1.local", testRadios())
	desc := w.Describe()
	if desc.Label != "wifi stations" {
		t.Errorf("expected Descriptor.Label 'wifi stations', got %q", desc.Label)
	}
}

func TestDescribe_IncludesTotal(t *testing.T) {
	w, _ := New("ap1.local", testRadios())
	desc := w.Describe()
	// 2 radios + 1 total = 3 metrics
	if len(desc.Metrics) != 3 {
		t.Fatalf("expected 3 metrics (2 radios + total), got %d", len(desc.Metrics))
	}
	last := desc.Metrics[len(desc.Metrics)-1]
	if last.DSName != TotalDSName {
		t.Errorf("expected last metric DSName %q, got %q", TotalDSName, last.DSName)
	}
	if last.ResultKey != TotalResultKey {
		t.Errorf("expected last metric ResultKey %q, got %q", TotalResultKey, last.ResultKey)
	}
	if last.Label != "total" {
		t.Errorf("expected last metric Label 'total', got %q", last.Label)
	}
	if last.Unit != "clients" {
		t.Errorf("expected last metric Unit 'clients', got %q", last.Unit)
	}
}

func TestDescribe_RadioMetrics(t *testing.T) {
	w, _ := New("ap1.local", testRadios())
	desc := w.Describe()
	if desc.Metrics[0].DSName != "radio0" {
		t.Errorf("expected DSName 'radio0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[1].DSName != "radio1" {
		t.Errorf("expected DSName 'radio1', got %q", desc.Metrics[1].DSName)
	}
}

func TestDescribe_SingleRadio_HasTotal(t *testing.T) {
	radios := []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
	}
	w, _ := New("ap1.local", radios)
	desc := w.Describe()
	// 1 radio + 1 total = 2 metrics
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics (1 radio + total), got %d", len(desc.Metrics))
	}
}

func TestDescribe_DynamicPerConfig(t *testing.T) {
	w1, _ := New("a.local", []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
	})
	w2, _ := New("b.local", []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
		{ifname: "phy1-ap0", resultKey: "phy1-ap0", dsName: "radio1", label: "phy1-ap0"},
		{ifname: "phy2-ap0", resultKey: "phy2-ap0", dsName: "radio2", label: "phy2-ap0"},
	})

	// 1 radio + total = 2
	if len(w1.Describe().Metrics) != 2 {
		t.Errorf("expected 2 metrics for w1, got %d", len(w1.Describe().Metrics))
	}
	// 3 radios + total = 4
	if len(w2.Describe().Metrics) != 4 {
		t.Errorf("expected 4 metrics for w2, got %d", len(w2.Describe().Metrics))
	}
}

// --- Run() tests ---

func TestRun_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sampleMetrics)
	}))
	defer server.Close()

	// Use a radio config that points to the test server's host:port
	// We construct the check directly with the server URL via a custom address
	// by overriding url after construction isn't possible, so we use a factory approach.
	radios := testRadios()
	w := &WifiStations{
		url:    server.URL,
		radios: radios,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	result := w.Run(context.Background())
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Err)
	}
	if p := result.Metrics["phy0-ap0"]; p == nil || *p != 3 {
		t.Errorf("expected phy0-ap0=3, got %v", result.Metrics["phy0-ap0"])
	}
	if p := result.Metrics["phy1-ap0"]; p == nil || *p != 7 {
		t.Errorf("expected phy1-ap0=7, got %v", result.Metrics["phy1-ap0"])
	}
	if p := result.Metrics[TotalResultKey]; p == nil || *p != 10 {
		t.Errorf("expected total=10, got %v", result.Metrics[TotalResultKey])
	}
}

func TestRun_TotalIsSum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `wifi_stations{ifname="phy0-ap0"} 5`)
		fmt.Fprintln(w, `wifi_stations{ifname="phy1-ap0"} 12`)
	}))
	defer server.Close()

	w := &WifiStations{
		url:    server.URL,
		radios: testRadios(),
		client: &http.Client{Timeout: 5 * time.Second},
	}

	result := w.Run(context.Background())
	if !result.Success {
		t.Fatalf("unexpected failure: %v", result.Err)
	}
	if p := result.Metrics[TotalResultKey]; p == nil || *p != 17 {
		t.Errorf("expected total=17, got %v", result.Metrics[TotalResultKey])
	}
}

func TestRun_500Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	w := &WifiStations{
		url:    server.URL,
		radios: testRadios(),
		client: &http.Client{Timeout: 5 * time.Second},
	}
	result := w.Run(context.Background())
	if result.Success {
		t.Error("expected failure for 500 response")
	}
}

func TestRun_ConnectionError(t *testing.T) {
	w := &WifiStations{
		url:    "http://127.0.0.1:1/metrics",
		radios: testRadios(),
		client: &http.Client{Timeout: 1 * time.Second},
	}
	result := w.Run(context.Background())
	if result.Success {
		t.Error("expected failure for connection error")
	}
}

func TestRun_NoMatchingRadios(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# just a comment")
	}))
	defer server.Close()

	w := &WifiStations{
		url:    server.URL,
		radios: testRadios(),
		client: &http.Client{Timeout: 5 * time.Second},
	}
	result := w.Run(context.Background())
	if result.Success {
		t.Error("expected failure when no matching radios found")
	}
}

// --- parseLine tests ---

func TestParseLine_Valid(t *testing.T) {
	ifname, value, err := parseLine(`wifi_stations{ifname="phy0-ap0"} 3`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ifname != "phy0-ap0" {
		t.Errorf("expected ifname 'phy0-ap0', got %q", ifname)
	}
	if value != 3 {
		t.Errorf("expected value 3, got %d", value)
	}
}

func TestParseLine_FloatValue(t *testing.T) {
	_, value, err := parseLine(`wifi_stations{ifname="phy1-ap0"} 7.0`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != 7 {
		t.Errorf("expected value 7, got %d", value)
	}
}

func TestParseLine_MultipleLabels(t *testing.T) {
	ifname, value, err := parseLine(`wifi_stations{ifname="phy0-ap0",ssid="mynet"} 12`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ifname != "phy0-ap0" {
		t.Errorf("expected ifname 'phy0-ap0', got %q", ifname)
	}
	if value != 12 {
		t.Errorf("expected value 12, got %d", value)
	}
}

func TestParseLine_NoIfname(t *testing.T) {
	_, _, err := parseLine(`wifi_stations{ssid="mynet"} 3`)
	if err == nil {
		t.Error("expected error for missing ifname label")
	}
}

func TestParseLine_InvalidValue(t *testing.T) {
	_, _, err := parseLine(`wifi_stations{ifname="phy0-ap0"} abc`)
	if err == nil {
		t.Error("expected error for non-numeric value")
	}
}

func TestParseLine_ZeroValue(t *testing.T) {
	_, value, err := parseLine(`wifi_stations{ifname="phy0-ap0"} 0`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != 0 {
		t.Errorf("expected value 0, got %d", value)
	}
}

// --- parseMetrics tests ---

func TestParseMetrics_Success(t *testing.T) {
	r := strings.NewReader(sampleMetrics)
	values, err := parseMetrics(r, testRadios())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if values["phy0-ap0"] != 3 {
		t.Errorf("expected phy0-ap0=3, got %d", values["phy0-ap0"])
	}
	if values["phy1-ap0"] != 7 {
		t.Errorf("expected phy1-ap0=7, got %d", values["phy1-ap0"])
	}
}

func TestParseMetrics_DoesNotIncludeTotal(t *testing.T) {
	// parseMetrics itself should NOT include total; Run() adds it
	r := strings.NewReader(sampleMetrics)
	values, err := parseMetrics(r, testRadios())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := values[TotalResultKey]; ok {
		t.Error("parseMetrics should not include 'total'; that is added by Run()")
	}
}

func TestParseMetrics_IgnoresUnknownRadios(t *testing.T) {
	input := `wifi_stations{ifname="phy0-ap0"} 3
wifi_stations{ifname="phy2-ap0"} 99
`
	r := strings.NewReader(input)
	radios := []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
	}
	values, err := parseMetrics(r, radios)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 {
		t.Errorf("expected 1 value, got %d", len(values))
	}
}

func TestParseMetrics_EmptyInput(t *testing.T) {
	r := strings.NewReader("")
	_, err := parseMetrics(r, testRadios())
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseMetrics_OnlyComments(t *testing.T) {
	r := strings.NewReader("# just comments\n")
	_, err := parseMetrics(r, testRadios())
	if err == nil {
		t.Error("expected error when no matching metrics found")
	}
}

// --- Factory tests ---

func TestFactory_WithAddress(t *testing.T) {
	config := map[string]any{
		"address": "ap1.local",
		"radios":  []any{"phy0-ap0", "phy1-ap0"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w := chk.(*WifiStations)
	if w.url != "http://ap1.local:9100/metrics" {
		t.Errorf("expected url 'http://ap1.local:9100/metrics', got %q", w.url)
	}
	if len(w.radios) != 2 {
		t.Errorf("expected 2 radios, got %d", len(w.radios))
	}
}

func TestFactory_IgnoresLabelKey(t *testing.T) {
	config := map[string]any{
		"address": "ap1.local",
		"radios":  []any{"phy0-ap0"},
		"label":   "ignored",
	}
	_, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactory_WithTimeout(t *testing.T) {
	config := map[string]any{
		"address": "ap1.local",
		"radios":  []any{"phy0-ap0"},
		"timeout": "10s",
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w := chk.(*WifiStations)
	if w.timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", w.timeout)
	}
}

func TestFactory_MissingAddress(t *testing.T) {
	config := map[string]any{
		"radios": []any{"phy0-ap0"},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing address")
	}
}

func TestFactory_EmptyAddress(t *testing.T) {
	config := map[string]any{
		"address": "",
		"radios":  []any{"phy0-ap0"},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestFactory_WrongAddressType(t *testing.T) {
	config := map[string]any{
		"address": 123,
		"radios":  []any{"phy0-ap0"},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string address")
	}
}

func TestFactory_MissingRadios(t *testing.T) {
	config := map[string]any{
		"address": "ap1.local",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing radios")
	}
}

func TestFactory_EmptyRadios(t *testing.T) {
	config := map[string]any{
		"address": "ap1.local",
		"radios":  []any{},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty radios")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	config := map[string]any{
		"address": "ap1.local",
		"radios":  []any{"phy0-ap0"},
		"timeout": "nope",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestFactory_NoTargetOrURL(t *testing.T) {
	// Old "target" key should no longer work
	config := map[string]any{
		"target": "ap1.local",
		"radios": []any{"phy0-ap0"},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error: 'target' key no longer accepted, must use 'address'")
	}
}

// --- Registry integration ---

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	if err := reg.Register(TypeName, Factory); err != nil {
		t.Fatalf("failed to register wifi_stations: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sampleMetrics)
	}))
	defer server.Close()

	// We can't directly pass the test server URL as "address" since New() builds
	// the URL from address:DefaultPort/DefaultPath. Use a direct struct for Run tests.
	chk, err := reg.Create("wifi_stations", map[string]any{
		"address": "ap1.local",
		"radios":  []any{"phy0-ap0", "phy1-ap0"},
	})
	if err != nil {
		t.Fatalf("failed to create wifi_stations check: %v", err)
	}

	if chk.Type() != "wifi_stations" {
		t.Errorf("expected type 'wifi_stations', got %q", chk.Type())
	}

	desc := chk.Describe()
	if desc.Label != "wifi stations" {
		t.Errorf("expected Descriptor.Label 'wifi stations', got %q", desc.Label)
	}
	// 2 radios + total = 3
	if len(desc.Metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].DSName != "radio0" {
		t.Errorf("expected first DSName 'radio0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[2].DSName != TotalDSName {
		t.Errorf("expected last DSName %q, got %q", TotalDSName, desc.Metrics[2].DSName)
	}
}
