package check

import (
	"testing"
)

func TestDescriptor_ZeroValue(t *testing.T) {
	var d Descriptor
	if d.Metrics != nil {
		t.Error("zero Descriptor should have nil Metrics")
	}
}

func TestDescriptor_WithMetrics(t *testing.T) {
	d := Descriptor{
		Metrics: []MetricDef{
			{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms"},
			{ResultKey: "rx_bytes", DSName: "rx", Label: "received", Unit: "bytes"},
		},
	}
	if len(d.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(d.Metrics))
	}
	if d.Metrics[0].ResultKey != "latency_us" {
		t.Errorf("expected first ResultKey 'latency_us', got %q", d.Metrics[0].ResultKey)
	}
	if d.Metrics[1].DSName != "rx" {
		t.Errorf("expected second DSName 'rx', got %q", d.Metrics[1].DSName)
	}
}
