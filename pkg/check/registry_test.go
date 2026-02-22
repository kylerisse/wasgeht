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
	typeName   string
	result     Result
	descriptor Descriptor
}

func (s *stubCheck) Type() string                 { return s.typeName }
func (s *stubCheck) Describe() Descriptor         { return s.descriptor }
func (s *stubCheck) Run(_ context.Context) Result { return s.result }

var stubDescriptor = Descriptor{
	Metrics: []MetricDef{{ResultKey: "value", DSName: "value", Label: "value", Unit: "units"}},
}

func stubFactory(typeName string, result Result) Factory {
	return func(config map[string]any) (Check, error) {
		return &stubCheck{typeName: typeName, result: result, descriptor: stubDescriptor}, nil
	}
}

func stubFactoryWithDescriptor(typeName string, desc Descriptor) Factory {
	return func(config map[string]any) (Check, error) {
		return &stubCheck{typeName: typeName, descriptor: desc}, nil
	}
}

func failingFactory(config map[string]any) (Check, error) {
	return nil, fmt.Errorf("factory error")
}

func TestRegistry_RegisterAndCreate(t *testing.T) {
	reg := NewRegistry()

	expected := Result{Success: true, Timestamp: time.Now()}
	err := reg.Register("stub", stubFactory("stub", expected))
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

	err := reg.Register("dup", stubFactory("dup", Result{}))
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err = reg.Register("dup", stubFactory("dup", Result{}))
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

	err := reg.Register("bad", failingFactory)
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

	reg.Register("ping", stubFactory("ping", Result{}))
	reg.Register("http", stubFactory("http", Result{}))
	reg.Register("tcp", stubFactory("tcp", Result{}))

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
			reg.Register(name, stubFactory(name, Result{}))
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
		return &stubCheck{typeName: "configtest", descriptor: stubDescriptor}, nil
	}

	reg.Register("configtest", factory)

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

func TestCheck_Describe(t *testing.T) {
	desc := Descriptor{
		Metrics: []MetricDef{
			{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms"},
		},
	}

	chk := &stubCheck{typeName: "ping", descriptor: desc}

	got := chk.Describe()
	if len(got.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(got.Metrics))
	}
	if got.Metrics[0].ResultKey != "latency_us" {
		t.Errorf("expected ResultKey 'latency_us', got %q", got.Metrics[0].ResultKey)
	}
	if got.Metrics[0].DSName != "latency" {
		t.Errorf("expected DSName 'latency', got %q", got.Metrics[0].DSName)
	}
}

func TestCheck_DescribeMultipleMetrics(t *testing.T) {
	desc := Descriptor{
		Metrics: []MetricDef{
			{ResultKey: "rx_bytes", DSName: "rx", Label: "received", Unit: "bytes"},
			{ResultKey: "tx_bytes", DSName: "tx", Label: "transmitted", Unit: "bytes"},
		},
	}

	chk := &stubCheck{typeName: "bandwidth", descriptor: desc}

	got := chk.Describe()
	if len(got.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(got.Metrics))
	}
}

func TestCheck_DescribeDynamic(t *testing.T) {
	// Simulate a factory that builds descriptor from config (like wifi_stations)
	factory := func(config map[string]any) (Check, error) {
		radios, _ := config["radios"].([]string)
		metrics := make([]MetricDef, len(radios))
		for i, radio := range radios {
			metrics[i] = MetricDef{
				ResultKey: radio,
				DSName:    fmt.Sprintf("radio%d", i),
				Label:     radio,
				Unit:      "clients",
			}
		}
		return &stubCheck{
			typeName:   "wifi_stations",
			descriptor: Descriptor{Metrics: metrics},
		}, nil
	}

	reg := NewRegistry()
	reg.Register("wifi_stations", factory)

	chk, err := reg.Create("wifi_stations", map[string]any{
		"radios": []string{"phy0-ap0", "phy1-ap0"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	desc := chk.Describe()
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].ResultKey != "phy0-ap0" {
		t.Errorf("expected first ResultKey 'phy0-ap0', got %q", desc.Metrics[0].ResultKey)
	}
	if desc.Metrics[1].DSName != "radio1" {
		t.Errorf("expected second DSName 'radio1', got %q", desc.Metrics[1].DSName)
	}
}

func TestRegistry_ConcurrentDescribe(t *testing.T) {
	reg := NewRegistry()

	desc := Descriptor{
		Metrics: []MetricDef{{ResultKey: "val", DSName: "val", Label: "value", Unit: "units"}},
	}
	reg.Register("test", stubFactoryWithDescriptor("test", desc))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chk, err := reg.Create("test", nil)
			if err != nil {
				t.Errorf("Create failed: %v", err)
				return
			}
			got := chk.Describe()
			if len(got.Metrics) != 1 {
				t.Errorf("expected 1 metric, got %d", len(got.Metrics))
			}
		}()
	}
	wg.Wait()
}
