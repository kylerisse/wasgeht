package check

import (
	"testing"
)

func TestDescriptor_ZeroValue(t *testing.T) {
	var d Descriptor
	if d.Metrics != nil {
		t.Error("zero Descriptor should have nil Metrics")
	}
	if d.GraphStyle != "" {
		t.Error("zero Descriptor should have empty GraphStyle")
	}
}

func TestDescriptor_WithMetrics(t *testing.T) {
	d := Descriptor{
		Metrics: []MetricDef{
			{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms", Scale: 1000},
			{ResultKey: "rx_bytes", DSName: "rx", Label: "received", Unit: "bytes"},
		},
	}
	if len(d.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(d.Metrics))
	}
	if d.Metrics[0].ResultKey != "latency_us" {
		t.Errorf("expected first ResultKey 'latency_us', got %q", d.Metrics[0].ResultKey)
	}
	if d.Metrics[0].Scale != 1000 {
		t.Errorf("expected first Scale 1000, got %d", d.Metrics[0].Scale)
	}
	if d.Metrics[1].DSName != "rx" {
		t.Errorf("expected second DSName 'rx', got %q", d.Metrics[1].DSName)
	}
}

func TestDescriptor_ScaleZeroMeansNoScaling(t *testing.T) {
	d := MetricDef{ResultKey: "rtt_ms", DSName: "rtt", Label: "rtt", Unit: "ms", Scale: 0}
	if d.Scale != 0 {
		t.Errorf("expected Scale 0, got %d", d.Scale)
	}
}

func TestDescriptor_ScaleOneMeansNoScaling(t *testing.T) {
	d := MetricDef{ResultKey: "rtt_ms", DSName: "rtt", Label: "rtt", Unit: "ms", Scale: 1}
	if d.Scale != 1 {
		t.Errorf("expected Scale 1, got %d", d.Scale)
	}
}

func TestDescriptor_GraphStyleStack(t *testing.T) {
	d := Descriptor{
		GraphStyle: GraphStyleStack,
		Metrics:    []MetricDef{{ResultKey: "a", DSName: "a", Label: "a", Unit: "x"}},
	}
	if d.GraphStyle != "stack" {
		t.Errorf("expected GraphStyle 'stack', got %q", d.GraphStyle)
	}
}

func TestDescriptor_GraphStyleLine(t *testing.T) {
	d := Descriptor{
		GraphStyle: GraphStyleLine,
		Metrics:    []MetricDef{{ResultKey: "a", DSName: "a", Label: "a", Unit: "x"}},
	}
	if d.GraphStyle != "line" {
		t.Errorf("expected GraphStyle 'line', got %q", d.GraphStyle)
	}
}

func TestDescriptor_EmptyGraphStyleDefaultsToStack(t *testing.T) {
	// Empty string should be treated as stack by the graph layer
	d := Descriptor{
		Metrics: []MetricDef{{ResultKey: "a", DSName: "a", Label: "a", Unit: "x"}},
	}
	if d.GraphStyle != "" {
		t.Errorf("expected empty GraphStyle, got %q", d.GraphStyle)
	}
}

func TestGraphStyleConstants(t *testing.T) {
	if GraphStyleStack != "stack" {
		t.Errorf("expected GraphStyleStack='stack', got %q", GraphStyleStack)
	}
	if GraphStyleLine != "line" {
		t.Errorf("expected GraphStyleLine='line', got %q", GraphStyleLine)
	}
}
