package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/host"
)

func TestHandlePrometheus_BasicOutput(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"google": {Name: "google"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	// Set up a ping status
	status := s.getOrCreateStatus("google", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]*int64{"latency_us": p64(12345)},
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	s.handlePrometheus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("expected text/plain, got %q", contentType)
	}

	body := w.Body.String()

	// Check HELP and TYPE lines
	if !strings.Contains(body, "# HELP check_alive") {
		t.Error("expected HELP check_alive header")
	}
	if !strings.Contains(body, "# TYPE check_alive gauge") {
		t.Error("expected TYPE check_alive gauge header")
	}
	if !strings.Contains(body, "# HELP check_metric") {
		t.Error("expected HELP check_metric header")
	}

	// Check alive metric (no address label)
	if !strings.Contains(body, `check_alive{host="google", check="ping"} 1`) {
		t.Errorf("expected check_alive line for google ping, got:\n%s", body)
	}

	// Check metric value (no address label)
	if !strings.Contains(body, `check_metric{host="google", check="ping", metric="latency_us"} 12345`) {
		t.Errorf("expected check_metric line for latency_us, got:\n%s", body)
	}
}

func TestHandlePrometheus_DownHost(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"badhost": {Name: "badhost"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("badhost", "ping")
	status.SetResult(check.Result{Success: false})

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	s.handlePrometheus(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `check_alive{host="badhost", check="ping"} 0`) {
		t.Errorf("expected check_alive=0 for down host, got:\n%s", body)
	}
}

func TestHandlePrometheus_NoStatuses(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"newhost": {Name: "newhost"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	s.handlePrometheus(w, req)

	body := w.Body.String()

	// Should still have headers but no host-specific lines
	if !strings.Contains(body, "# HELP check_alive") {
		t.Error("expected HELP header even with no statuses")
	}
	// Should not contain any host metrics
	if strings.Contains(body, "newhost") {
		t.Error("expected no metrics for host with no statuses")
	}
}

func TestHandlePrometheus_MultipleChecks(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"multi": {Name: "multi"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	pingStatus := s.getOrCreateStatus("multi", "ping")
	pingStatus.SetResult(check.Result{
		Success: true,
		Metrics: map[string]*int64{"latency_us": p64(500)},
	})

	httpStatus := s.getOrCreateStatus("multi", "http")
	httpStatus.SetResult(check.Result{
		Success: true,
		Metrics: map[string]*int64{"response_ms": p64(42)},
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	s.handlePrometheus(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `check="ping"`) {
		t.Error("expected ping check in output")
	}
	if !strings.Contains(body, `check="http"`) {
		t.Error("expected http check in output")
	}
	if !strings.Contains(body, `metric="latency_us"`) {
		t.Error("expected latency_us metric")
	}
	if !strings.Contains(body, `metric="response_ms"`) {
		t.Error("expected response_ms metric")
	}
}
