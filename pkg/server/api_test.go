package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	google, ok := body.Hosts["google"]
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

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for name, host := range body.Hosts {
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

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Hosts) != 0 {
		t.Errorf("expected empty hosts, got %d", len(body.Hosts))
	}
}

func TestHandleAPI_Envelope(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	before := time.Now().Unix()

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	after := time.Now().Unix()

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.GeneratedAt < before || body.GeneratedAt > after {
		t.Errorf("generated_at %d not between %d and %d", body.GeneratedAt, before, after)
	}

	if _, ok := body.Hosts["router"]; !ok {
		t.Error("expected router in hosts")
	}
}

func TestHandleHostAPI_Found(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1", Tags: map[string]string{"category": "ap"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("ap1", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 5000},
	})
	status.SetLastUpdate(time.Now().Unix())

	req := httptest.NewRequest("GET", "/api/hosts/ap1", nil)
	req.SetPathValue("hostname", "ap1")
	w := httptest.NewRecorder()

	s.handleHostAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Status != HostStatusUp {
		t.Errorf("expected status up, got %q", body.Status)
	}
	if body.Tags["category"] != "ap" {
		t.Errorf("expected category=ap, got %q", body.Tags["category"])
	}
	if _, ok := body.Checks["ping"]; !ok {
		t.Error("expected ping check in response")
	}
}

func TestHandleHostAPI_NotFound(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api/hosts/nobody", nil)
	req.SetPathValue("hostname", "nobody")
	w := httptest.NewRecorder()

	s.handleHostAPI(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Result().StatusCode)
	}
}

func TestHandleAPI_TagsPassthrough(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1", Tags: map[string]string{"category": "ap", "building": "expo"}},
			"router": {Name: "router"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	ap1 := body.Hosts["ap1"]
	if ap1.Tags["category"] != "ap" {
		t.Errorf("expected category=ap, got %q", ap1.Tags["category"])
	}
	if ap1.Tags["building"] != "expo" {
		t.Errorf("expected building=expo, got %q", ap1.Tags["building"])
	}

	router := body.Hosts["router"]
	if router.Tags != nil {
		t.Error("expected nil tags for router")
	}
}
