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
	status.SetLastUpdate(1700000000)

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected application/json, got %q", contentType)
	}

	var body map[string]HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	google, ok := body["google"]
	if !ok {
		t.Fatal("expected google in response")
	}

	pingCheck, ok := google.Checks["ping"]
	if !ok {
		t.Fatal("expected ping check in response")
	}
	if !pingCheck.Alive {
		t.Error("expected ping to be alive")
	}
	if pingCheck.Metrics["latency_us"] != 12345 {
		t.Errorf("expected latency_us=12345, got %d", pingCheck.Metrics["latency_us"])
	}
	if pingCheck.LastUpdate != 1700000000 {
		t.Errorf("expected lastupdate=1700000000, got %d", pingCheck.LastUpdate)
	}
}

func TestHandleAPI_IncludesHostStatus(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"uphost":   {Name: "uphost"},
			"downhost": {Name: "downhost"},
			"newhost":  {Name: "newhost"},
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
