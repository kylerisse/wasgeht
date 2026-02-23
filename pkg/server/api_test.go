package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/host"
)

func TestHandleAPI_BasicResponse(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"google": {Name: "google"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("google", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 12345},
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	google, ok := body["google"]
	if !ok {
		t.Fatal("expected google in response")
	}

	if google.Status != HostStatusUp {
		t.Errorf("expected status up, got %q", google.Status)
	}

	ping, ok := google.Checks["ping"]
	if !ok {
		t.Fatal("expected ping check in response")
	}
	if !ping.Alive {
		t.Error("expected ping to be alive")
	}
	if ping.Metrics["latency_us"] != 12345 {
		t.Errorf("expected latency_us=12345, got %d", ping.Metrics["latency_us"])
	}
}

func TestHandleAPI_EmptyHosts(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	var body map[string]HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body) != 0 {
		t.Errorf("expected empty response, got %d hosts", len(body))
	}
}

func TestHandleAPI_MultipleChecks(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	pingStatus := s.getOrCreateStatus("ap1", "ping")
	pingStatus.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 500},
	})

	wifiStatus := s.getOrCreateStatus("ap1", "wifi_stations")
	wifiStatus.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"phy0-ap0": 3, "phy1-ap0": 7},
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	var body map[string]HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	ap1, ok := body["ap1"]
	if !ok {
		t.Fatal("expected ap1 in response")
	}
	if ap1.Status != HostStatusUp {
		t.Errorf("expected status up, got %q", ap1.Status)
	}
	if _, ok := ap1.Checks["ping"]; !ok {
		t.Error("expected ping check")
	}
	if _, ok := ap1.Checks["wifi_stations"]; !ok {
		t.Error("expected wifi_stations check")
	}
}

func TestHandleAPI_UnknownStatus(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
			"ap1":    {Name: "ap1"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	var body map[string]HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for name, host := range body {
		if host.Status != HostStatusUnknown {
			t.Errorf("host %q: expected status unknown, got %q", name, host.Status)
		}
	}
}
