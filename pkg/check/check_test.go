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

func TestResult_ZeroValue(t *testing.T) {
	var r Result
	if r.Success {
		t.Error("zero Result should not be successful")
	}
	if r.Err != nil {
		t.Error("zero Result should have nil error")
	}
	if r.Metrics != nil {
		t.Error("zero Result should have nil metrics")
	}
	if !r.Timestamp.IsZero() {
		t.Error("zero Result should have zero timestamp")
	}
}

func TestResult_WithMetrics(t *testing.T) {
	r := Result{
		Timestamp: time.Now(),
		Success:   true,
		Metrics:   map[string]float64{"latency_us": 1234.5},
	}
	if !r.Success {
		t.Error("expected success")
	}
	if v, ok := r.Metrics["latency_us"]; !ok || v != 1234.5 {
		t.Errorf("expected latency_us=1234.5, got %v", v)
	}
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
		return &stubCheck{typeName: "configtest"}, nil
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
