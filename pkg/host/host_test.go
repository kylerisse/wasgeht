package host

import (
	"encoding/json"
	"testing"
)

func TestApplyDefaults_NilChecks(t *testing.T) {
	h := Host{Name: "test"}
	h.ApplyDefaults()

	if len(h.Checks) != 1 {
		t.Fatalf("expected 1 default check, got %d", len(h.Checks))
	}
	if _, ok := h.Checks["ping"]; !ok {
		t.Error("expected default ping check")
	}
}

func TestApplyDefaults_EmptyChecks(t *testing.T) {
	h := Host{Name: "test", Checks: map[string]map[string]any{}}
	h.ApplyDefaults()

	if _, ok := h.Checks["ping"]; !ok {
		t.Error("expected default ping check for empty checks map")
	}
}

func TestApplyDefaults_ExistingChecks(t *testing.T) {
	h := Host{
		Name: "test",
		Checks: map[string]map[string]any{
			"http": {"path": "/health"},
		},
	}
	h.ApplyDefaults()

	if len(h.Checks) != 1 {
		t.Fatalf("expected 1 check (http), got %d", len(h.Checks))
	}
	if _, ok := h.Checks["http"]; !ok {
		t.Error("expected http check to be preserved")
	}
	if _, ok := h.Checks["ping"]; ok {
		t.Error("ping should not be injected when checks are explicitly set")
	}
}

func TestApplyDefaults_DoesNotMutateDefault(t *testing.T) {
	h1 := Host{Name: "h1"}
	h1.ApplyDefaults()
	h1.Checks["ping"]["timeout"] = "5s"

	h2 := Host{Name: "h2"}
	h2.ApplyDefaults()

	if _, ok := h2.Checks["ping"]["timeout"]; ok {
		t.Error("mutating one host's defaults should not affect another")
	}
}

func TestEnabledChecks_AllEnabled(t *testing.T) {
	h := Host{
		Checks: map[string]map[string]any{
			"ping": {},
			"http": {"path": "/health"},
		},
	}

	enabled := h.EnabledChecks()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled checks, got %d", len(enabled))
	}
}

func TestEnabledChecks_OneDisabled(t *testing.T) {
	h := Host{
		Checks: map[string]map[string]any{
			"ping": {"enabled": false},
			"http": {"path": "/health"},
		},
	}

	enabled := h.EnabledChecks()
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled check, got %d", len(enabled))
	}
	if _, ok := enabled["http"]; !ok {
		t.Error("expected http to be enabled")
	}
	if _, ok := enabled["ping"]; ok {
		t.Error("expected ping to be disabled")
	}
}

func TestEnabledChecks_ExplicitlyEnabled(t *testing.T) {
	h := Host{
		Checks: map[string]map[string]any{
			"ping": {"enabled": true},
		},
	}

	enabled := h.EnabledChecks()
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled check, got %d", len(enabled))
	}
}

func TestEnabledChecks_AllDisabled(t *testing.T) {
	h := Host{
		Checks: map[string]map[string]any{
			"ping": {"enabled": false},
			"http": {"enabled": false},
		},
	}

	enabled := h.EnabledChecks()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled checks, got %d", len(enabled))
	}
}

func TestJSON_WithChecks(t *testing.T) {
	input := `{
		"address": "8.8.8.8",
		"checks": {
			"ping": {"timeout": "5s"},
			"http": {"path": "/health"}
		}
	}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if h.Address != "8.8.8.8" {
		t.Errorf("expected address 8.8.8.8, got %q", h.Address)
	}
	if len(h.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(h.Checks))
	}
	if h.Checks["ping"]["timeout"] != "5s" {
		t.Errorf("expected ping timeout '5s', got %v", h.Checks["ping"]["timeout"])
	}
	if h.Checks["http"]["path"] != "/health" {
		t.Errorf("expected http path '/health', got %v", h.Checks["http"]["path"])
	}
}

func TestJSON_WithoutChecks(t *testing.T) {
	input := `{"address": "1.1.1.1"}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if h.Checks != nil {
		t.Error("expected nil checks when not in JSON")
	}

	h.ApplyDefaults()
	if _, ok := h.Checks["ping"]; !ok {
		t.Error("expected default ping after ApplyDefaults")
	}
}

func TestJSON_EmptyHost(t *testing.T) {
	input := `{}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	h.ApplyDefaults()
	if h.Address != "" {
		t.Errorf("expected empty address, got %q", h.Address)
	}
	if _, ok := h.Checks["ping"]; !ok {
		t.Error("expected default ping after ApplyDefaults")
	}
}

func TestJSON_DisabledCheck(t *testing.T) {
	input := `{
		"address": "10.0.0.1",
		"checks": {
			"ping": {"enabled": false}
		}
	}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	enabled := h.EnabledChecks()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled checks, got %d", len(enabled))
	}
}

func TestJSON_BackwardCompatible(t *testing.T) {
	// This mirrors the current sample-hosts.json format
	input := `{
		"router": {},
		"google": {"address": "8.8.8.8"},
		"localhostv6": {"address": "::1"}
	}`

	var hosts map[string]Host
	if err := json.Unmarshal([]byte(input), &hosts); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for name, h := range hosts {
		h.Name = name
		h.ApplyDefaults()

		if _, ok := h.Checks["ping"]; !ok {
			t.Errorf("host %q: expected default ping check", name)
		}
	}

	if hosts["google"].Address != "8.8.8.8" {
		t.Errorf("expected google address 8.8.8.8, got %q", hosts["google"].Address)
	}
	if hosts["router"].Address != "" {
		t.Errorf("expected empty router address, got %q", hosts["router"].Address)
	}
}
