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
	v := int64(12345)
	r := Result{
		Timestamp: time.Now(),
		Success:   true,
		Metrics:   map[string]*int64{"latency_us": &v},
	}
	if !r.Success {
		t.Error("expected success")
	}
	p, ok := r.Metrics["latency_us"]
	if !ok || p == nil || *p != 12345 {
		t.Errorf("expected latency_us=12345, got %v", p)
	}
}
