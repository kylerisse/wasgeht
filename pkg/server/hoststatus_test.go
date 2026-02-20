package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestComputeHostStatus_NoSnapshots(t *testing.T) {
	got := computeHostStatus(nil, time.Now())
	if got != HostStatusUnknown {
		t.Errorf("expected unknown, got %q", got)
	}

	got = computeHostStatus(map[string]check.StatusSnapshot{}, time.Now())
	if got != HostStatusUnknown {
		t.Errorf("expected unknown for empty map, got %q", got)
	}
}

func TestComputeHostStatus_AllUp(t *testing.T) {
	now := time.Now()
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: true, LastUpdate: now.Unix()},
		"http": {Alive: true, LastUpdate: now.Add(-2 * time.Minute).Unix()},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusUp {
		t.Errorf("expected up, got %q", got)
	}
}

func TestComputeHostStatus_AllDown(t *testing.T) {
	now := time.Now()
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: false, LastUpdate: now.Unix()},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusDown {
		t.Errorf("expected down, got %q", got)
	}
}

func TestComputeHostStatus_Degraded_MixedAlive(t *testing.T) {
	now := time.Now()
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: true, LastUpdate: now.Unix()},
		"http": {Alive: false, LastUpdate: now.Unix()},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusDegraded {
		t.Errorf("expected degraded, got %q", got)
	}
}

func TestComputeHostStatus_Degraded_OneStale(t *testing.T) {
	now := time.Now()
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: true, LastUpdate: now.Unix()},
		"http": {Alive: true, LastUpdate: now.Add(-10 * time.Minute).Unix()},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusDegraded {
		t.Errorf("expected degraded when one check is stale, got %q", got)
	}
}

func TestComputeHostStatus_Unknown_AllStaleNoResults(t *testing.T) {
	now := time.Now()
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: false, LastUpdate: 0},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusUnknown {
		t.Errorf("expected unknown when no results ever recorded, got %q", got)
	}
}

func TestComputeHostStatus_Down_StaleButHadResults(t *testing.T) {
	now := time.Now()
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: false, LastUpdate: now.Add(-10 * time.Minute).Unix()},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusDown {
		t.Errorf("expected down when stale but had previous results, got %q", got)
	}
}

func TestComputeHostStatus_BoundaryFreshness(t *testing.T) {
	// Use a fixed time to avoid sub-second truncation issues with Unix()
	now := time.Unix(1700000300, 0) // arbitrary fixed point
	cutoff := now.Add(-stalenessWindow).Unix()

	// Exactly at the cutoff should be stale (cutoff uses > not >=)
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: true, LastUpdate: cutoff},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusDown {
		t.Errorf("expected down at exact cutoff (%d), got %q", cutoff, got)
	}

	// One second after the cutoff should be fresh
	snaps["ping"] = check.StatusSnapshot{Alive: true, LastUpdate: cutoff + 1}
	got = computeHostStatus(snaps, now)
	if got != HostStatusUp {
		t.Errorf("expected up one second after cutoff (%d), got %q", cutoff+1, got)
	}

	// One second before the cutoff should be stale
	snaps["ping"] = check.StatusSnapshot{Alive: true, LastUpdate: cutoff - 1}
	got = computeHostStatus(snaps, now)
	if got != HostStatusDown {
		t.Errorf("expected down one second before cutoff (%d), got %q", cutoff-1, got)
	}
}

func TestComputeHostStatus_AliveButZeroLastUpdate(t *testing.T) {
	now := time.Now()
	// Alive but no RRD update yet - should be treated as unknown
	snaps := map[string]check.StatusSnapshot{
		"ping": {Alive: true, LastUpdate: 0},
	}
	got := computeHostStatus(snaps, now)
	if got != HostStatusUnknown {
		t.Errorf("expected unknown when alive but no last update, got %q", got)
	}
}

func TestComputeHostStatus_TableDriven(t *testing.T) {
	now := time.Now()
	fresh := now.Unix()
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
	// Verify the string values match what the UI expects
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
	// Verify HostStatus serializes correctly in API responses
	resp := HostAPIResponse{
		Address: "8.8.8.8",
		Status:  HostStatusUp,
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

func TestHostAPIResponse_OmitsEmptyAddress(t *testing.T) {
	resp := HostAPIResponse{
		Status: HostStatusUnknown,
		Checks: map[string]CheckStatusResponse{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := decoded["address"]; ok {
		t.Error("expected address to be omitted when empty")
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
