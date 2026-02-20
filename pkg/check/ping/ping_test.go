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
		t.Errorf("expected default timeout %v, got %v", DefaultTimeout, p.timeout)
	}
	if p.count != DefaultCount {
		t.Errorf("expected default count %d, got %d", DefaultCount, p.count)
	}
}

func TestNew_EmptyTarget(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestNew_WithOptions(t *testing.T) {
	p, err := New("example.com",
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
	_, err := New("localhost", WithTimeout(0))
	if err == nil {
		t.Error("expected error for zero timeout")
	}

	_, err = New("localhost", WithTimeout(-1*time.Second))
	if err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestWithCount_Invalid(t *testing.T) {
	_, err := New("localhost", WithCount(0))
	if err == nil {
		t.Error("expected error for zero count")
	}

	_, err = New("localhost", WithCount(-1))
	if err == nil {
		t.Error("expected error for negative count")
	}
}

func TestType(t *testing.T) {
	p, _ := New("localhost")
	if p.Type() != "ping" {
		t.Errorf("expected type 'ping', got %q", p.Type())
	}
}

func TestParseOutput_Milliseconds(t *testing.T) {
	output := `PING localhost (127.0.0.1) 56(84) bytes of data.
64 bytes from localhost (127.0.0.1): icmp_seq=1 ttl=64 time=0.042 ms

--- localhost ping statistics ---
1 packets transmitted, 1 received, 0% packet loss, time 0ms
rtt min/avg/max/mdev = 0.042/0.042/0.042/0.000 ms`

	d, err := parseOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != time.Duration(0.042*float64(time.Millisecond)) {
		t.Errorf("expected ~42µs, got %v", d)
	}
}

func TestParseOutput_Microseconds(t *testing.T) {
	output := `64 bytes from localhost: icmp_seq=1 ttl=64 time=42 us`

	d, err := parseOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != time.Duration(42*float64(time.Microsecond)) {
		t.Errorf("expected 42µs, got %v", d)
	}
}

func TestParseOutput_MicrosecondsUnicode(t *testing.T) {
	output := `64 bytes from localhost: icmp_seq=1 ttl=64 time=42 µs`

	d, err := parseOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != time.Duration(42*float64(time.Microsecond)) {
		t.Errorf("expected 42µs, got %v", d)
	}
}

func TestParseOutput_Seconds(t *testing.T) {
	output := `64 bytes from host: icmp_seq=1 ttl=64 time=1.5 s`

	d, err := parseOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != time.Duration(1.5*float64(time.Second)) {
		t.Errorf("expected 1.5s, got %v", d)
	}
}

func TestParseOutput_NoRTT(t *testing.T) {
	output := `PING badhost (0.0.0.0): 56 data bytes
--- badhost ping statistics ---
1 packets transmitted, 0 received, 100% packet loss`

	_, err := parseOutput(output)
	if err == nil {
		t.Error("expected error for output with no RTT")
	}
}

func TestParseOutput_InvalidRTT(t *testing.T) {
	output := `64 bytes from localhost: icmp_seq=1 ttl=64 time=abc ms`

	_, err := parseOutput(output)
	if err == nil {
		t.Error("expected error for non-numeric RTT")
	}
}

func TestParseOutput_UnknownUnit(t *testing.T) {
	output := `64 bytes from localhost: icmp_seq=1 ttl=64 time=42 furlongs`

	_, err := parseOutput(output)
	if err == nil {
		t.Error("expected error for unknown time unit")
	}
}

func TestFactory_Valid(t *testing.T) {
	config := map[string]any{
		"target":  "localhost",
		"timeout": "5s",
		"count":   float64(2),
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.Type() != "ping" {
		t.Errorf("expected type 'ping', got %q", chk.Type())
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

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	err := reg.Register(TypeName, Factory, Desc)
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

	desc, err := reg.Describe("ping")
	if err != nil {
		t.Fatalf("failed to describe ping: %v", err)
	}
	if len(desc.Metrics) != 1 || desc.Metrics[0].ResultKey != "latency_us" {
		t.Errorf("unexpected descriptor: %+v", desc)
	}
	if desc.Metrics[0].Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", desc.Metrics[0].Scale)
	}
}

// TestRun_Localhost actually pings localhost. Requires ping binary on PATH.
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
