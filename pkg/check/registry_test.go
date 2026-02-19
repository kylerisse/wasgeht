package check

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

// stubCheck is a minimal Check implementation for testing.
type stubCheck struct {
	typeName string
	result   Result
}

func (s *stubCheck) Type() string                 { return s.typeName }
func (s *stubCheck) Run(_ context.Context) Result { return s.result }

func stubFactory(typeName string, result Result) Factory {
	return func(config map[string]any) (Check, error) {
		return &stubCheck{typeName: typeName, result: result}, nil
	}
}

func failingFactory(config map[string]any) (Check, error) {
	return nil, fmt.Errorf("factory error")
}

var stubDescriptor = Descriptor{
	Metrics: []MetricDef{{ResultKey: "value", DSName: "value", Label: "value", Unit: "units"}},
}

func TestRegistry_RegisterAndCreate(t *testing.T) {
	reg := NewRegistry()

	expected := Result{Success: true, Timestamp: time.Now()}
	err := reg.Register("stub", stubFactory("stub", expected), stubDescriptor)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	chk, err := reg.Create("stub", nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if chk.Type() != "stub" {
		t.Errorf("expected type 'stub', got %q", chk.Type())
	}

	result := chk.Run(context.Background())
	if !result.Success {
		t.Error("expected successful result from stub check")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register("dup", stubFactory("dup", Result{}), stubDescriptor)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err = reg.Register("dup", stubFactory("dup", Result{}), stubDescriptor)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestRegistry_CreateUnknownType(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Create("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown check type")
	}
}

func TestRegistry_CreateFactoryError(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register("bad", failingFactory, stubDescriptor)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, err = reg.Create("bad", nil)
	if err == nil {
		t.Error("expected error from failing factory")
	}
}

func TestRegistry_Types(t *testing.T) {
	reg := NewRegistry()

	reg.Register("ping", stubFactory("ping", Result{}), stubDescriptor)
	reg.Register("http", stubFactory("http", Result{}), stubDescriptor)
	reg.Register("tcp", stubFactory("tcp", Result{}), stubDescriptor)

	types := reg.Types()
	sort.Strings(types)

	expected := []string{"http", "ping", "tcp"}
	if len(types) != len(expected) {
		t.Fatalf("expected %d types, got %d", len(expected), len(types))
	}
	for i, typ := range types {
		if typ != expected[i] {
			t.Errorf("expected type %q at index %d, got %q", expected[i], i, typ)
		}
	}
}

func TestRegistry_TypesEmpty(t *testing.T) {
	reg := NewRegistry()
	types := reg.Types()
	if len(types) != 0 {
		t.Errorf("expected empty types, got %v", types)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry()

	var wg sync.WaitGroup
	// Register concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("type-%d", n)
			reg.Register(name, stubFactory(name, Result{}), stubDescriptor)
		}(i)
	}
	wg.Wait()

	// Create concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("type-%d", n)
			chk, err := reg.Create(name, nil)
			if err != nil {
				t.Errorf("Create(%q) failed: %v", name, err)
				return
			}
			if chk.Type() != name {
				t.Errorf("expected type %q, got %q", name, chk.Type())
			}
		}(i)
	}
	wg.Wait()

	types := reg.Types()
	if len(types) != 50 {
		t.Errorf("expected 50 registered types, got %d", len(types))
	}
}

func TestRegistry_ConfigPassthrough(t *testing.T) {
	reg := NewRegistry()

	var receivedConfig map[string]any
	factory := func(config map[string]any) (Check, error) {
		receivedConfig = config
		return &stubCheck{typeName: "configtest"}, nil
	}

	reg.Register("configtest", factory, stubDescriptor)

	config := map[string]any{
		"target":  "example.com",
		"timeout": 5.0,
	}
	reg.Create("configtest", config)

	if receivedConfig == nil {
		t.Fatal("factory did not receive config")
	}
	if receivedConfig["target"] != "example.com" {
		t.Errorf("expected target 'example.com', got %v", receivedConfig["target"])
	}
	if receivedConfig["timeout"] != 5.0 {
		t.Errorf("expected timeout 5.0, got %v", receivedConfig["timeout"])
	}
}

func TestRegistry_Describe(t *testing.T) {
	reg := NewRegistry()

	desc := Descriptor{
		Metrics: []MetricDef{
			{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms"},
		},
	}
	reg.Register("ping", stubFactory("ping", Result{}), desc)

	got, err := reg.Describe("ping")
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if len(got.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(got.Metrics))
	}
	if got.Metrics[0].ResultKey != "latency_us" {
		t.Errorf("expected ResultKey 'latency_us', got %q", got.Metrics[0].ResultKey)
	}
	if got.Metrics[0].DSName != "latency" {
		t.Errorf("expected DSName 'latency', got %q", got.Metrics[0].DSName)
	}
	if got.Metrics[0].Label != "latency" {
		t.Errorf("expected Label 'latency', got %q", got.Metrics[0].Label)
	}
	if got.Metrics[0].Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", got.Metrics[0].Unit)
	}
}

func TestRegistry_DescribeUnknownType(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Describe("nonexistent")
	if err == nil {
		t.Error("expected error for unknown check type")
	}
}

func TestRegistry_DescribeMultipleMetrics(t *testing.T) {
	reg := NewRegistry()

	desc := Descriptor{
		Metrics: []MetricDef{
			{ResultKey: "rx_bytes", DSName: "rx", Label: "received", Unit: "bytes"},
			{ResultKey: "tx_bytes", DSName: "tx", Label: "transmitted", Unit: "bytes"},
		},
	}
	reg.Register("bandwidth", stubFactory("bandwidth", Result{}), desc)

	got, err := reg.Describe("bandwidth")
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if len(got.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(got.Metrics))
	}
}

func TestRegistry_ConcurrentDescribe(t *testing.T) {
	reg := NewRegistry()

	desc := Descriptor{
		Metrics: []MetricDef{{ResultKey: "val", DSName: "val", Label: "value", Unit: "units"}},
	}
	reg.Register("test", stubFactory("test", Result{}), desc)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := reg.Describe("test")
			if err != nil {
				t.Errorf("Describe failed: %v", err)
				return
			}
			if len(got.Metrics) != 1 {
				t.Errorf("expected 1 metric, got %d", len(got.Metrics))
			}
		}()
	}
	wg.Wait()
}
