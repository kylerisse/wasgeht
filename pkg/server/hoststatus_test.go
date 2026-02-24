package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestComputeHostStatus(t *testing.T) {
	now := time.Now()
	fresh := now.Add(-1 * time.Minute).Unix()
	stale := now.Add(-10 * time.Minute).Unix()

	tests := []struct {
		name      string
		snapshots map[string]check.StatusSnapshot
		want      HostStatus
	}{
		{
			name:      "single check fresh and up",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: true, LastUpdate: fresh}},
			want:      HostStatusUp,
		},
		{
			name:      "single check fresh and down",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: false, LastUpdate: fresh}},
			want:      HostStatusDown,
		},
		{
			name:      "single check stale and alive",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: true, LastUpdate: stale}},
			want:      HostStatusDown,
		},
		{
			name:      "single check stale and down",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: false, LastUpdate: stale}},
			want:      HostStatusDown,
		},
		{
			name:      "single check never reported",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: false, LastUpdate: 0}},
			want:      HostStatusUnknown,
		},
		{
			name: "three checks all fresh and up",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: true, LastUpdate: fresh},
				"tcp":  {Alive: true, LastUpdate: fresh},
			},
			want: HostStatusUp,
		},
		{
			name: "three checks all fresh and down",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: fresh},
				"http": {Alive: false, LastUpdate: fresh},
				"tcp":  {Alive: false, LastUpdate: fresh},
			},
			want: HostStatusDown,
		},
		{
			name: "three checks mixed - one down",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: false, LastUpdate: fresh},
				"tcp":  {Alive: true, LastUpdate: fresh},
			},
			want: HostStatusDegraded,
		},
		{
			name: "three checks mixed - one stale",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: true, LastUpdate: stale},
				"tcp":  {Alive: true, LastUpdate: fresh},
			},
			want: HostStatusDegraded,
		},
		{
			name: "three checks mixed - one never reported",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: true, LastUpdate: fresh},
				"tcp":  {Alive: false, LastUpdate: 0},
			},
			want: HostStatusDegraded,
		},
		{
			name: "three checks all never reported",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: 0},
				"http": {Alive: false, LastUpdate: 0},
				"tcp":  {Alive: false, LastUpdate: 0},
			},
			want: HostStatusUnknown,
		},
		{
			name: "two checks - one never reported one down with history",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: 0},
				"http": {Alive: false, LastUpdate: stale},
			},
			want: HostStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeHostStatus(tt.snapshots, now)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostStatus_StringValues(t *testing.T) {
	tests := []struct {
		status HostStatus
		want   string
	}{
		{HostStatusUnknown, "unknown"},
		{HostStatusUp, "up"},
		{HostStatusDegraded, "degraded"},
		{HostStatusDown, "down"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("HostStatus %v: got %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestHostStatus_JSONSerialization(t *testing.T) {
	resp := HostAPIResponse{
		Status: HostStatusUp,
		Checks: map[string]CheckStatusResponse{
			"ping": {Alive: true, LastUpdate: 1700000000},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	status, ok := decoded["status"]
	if !ok {
		t.Fatal("expected 'status' field in JSON")
	}
	if status != "up" {
		t.Errorf("expected status 'up', got %q", status)
	}
}

func TestHostStatus_JSONRoundTrip(t *testing.T) {
	statuses := []HostStatus{HostStatusUnknown, HostStatusUp, HostStatusDegraded, HostStatusDown}

	for _, s := range statuses {
		resp := HostAPIResponse{Status: s, Checks: map[string]CheckStatusResponse{}}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal %q failed: %v", s, err)
		}

		var decoded HostAPIResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal %q failed: %v", s, err)
		}

		if decoded.Status != s {
			t.Errorf("round-trip: got %q, want %q", decoded.Status, s)
		}
	}
}

func TestCheckStatusResponse_OmitsEmptyMetrics(t *testing.T) {
	resp := CheckStatusResponse{
		Alive:      true,
		LastUpdate: 1700000000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := decoded["metrics"]; ok {
		t.Error("expected metrics to be omitted when nil")
	}
}

func TestCheckStatusResponse_IncludesMetrics(t *testing.T) {
	resp := CheckStatusResponse{
		Alive:      true,
		Metrics:    map[string]int64{"latency_us": 12345},
		LastUpdate: 1700000000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded CheckStatusResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Metrics["latency_us"] != 12345 {
		t.Errorf("expected latency_us=12345, got %v", decoded.Metrics["latency_us"])
	}
}
