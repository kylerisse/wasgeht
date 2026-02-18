package server

import (
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/host"
)

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

func TestApplyPingResult_Success(t *testing.T) {
	h := &host.Host{Name: "test"}
	result := check.Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 1234.0},
	}

	applyPingResult(h, "test", result)

	if !h.Alive {
		t.Error("expected host to be alive")
	}
	expected := time.Duration(1234) * time.Microsecond
	if h.Latency != expected {
		t.Errorf("expected latency %v, got %v", expected, h.Latency)
	}
}

func TestApplyPingResult_Failure(t *testing.T) {
	h := &host.Host{Name: "test", Alive: true, Latency: 100 * time.Microsecond}
	result := check.Result{
		Success: false,
	}

	applyPingResult(h, "test", result)

	if h.Alive {
		t.Error("expected host to be not alive")
	}
}

func TestApplyPingResult_SuccessWithoutLatencyMetric(t *testing.T) {
	h := &host.Host{Name: "test", Latency: 999 * time.Microsecond}
	result := check.Result{
		Success: true,
		Metrics: map[string]float64{},
	}

	applyPingResult(h, "test", result)

	if !h.Alive {
		t.Error("expected host to be alive")
	}
	// Latency should be unchanged since metric is absent
	if h.Latency != 999*time.Microsecond {
		t.Errorf("expected latency unchanged, got %v", h.Latency)
	}
}

func TestRrdValuesFromResult_Success(t *testing.T) {
	result := check.Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 5678.0},
	}

	vals := rrdValuesFromResult(result)
	if len(vals) != 1 || vals[0] != 5678.0 {
		t.Errorf("expected [5678.0], got %v", vals)
	}
}

func TestRrdValuesFromResult_Failure(t *testing.T) {
	result := check.Result{Success: false}

	vals := rrdValuesFromResult(result)
	if len(vals) != 0 {
		t.Errorf("expected empty slice, got %v", vals)
	}
}

func TestRrdValuesFromResult_NoLatencyMetric(t *testing.T) {
	result := check.Result{
		Success: true,
		Metrics: map[string]float64{"something_else": 42.0},
	}

	vals := rrdValuesFromResult(result)
	if len(vals) != 0 {
		t.Errorf("expected empty slice for missing latency, got %v", vals)
	}
}
