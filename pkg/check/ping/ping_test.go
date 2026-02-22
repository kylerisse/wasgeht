package ping

import (
	"context"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestNew_ValidTarget(t *testing.T) {
	p, err := New("localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.target != "localhost" {
		t.Errorf("expected target 'localhost', got %q", p.target)
	}
	if p.timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %v", p.timeout)
	}
	if p.count != DefaultCount {
		t.Errorf("expected default count, got %d", p.count)
	}
}

func TestNew_EmptyTarget(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestNew_WithOptions(t *testing.T) {
	p, err := New("localhost",
		WithTimeout(5*time.Second),
		WithCount(2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", p.timeout)
	}
	if p.count != 2 {
		t.Errorf("expected count 2, got %d", p.count)
	}
}

func TestWithTimeout_Invalid(t *testing.T) {
	_, err := New("localhost", WithTimeout(-1*time.Second))
	if err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestWithCount_Invalid(t *testing.T) {
	_, err := New("localhost", WithCount(0))
	if err == nil {
		t.Error("expected error for count=0")
	}
}

func TestType(t *testing.T) {
	p, _ := New("localhost")
	if p.Type() != "ping" {
		t.Errorf("expected type 'ping', got %q", p.Type())
	}
}

func TestDescribe(t *testing.T) {
	p, _ := New("localhost")
	desc := p.Describe()
	if len(desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric in Describe(), got %d", len(desc.Metrics))
	}
	m := desc.Metrics[0]
	if m.ResultKey != "latency_us" {
		t.Errorf("expected ResultKey 'latency_us', got %q", m.ResultKey)
	}
	if m.DSName != "latency" {
		t.Errorf("expected DSName 'latency', got %q", m.DSName)
	}
	if m.Label != "latency" {
		t.Errorf("expected Label 'latency', got %q", m.Label)
	}
	if m.Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", m.Unit)
	}
	if m.Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", m.Scale)
	}
}

func TestDesc(t *testing.T) {
	if len(Desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric in Desc, got %d", len(Desc.Metrics))
	}
	m := Desc.Metrics[0]
	if m.ResultKey != "latency_us" {
		t.Errorf("expected ResultKey 'latency_us', got %q", m.ResultKey)
	}
	if m.DSName != "latency" {
		t.Errorf("expected DSName 'latency', got %q", m.DSName)
	}
	if m.Label != "latency" {
		t.Errorf("expected Label 'latency', got %q", m.Label)
	}
	if m.Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", m.Unit)
	}
	if m.Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", m.Scale)
	}
}

func TestFactory_FullConfig(t *testing.T) {
	config := map[string]any{
		"target":  "localhost",
		"timeout": "5s",
		"count":   float64(2),
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := chk.(*Ping)
	if p.target != "localhost" {
		t.Errorf("expected target 'localhost', got %q", p.target)
	}
	if p.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", p.timeout)
	}
	if p.count != 2 {
		t.Errorf("expected count 2, got %d", p.count)
	}
}

func TestFactory_MinimalConfig(t *testing.T) {
	config := map[string]any{
		"target": "localhost",
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := chk.(*Ping)
	if p.timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %v", p.timeout)
	}
	if p.count != DefaultCount {
		t.Errorf("expected default count, got %d", p.count)
	}
}

func TestFactory_MissingTarget(t *testing.T) {
	_, err := Factory(map[string]any{})
	if err == nil {
		t.Error("expected error for missing target")
	}
}

func TestFactory_WrongTargetType(t *testing.T) {
	_, err := Factory(map[string]any{"target": 123})
	if err == nil {
		t.Error("expected error for non-string target")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	_, err := Factory(map[string]any{
		"target":  "localhost",
		"timeout": "not-a-duration",
	})
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestFactory_WrongTimeoutType(t *testing.T) {
	_, err := Factory(map[string]any{
		"target":  "localhost",
		"timeout": 123,
	})
	if err == nil {
		t.Error("expected error for non-string timeout")
	}
}

func TestFactory_WrongCountType(t *testing.T) {
	_, err := Factory(map[string]any{
		"target": "localhost",
		"count":  "two",
	})
	if err == nil {
		t.Error("expected error for non-numeric count")
	}
}

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	err := reg.Register(TypeName, Factory)
	if err != nil {
		t.Fatalf("failed to register ping: %v", err)
	}

	chk, err := reg.Create("ping", map[string]any{"target": "localhost"})
	if err != nil {
		t.Fatalf("failed to create ping check: %v", err)
	}
	if chk.Type() != "ping" {
		t.Errorf("expected type 'ping', got %q", chk.Type())
	}

	desc := chk.Describe()
	if len(desc.Metrics) != 1 || desc.Metrics[0].ResultKey != "latency_us" {
		t.Errorf("unexpected descriptor: %+v", desc)
	}
	if desc.Metrics[0].Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", desc.Metrics[0].Scale)
	}
}

// TestRun_Localhost actually pings localhost.
// Requires ping binary on PATH.
func TestRun_Localhost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p, err := New("127.0.0.1", WithTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := p.Run(context.Background())
	if !result.Success {
		t.Fatalf("expected ping to localhost to succeed, got error: %v", result.Err)
	}
	if result.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	latency, ok := result.Metrics["latency_us"]
	if !ok {
		t.Fatal("expected latency_us metric")
	}
	if latency <= 0 {
		t.Errorf("expected positive latency, got %d", latency)
	}
}

// TestRun_UnreachableHost tests ping to an address that should fail.
func TestRun_UnreachableHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p, err := New("192.0.2.1", WithTimeout(1*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := p.Run(context.Background())
	if result.Success {
		t.Error("expected ping to unreachable host to fail")
	}
	if result.Err == nil {
		t.Error("expected non-nil error")
	}
}

// TestRun_ContextCancellation verifies the check respects context cancellation.
func TestRun_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p, err := New("192.0.2.1", WithTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result := p.Run(ctx)
	if result.Success {
		t.Error("expected cancelled check to fail")
	}
}
