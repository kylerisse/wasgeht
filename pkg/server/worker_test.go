package server

import (
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
)

// pingMetrics is the standard ping metric definition used across tests.
var pingMetrics = []check.MetricDef{
	{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms", Scale: 1000},
}

func TestBuildFactoryConfig_InjectsTarget(t *testing.T) {
	cfg := map[string]any{"timeout": "5s"}
	result := buildFactoryConfig(cfg, "8.8.8.8")

	if result["target"] != "8.8.8.8" {
		t.Errorf("expected target '8.8.8.8', got %v", result["target"])
	}
	if result["timeout"] != "5s" {
		t.Errorf("expected timeout '5s', got %v", result["timeout"])
	}
}

func TestBuildFactoryConfig_DoesNotMutateOriginal(t *testing.T) {
	cfg := map[string]any{"timeout": "5s"}
	buildFactoryConfig(cfg, "8.8.8.8")

	if _, ok := cfg["target"]; ok {
		t.Error("original config should not be mutated")
	}
}

func TestBuildFactoryConfig_EmptyConfig(t *testing.T) {
	result := buildFactoryConfig(map[string]any{}, "localhost")

	if result["target"] != "localhost" {
		t.Errorf("expected target 'localhost', got %v", result["target"])
	}
	if len(result) != 1 {
		t.Errorf("expected 1 key, got %d", len(result))
	}
}

func TestBuildFactoryConfig_OverridesExistingTarget(t *testing.T) {
	cfg := map[string]any{"target": "should-be-overridden"}
	result := buildFactoryConfig(cfg, "8.8.8.8")

	if result["target"] != "8.8.8.8" {
		t.Errorf("expected injected target to win, got %v", result["target"])
	}
}

func TestRrdValuesFromResult_Success(t *testing.T) {
	result := check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 56780},
	}

	vals := rrdValuesFromResult(result, pingMetrics)
	if len(vals) != 1 || vals[0] != 56780 {
		t.Errorf("expected [56780], got %v", vals)
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
	if len(vals) != 0 {
		t.Errorf("expected empty slice for missing latency, got %v", vals)
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
			"tx_bytes": 5000,
		},
	}

	vals := rrdValuesFromResult(result, multiMetrics)
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}
}
