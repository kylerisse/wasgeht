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
		// unconfigured
		{
			name:      "no checks",
			snapshots: map[string]check.StatusSnapshot{},
			want:      HostStatusUnconfigured,
		},
		// pending
		{
			name:      "single check never run",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: false, LastUpdate: 0}},
			want:      HostStatusPending,
		},
		{
			name: "all checks never run",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: 0},
				"http": {Alive: false, LastUpdate: 0},
				"tcp":  {Alive: false, LastUpdate: 0},
			},
			want: HostStatusPending,
		},
		// up
		{
			name:      "single check fresh and up",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: true, LastUpdate: fresh}},
			want:      HostStatusUp,
		},
		{
			name: "all checks fresh and up",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: true, LastUpdate: fresh},
				"tcp":  {Alive: true, LastUpdate: fresh},
			},
			want: HostStatusUp,
		},
		// down
		{
			name:      "single check fresh and down",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: false, LastUpdate: fresh}},
			want:      HostStatusDown,
		},
		{
			name: "all checks fresh and down",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: fresh},
				"http": {Alive: false, LastUpdate: fresh},
				"tcp":  {Alive: false, LastUpdate: fresh},
			},
			want: HostStatusDown,
		},
		// degraded
		{
			name: "mixed fresh up and fresh down",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: false, LastUpdate: fresh},
				"tcp":  {Alive: true, LastUpdate: fresh},
			},
			want: HostStatusDegraded,
		},
		{
			name: "fresh up mixed with stale",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: true, LastUpdate: stale},
				"tcp":  {Alive: true, LastUpdate: fresh},
			},
			want: HostStatusDegraded,
		},
		{
			name: "fresh up mixed with never run",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: fresh},
				"http": {Alive: true, LastUpdate: fresh},
				"tcp":  {Alive: false, LastUpdate: 0},
			},
			want: HostStatusDegraded,
		},
		// stale
		{
			name:      "single check stale",
			snapshots: map[string]check.StatusSnapshot{"ping": {Alive: true, LastUpdate: stale}},
			want:      HostStatusStale,
		},
		{
			name: "all checks stale",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: true, LastUpdate: stale},
				"http": {Alive: false, LastUpdate: stale},
			},
			want: HostStatusStale,
		},
		{
			name: "mix of stale and never run",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: 0},
				"http": {Alive: false, LastUpdate: stale},
			},
			want: HostStatusStale,
		},
		{
			name: "all fresh down mixed with stale",
			snapshots: map[string]check.StatusSnapshot{
				"ping": {Alive: false, LastUpdate: fresh},
				"http": {Alive: false, LastUpdate: stale},
			},
			want: HostStatusStale,
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
		{HostStatusUnconfigured, "unconfigured"},
		{HostStatusPending, "pending"},
		{HostStatusStale, "stale"},
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
	statuses := []HostStatus{
		HostStatusUnconfigured,
		HostStatusPending,
		HostStatusStale,
		HostStatusUp,
		HostStatusDegraded,
		HostStatusDown,
	}

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
