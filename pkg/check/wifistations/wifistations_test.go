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

func TestNew_Valid(t *testing.T) {
	w, err := New("http://localhost:9100/metrics", testRadios())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.url != "http://localhost:9100/metrics" {
		t.Errorf("expected url, got %q", w.url)
	}
	if w.timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %v", w.timeout)
	}
	if len(w.radios) != 2 {
		t.Errorf("expected 2 radios, got %d", len(w.radios))
	}
}

func TestNew_EmptyURL(t *testing.T) {
	_, err := New("", testRadios())
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestNew_EmptyRadios(t *testing.T) {
	_, err := New("http://localhost:9100/metrics", []radioConfig{})
	if err == nil {
		t.Error("expected error for empty radios")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	w, err := New("http://localhost:9100/metrics", testRadios(), WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", w.timeout)
	}
}

func TestWithTimeout_Invalid(t *testing.T) {
	_, err := New("http://localhost:9100/metrics", testRadios(), WithTimeout(-1*time.Second))
	if err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestType(t *testing.T) {
	w, _ := New("http://localhost:9100/metrics", testRadios())
	if w.Type() != "wifi_stations" {
		t.Errorf("expected type 'wifi_stations', got %q", w.Type())
	}
}

func TestDescribe(t *testing.T) {
	w, _ := New("http://localhost:9100/metrics", testRadios())
	desc := w.Describe()

	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(desc.Metrics))
	}

	if desc.Metrics[0].ResultKey != "phy0-ap0" {
		t.Errorf("expected ResultKey 'phy0-ap0', got %q", desc.Metrics[0].ResultKey)
	}
	if desc.Metrics[0].DSName != "radio0" {
		t.Errorf("expected DSName 'radio0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[0].Unit != "clients" {
		t.Errorf("expected Unit 'clients', got %q", desc.Metrics[0].Unit)
	}
	if desc.Metrics[1].ResultKey != "phy1-ap0" {
		t.Errorf("expected ResultKey 'phy1-ap0', got %q", desc.Metrics[1].ResultKey)
	}
	if desc.Metrics[1].DSName != "radio1" {
		t.Errorf("expected DSName 'radio1', got %q", desc.Metrics[1].DSName)
	}
}

func TestRun_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleMetrics))
	}))
	defer server.Close()

	w, err := New(server.URL, testRadios())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := w.Run(context.Background())
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Err)
	}
	if result.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	if result.Metrics["phy0-ap0"] != 3 {
		t.Errorf("expected phy0-ap0=3, got %d", result.Metrics["phy0-ap0"])
	}
	if result.Metrics["phy1-ap0"] != 7 {
		t.Errorf("expected phy1-ap0=7, got %d", result.Metrics["phy1-ap0"])
	}
}

func TestRun_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	w, _ := New(server.URL, testRadios())
	result := w.Run(context.Background())

	if result.Success {
		t.Error("expected failure for 500 response")
	}
	if result.Err == nil {
		t.Error("expected non-nil error")
	}
}

func TestRun_ConnectionError(t *testing.T) {
	w, _ := New("http://127.0.0.1:1/metrics", testRadios(), WithTimeout(1*time.Second))
	result := w.Run(context.Background())

	if result.Success {
		t.Error("expected failure for connection error")
	}
}

func TestRun_NoMatchingRadios(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# just a comment\nnode_cpu_seconds_total 42\n"))
	}))
	defer server.Close()

	w, _ := New(server.URL, testRadios())
	result := w.Run(context.Background())

	if result.Success {
		t.Error("expected failure when no matching radios found")
	}
}

func TestRun_PartialRadios(t *testing.T) {
	// Only one radio present in metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`wifi_stations{ifname="phy0-ap0"} 5` + "\n"))
	}))
	defer server.Close()

	w, _ := New(server.URL, testRadios())
	result := w.Run(context.Background())

	if !result.Success {
		t.Fatalf("expected success with partial radios, got error: %v", result.Err)
	}
	if result.Metrics["phy0-ap0"] != 5 {
		t.Errorf("expected phy0-ap0=5, got %d", result.Metrics["phy0-ap0"])
	}
	if _, ok := result.Metrics["phy1-ap0"]; ok {
		t.Error("expected phy1-ap0 to be absent")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // slow server
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, _ := New(server.URL, testRadios(), WithTimeout(30*time.Second))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := w.Run(ctx)
	if result.Success {
		t.Error("expected cancelled check to fail")
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
	ifname, value, err := parseLine(`wifi_stations{ifname="phy1-ap0"} 7.0`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ifname != "phy1-ap0" {
		t.Errorf("expected ifname 'phy1-ap0', got %q", ifname)
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
	if values["phy0-ap0"] != 3 {
		t.Errorf("expected phy0-ap0=3, got %d", values["phy0-ap0"])
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
	r := strings.NewReader("# just comments\n# another comment\n")
	_, err := parseMetrics(r, testRadios())
	if err == nil {
		t.Error("expected error when no matching metrics found")
	}
}

// --- Factory tests ---

func TestFactory_WithTarget(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
		"radios": []any{"phy0-ap0", "phy1-ap0"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w := chk.(*WifiStations)
	if w.url != "http://ap1:9100/metrics" {
		t.Errorf("expected url 'http://ap1:9100/metrics', got %q", w.url)
	}
	if len(w.radios) != 2 {
		t.Errorf("expected 2 radios, got %d", len(w.radios))
	}
}

func TestFactory_WithURL(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
		"url":    "http://custom:8080/prom",
		"radios": []any{"phy0-ap0"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w := chk.(*WifiStations)
	if w.url != "http://custom:8080/prom" {
		t.Errorf("expected custom url, got %q", w.url)
	}
}

func TestFactory_WithTimeout(t *testing.T) {
	config := map[string]any{
		"target":  "ap1",
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

func TestFactory_MissingTarget(t *testing.T) {
	config := map[string]any{
		"radios": []any{"phy0-ap0"},
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing target")
	}
}

func TestFactory_MissingRadios(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing radios")
	}
}

func TestFactory_EmptyRadios(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
		"radios": []any{},
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty radios")
	}
}

func TestFactory_InvalidRadioType(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
		"radios": []any{123},
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string radio")
	}
}

func TestFactory_RadiosWrongType(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
		"radios": "not-a-list",
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-list radios")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	config := map[string]any{
		"target":  "ap1",
		"radios":  []any{"phy0-ap0"},
		"timeout": "nope",
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestFactory_WrongURLType(t *testing.T) {
	config := map[string]any{
		"target": "ap1",
		"radios": []any{"phy0-ap0"},
		"url":    123,
	}

	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string url")
	}
}

// --- Registry integration ---

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	err := reg.Register(TypeName, Factory)
	if err != nil {
		t.Fatalf("failed to register wifi_stations: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sampleMetrics)
	}))
	defer server.Close()

	chk, err := reg.Create("wifi_stations", map[string]any{
		"target": "unused",
		"url":    server.URL,
		"radios": []any{"phy0-ap0", "phy1-ap0"},
	})
	if err != nil {
		t.Fatalf("failed to create wifi_stations check: %v", err)
	}

	if chk.Type() != "wifi_stations" {
		t.Errorf("expected type 'wifi_stations', got %q", chk.Type())
	}

	desc := chk.Describe()
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics in descriptor, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].DSName != "radio0" {
		t.Errorf("expected DSName 'radio0', got %q", desc.Metrics[0].DSName)
	}

	result := chk.Run(context.Background())
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Err)
	}
	if result.Metrics["phy0-ap0"] != 3 {
		t.Errorf("expected phy0-ap0=3, got %d", result.Metrics["phy0-ap0"])
	}
	if result.Metrics["phy1-ap0"] != 7 {
		t.Errorf("expected phy1-ap0=7, got %d", result.Metrics["phy1-ap0"])
	}
}

// --- Dynamic descriptor test ---

func TestDescribe_DynamicPerConfig(t *testing.T) {
	// Two instances with different radio configs should have different descriptors
	w1, _ := New("http://a:9100/metrics", []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
	})
	w2, _ := New("http://b:9100/metrics", []radioConfig{
		{ifname: "phy0-ap0", resultKey: "phy0-ap0", dsName: "radio0", label: "phy0-ap0"},
		{ifname: "phy1-ap0", resultKey: "phy1-ap0", dsName: "radio1", label: "phy1-ap0"},
		{ifname: "phy2-ap0", resultKey: "phy2-ap0", dsName: "radio2", label: "phy2-ap0"},
	})

	if len(w1.Describe().Metrics) != 1 {
		t.Errorf("expected 1 metric for w1, got %d", len(w1.Describe().Metrics))
	}
	if len(w2.Describe().Metrics) != 3 {
		t.Errorf("expected 3 metrics for w2, got %d", len(w2.Describe().Metrics))
	}
}
