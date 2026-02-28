package server

import (
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
)

// pingMetrics is the standard ping metric definition used across tests.
var pingMetrics = []check.MetricDef{
	{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms", Scale: 1000},
}

func TestCopyConfig_CopiesAllKeys(t *testing.T) {
	cfg := map[string]any{"timeout": "5s", "count": float64(3)}
	result := copyConfig(cfg)

	if result["timeout"] != "5s" {
		t.Errorf("expected timeout '5s', got %v", result["timeout"])
	}
	if result["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", result["count"])
	}
	if len(result) != 2 {
		t.Errorf("expected 2 keys, got %d", len(result))
	}
}

func TestCopyConfig_DoesNotMutateOriginal(t *testing.T) {
	cfg := map[string]any{"timeout": "5s"}
	result := copyConfig(cfg)
	result["injected"] = "value"

	if _, ok := cfg["injected"]; ok {
		t.Error("original config should not be mutated by changes to copy")
	}
}

func TestCopyConfig_EmptyConfig(t *testing.T) {
	result := copyConfig(map[string]any{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d keys", len(result))
	}
}

func TestCopyConfig_NoTargetInjected(t *testing.T) {
	cfg := map[string]any{"addresses": []any{"127.0.0.1"}, "label": "test"}
	result := copyConfig(cfg)

	if _, ok := result["target"]; ok {
		t.Error("copyConfig should not inject a 'target' key")
	}
}

func TestRrdValuesFromResult_Success(t *testing.T) {
	result := check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 56780},
	}

	vals := rrdValuesFromResult(result, pingMetrics)
	if len(vals) != 1 || vals[0] != "56780" {
		t.Errorf("expected [\"56780\"], got %v", vals)
	}
}

func TestRrdValuesFromResult_Failure(t *testing.T) {
	result := check.Result{Success: false}

	vals := rrdValuesFromResult(result, pingMetrics)
	if len(vals) != 0 {
		t.Errorf("expected empty slice, got %v", vals)
	}
}

func TestRrdValuesFromResult_NoLatencyMetric(t *testing.T) {
	result := check.Result{
		Success: true,
		Metrics: map[string]int64{"something_else": 420},
	}

	vals := rrdValuesFromResult(result, pingMetrics)
	if len(vals) != 1 || vals[0] != "U" {
		t.Errorf("expected [\"U\"] for missing latency, got %v", vals)
	}
}

func TestRrdValuesFromResult_MultipleMetrics(t *testing.T) {
	multiMetrics := []check.MetricDef{
		{ResultKey: "rx_bytes", DSName: "rx", Label: "received", Unit: "bytes"},
		{ResultKey: "tx_bytes", DSName: "tx", Label: "transmitted", Unit: "bytes"},
	}
	result := check.Result{
		Success: true,
		Metrics: map[string]int64{
			"rx_bytes": 10000,
			"tx_bytes": 2000,
		},
	}

	vals := rrdValuesFromResult(result, multiMetrics)
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}
	if vals[0] != "10000" {
		t.Errorf("expected rx_bytes=\"10000\", got %q", vals[0])
	}
	if vals[1] != "2000" {
		t.Errorf("expected tx_bytes=\"2000\", got %q", vals[1])
	}
}

func TestRrdValuesFromResult_PartialMetrics(t *testing.T) {
	multiMetrics := []check.MetricDef{
		{ResultKey: "rx_bytes", DSName: "rx", Label: "received", Unit: "bytes"},
		{ResultKey: "tx_bytes", DSName: "tx", Label: "transmitted", Unit: "bytes"},
	}
	result := check.Result{
		Success: false,
		Metrics: map[string]int64{
			"rx_bytes": 10000,
			// tx_bytes missing (target failed)
		},
	}

	vals := rrdValuesFromResult(result, multiMetrics)
	if len(vals) != 2 {
		t.Fatalf("expected 2 values for partial metrics, got %d", len(vals))
	}
	if vals[0] != "10000" {
		t.Errorf("expected rx_bytes=\"10000\", got %q", vals[0])
	}
	if vals[1] != "U" {
		t.Errorf("expected tx_bytes=\"U\" for failed target, got %q", vals[1])
	}
}

func TestRrdValuesFromResult_EmptyMetricDefs(t *testing.T) {
	result := check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 12340},
	}

	vals := rrdValuesFromResult(result, []check.MetricDef{})
	if len(vals) != 0 {
		t.Errorf("expected empty slice for empty metric defs, got %v", vals)
	}
}
