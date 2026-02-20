package check

import (
	"testing"
	"time"
)

func TestResult_ZeroValue(t *testing.T) {
	var r Result
	if r.Success {
		t.Error("zero Result should not be successful")
	}
	if r.Err != nil {
		t.Error("zero Result should have nil error")
	}
	if r.Metrics != nil {
		t.Error("zero Result should have nil metrics")
	}
	if !r.Timestamp.IsZero() {
		t.Error("zero Result should have zero timestamp")
	}
}

func TestResult_WithMetrics(t *testing.T) {
	r := Result{
		Timestamp: time.Now(),
		Success:   true,
		Metrics:   map[string]int64{"latency_us": 12345},
	}
	if !r.Success {
		t.Error("expected success")
	}
	if v, ok := r.Metrics["latency_us"]; !ok || v != 12345 {
		t.Errorf("expected latency_us=12345, got %v", v)
	}
}
