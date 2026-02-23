package ping

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

// --- New() tests ---

func TestNew_SingleAddress(t *testing.T) {
	p, err := New([]string{"127.0.0.1"}, "loopback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.addresses) != 1 {
		t.Fatalf("expected 1 address, got %d", len(p.addresses))
	}
	if p.addresses[0].address != "127.0.0.1" {
		t.Errorf("expected address '127.0.0.1', got %q", p.addresses[0].address)
	}
	if p.addresses[0].dsName != "addr0" {
		t.Errorf("expected dsName 'addr0', got %q", p.addresses[0].dsName)
	}
	if p.label != "loopback" {
		t.Errorf("expected label 'loopback', got %q", p.label)
	}
	if p.timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %v", p.timeout)
	}
	if p.count != DefaultCount {
		t.Errorf("expected default count, got %d", p.count)
	}
}

func TestNew_MultipleAddresses(t *testing.T) {
	p, err := New([]string{"1.1.1.1", "8.8.8.8"}, "dns servers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(p.addresses))
	}
	if p.addresses[0].dsName != "addr0" {
		t.Errorf("expected dsName 'addr0', got %q", p.addresses[0].dsName)
	}
	if p.addresses[1].dsName != "addr1" {
		t.Errorf("expected dsName 'addr1', got %q", p.addresses[1].dsName)
	}
}

func TestNew_EmptyAddresses(t *testing.T) {
	_, err := New([]string{}, "test")
	if err == nil {
		t.Error("expected error for empty addresses")
	}
}

func TestNew_EmptyAddressInList(t *testing.T) {
	_, err := New([]string{"1.1.1.1", ""}, "test")
	if err == nil {
		t.Error("expected error for empty address in list")
	}
}

func TestNew_EmptyLabel(t *testing.T) {
	_, err := New([]string{"127.0.0.1"}, "")
	if err == nil {
		t.Error("expected error for empty label")
	}
}

func TestNew_WithOptions(t *testing.T) {
	p, err := New([]string{"127.0.0.1"}, "test",
		WithTimeout(5*time.Second),
		WithCount(3),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", p.timeout)
	}
	if p.count != 3 {
		t.Errorf("expected count 3, got %d", p.count)
	}
}

func TestWithTimeout_Invalid(t *testing.T) {
	_, err := New([]string{"127.0.0.1"}, "test", WithTimeout(-1*time.Second))
	if err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestWithCount_Invalid(t *testing.T) {
	_, err := New([]string{"127.0.0.1"}, "test", WithCount(0))
	if err == nil {
		t.Error("expected error for count=0")
	}
}

// --- Type / Describe tests ---

func TestType(t *testing.T) {
	p, _ := New([]string{"127.0.0.1"}, "test")
	if p.Type() != "ping" {
		t.Errorf("expected type 'ping', got %q", p.Type())
	}
}

func TestDescribe_SingleAddress(t *testing.T) {
	p, _ := New([]string{"127.0.0.1"}, "loopback ping")
	desc := p.Describe()

	if desc.Label != "loopback ping" {
		t.Errorf("expected Descriptor.Label 'loopback ping', got %q", desc.Label)
	}
	if len(desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(desc.Metrics))
	}
	m := desc.Metrics[0]
	if m.ResultKey != "127.0.0.1" {
		t.Errorf("expected ResultKey '127.0.0.1', got %q", m.ResultKey)
	}
	if m.DSName != "addr0" {
		t.Errorf("expected DSName 'addr0', got %q", m.DSName)
	}
	if m.Label != "127.0.0.1" {
		t.Errorf("expected Label '127.0.0.1', got %q", m.Label)
	}
	if m.Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", m.Unit)
	}
	if m.Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", m.Scale)
	}
}

func TestDescribe_MultipleAddresses(t *testing.T) {
	p, _ := New([]string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}, "dns")
	desc := p.Describe()

	if desc.Label != "dns" {
		t.Errorf("expected Descriptor.Label 'dns', got %q", desc.Label)
	}
	if len(desc.Metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(desc.Metrics))
	}
	for i, addr := range []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"} {
		if desc.Metrics[i].ResultKey != addr {
			t.Errorf("metric %d: expected ResultKey %q, got %q", i, addr, desc.Metrics[i].ResultKey)
		}
		expectedDS := fmt.Sprintf("addr%d", i)
		if desc.Metrics[i].DSName != expectedDS {
			t.Errorf("metric %d: expected DSName %q, got %q", i, expectedDS, desc.Metrics[i].DSName)
		}
	}
}

func TestDescribe_IsInstanceSpecific(t *testing.T) {
	p1, _ := New([]string{"1.1.1.1"}, "one")
	p2, _ := New([]string{"1.1.1.1", "8.8.8.8"}, "two")

	if len(p1.Describe().Metrics) != 1 {
		t.Errorf("expected 1 metric for p1, got %d", len(p1.Describe().Metrics))
	}
	if len(p2.Describe().Metrics) != 2 {
		t.Errorf("expected 2 metrics for p2, got %d", len(p2.Describe().Metrics))
	}
}

// --- Factory tests ---

func TestFactory_MinimalConfig(t *testing.T) {
	config := map[string]any{
		"addresses": []any{"127.0.0.1"},
		"label":     "loopback",
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := chk.(*Ping)
	if len(p.addresses) != 1 {
		t.Fatalf("expected 1 address, got %d", len(p.addresses))
	}
	if p.addresses[0].address != "127.0.0.1" {
		t.Errorf("expected address '127.0.0.1', got %q", p.addresses[0].address)
	}
}

func TestFactory_MultipleAddresses(t *testing.T) {
	config := map[string]any{
		"addresses": []any{"1.1.1.1", "8.8.8.8"},
		"label":     "dns",
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := chk.(*Ping)
	if len(p.addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(p.addresses))
	}
}

func TestFactory_FullConfig(t *testing.T) {
	config := map[string]any{
		"addresses": []any{"127.0.0.1"},
		"label":     "loopback",
		"timeout":   "5s",
		"count":     float64(3),
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := chk.(*Ping)
	if p.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", p.timeout)
	}
	if p.count != 3 {
		t.Errorf("expected count 3, got %d", p.count)
	}
}

func TestFactory_IgnoresTargetKey(t *testing.T) {
	config := map[string]any{
		"target":    "injected-by-worker",
		"addresses": []any{"127.0.0.1"},
		"label":     "test",
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := chk.(*Ping)
	if len(p.addresses) != 1 || p.addresses[0].address != "127.0.0.1" {
		t.Errorf("expected address from addresses config, not target")
	}
}

func TestFactory_StringSliceAddresses(t *testing.T) {
	config := map[string]any{
		"addresses": []string{"127.0.0.1"},
		"label":     "test",
	}
	_, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactory_MissingAddresses(t *testing.T) {
	_, err := Factory(map[string]any{"label": "test"})
	if err == nil {
		t.Error("expected error for missing addresses")
	}
}

func TestFactory_EmptyAddresses(t *testing.T) {
	_, err := Factory(map[string]any{"addresses": []any{}, "label": "test"})
	if err == nil {
		t.Error("expected error for empty addresses")
	}
}

func TestFactory_NonStringAddress(t *testing.T) {
	_, err := Factory(map[string]any{"addresses": []any{123}, "label": "test"})
	if err == nil {
		t.Error("expected error for non-string address")
	}
}

func TestFactory_WrongAddressesType(t *testing.T) {
	_, err := Factory(map[string]any{"addresses": "not-a-list", "label": "test"})
	if err == nil {
		t.Error("expected error for non-list addresses")
	}
}

func TestFactory_MissingLabel(t *testing.T) {
	_, err := Factory(map[string]any{"addresses": []any{"127.0.0.1"}})
	if err == nil {
		t.Error("expected error for missing label")
	}
}

func TestFactory_EmptyLabel(t *testing.T) {
	_, err := Factory(map[string]any{"addresses": []any{"127.0.0.1"}, "label": ""})
	if err == nil {
		t.Error("expected error for empty label")
	}
}

func TestFactory_WrongLabelType(t *testing.T) {
	_, err := Factory(map[string]any{"addresses": []any{"127.0.0.1"}, "label": 42})
	if err == nil {
		t.Error("expected error for non-string label")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	_, err := Factory(map[string]any{
		"addresses": []any{"127.0.0.1"},
		"label":     "test",
		"timeout":   "bad",
	})
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestFactory_WrongTimeoutType(t *testing.T) {
	_, err := Factory(map[string]any{
		"addresses": []any{"127.0.0.1"},
		"label":     "test",
		"timeout":   123,
	})
	if err == nil {
		t.Error("expected error for non-string timeout")
	}
}

func TestFactory_WrongCountType(t *testing.T) {
	_, err := Factory(map[string]any{
		"addresses": []any{"127.0.0.1"},
		"label":     "test",
		"count":     "three",
	})
	if err == nil {
		t.Error("expected error for non-numeric count")
	}
}

// --- Registry integration ---

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	if err := reg.Register(TypeName, Factory); err != nil {
		t.Fatalf("failed to register ping: %v", err)
	}

	chk, err := reg.Create("ping", map[string]any{
		"addresses": []any{"1.1.1.1", "8.8.8.8"},
		"label":     "dns servers",
	})
	if err != nil {
		t.Fatalf("failed to create ping check: %v", err)
	}
	if chk.Type() != "ping" {
		t.Errorf("expected type 'ping', got %q", chk.Type())
	}

	desc := chk.Describe()
	if desc.Label != "dns servers" {
		t.Errorf("expected Descriptor.Label 'dns servers', got %q", desc.Label)
	}
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].ResultKey != "1.1.1.1" {
		t.Errorf("expected first ResultKey '1.1.1.1', got %q", desc.Metrics[0].ResultKey)
	}
	if desc.Metrics[1].ResultKey != "8.8.8.8" {
		t.Errorf("expected second ResultKey '8.8.8.8', got %q", desc.Metrics[1].ResultKey)
	}
	if desc.Metrics[0].Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", desc.Metrics[0].Scale)
	}
}

// TestRun_Localhost pings localhost. Requires ping binary on PATH.
func TestRun_Localhost(t *testing.T) {
	p, err := New([]string{"127.0.0.1"}, "loopback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := p.Run(context.Background())
	if !result.Success {
		t.Skipf("ping failed (may not have permission): %v", result.Err)
	}
	if result.Metrics["127.0.0.1"] <= 0 {
		t.Errorf("expected positive latency, got %d", result.Metrics["127.0.0.1"])
	}
}
