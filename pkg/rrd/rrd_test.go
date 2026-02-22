package rrd

import (
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestExpandTimeLength(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"15m", "fifteen minutes"},
		{"1h", "one hour"},
		{"4h", "four hours"},
		{"8h", "eight hours"},
		{"1d", "one day"},
		{"4d", "four days"},
		{"1w", "week"},
		{"31d", "month"},
		{"93d", "quarter"},
		{"1y", "year"},
		{"2y", "two years"},
		{"5y", "five years"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandTimeLength(tt.input)
			if got != tt.want {
				t.Errorf("expandTimeLength(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNeedsScaling(t *testing.T) {
	tests := []struct {
		name  string
		scale int
		want  bool
	}{
		{"zero means no scaling", 0, false},
		{"one means no scaling", 1, false},
		{"1000 needs scaling", 1000, true},
		{"2 needs scaling", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := check.MetricDef{Scale: tt.scale}
			if got := needsScaling(m); got != tt.want {
				t.Errorf("needsScaling() with scale=%d: got %v, want %v", tt.scale, got, tt.want)
			}
		})
	}
}

func TestDisplayVarName(t *testing.T) {
	tests := []struct {
		name   string
		dsName string
		unit   string
		scale  int
		want   string
	}{
		{"with scaling", "latency", "ms", 1000, "latency_ms"},
		{"without scaling zero", "latency", "ms", 0, "latency_raw"},
		{"without scaling one", "latency", "ms", 1, "latency_raw"},
		{"different ds name", "rtt", "us", 100, "rtt_us"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := check.MetricDef{DSName: tt.dsName, Unit: tt.unit, Scale: tt.scale}
			if got := displayVarName(m); got != tt.want {
				t.Errorf("displayVarName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStackColors_HasEntries(t *testing.T) {
	if len(stackColors) == 0 {
		t.Error("stackColors should have at least one entry")
	}
}

func TestStackColors_CyclesCorrectly(t *testing.T) {
	// Ensure color indexing wraps around
	for i := 0; i < len(stackColors)*2; i++ {
		color := stackColors[i%len(stackColors)]
		if color == "" {
			t.Errorf("stackColors[%d %% %d] is empty", i, len(stackColors))
		}
	}
}
