package check

import (
	"sync"
	"testing"
)

func TestNewStatus_ZeroValues(t *testing.T) {
	s := NewStatus()
	if s.Alive() {
		t.Error("new status should not be alive")
	}
	if v, ok := s.Metric("latency_us"); ok {
		t.Errorf("new status should have no metrics, got latency_us=%f", v)
	}
	if s.LastUpdate() != 0 {
		t.Errorf("new status should have zero last update, got %d", s.LastUpdate())
	}
}

func TestStatus_SetResult_Success(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 1234.0},
	})

	if !s.Alive() {
		t.Error("expected alive after successful result")
	}
	v, ok := s.Metric("latency_us")
	if !ok {
		t.Fatal("expected latency_us metric to be present")
	}
	if v != 1234.0 {
		t.Errorf("expected latency_us=1234.0, got %f", v)
	}
}

func TestStatus_SetResult_Failure(t *testing.T) {
	s := NewStatus()
	// First set it alive
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 1000.0},
	})
	// Then fail
	s.SetResult(Result{
		Success: false,
	})

	if s.Alive() {
		t.Error("expected not alive after failed result")
	}
	if _, ok := s.Metric("latency_us"); ok {
		t.Error("expected no metrics after failed result")
	}
}

func TestStatus_SetResult_SuccessWithoutMetrics(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{},
	})

	if !s.Alive() {
		t.Error("expected alive after successful result")
	}
	if _, ok := s.Metric("latency_us"); ok {
		t.Error("expected no latency_us when metric absent")
	}
}

func TestStatus_Metric_MultipleMetrics(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{
			"latency_us":    1234.0,
			"response_code": 200.0,
		},
	})

	v, ok := s.Metric("latency_us")
	if !ok || v != 1234.0 {
		t.Errorf("expected latency_us=1234.0, got %f (ok=%v)", v, ok)
	}
	v, ok = s.Metric("response_code")
	if !ok || v != 200.0 {
		t.Errorf("expected response_code=200.0, got %f (ok=%v)", v, ok)
	}
	if _, ok := s.Metric("nonexistent"); ok {
		t.Error("expected nonexistent metric to not be found")
	}
}

func TestStatus_SetLastUpdate(t *testing.T) {
	s := NewStatus()
	s.SetLastUpdate(1700000000)

	if s.LastUpdate() != 1700000000 {
		t.Errorf("expected last update 1700000000, got %d", s.LastUpdate())
	}
}

func TestStatus_Snapshot(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 5678.0},
	})
	s.SetLastUpdate(1700000000)

	snap := s.Snapshot()

	if !snap.Alive {
		t.Error("snapshot should be alive")
	}
	if v, ok := snap.Metrics["latency_us"]; !ok || v != 5678.0 {
		t.Errorf("snapshot metrics: expected latency_us=5678.0, got %v (ok=%v)", v, ok)
	}
	if snap.LastUpdate != 1700000000 {
		t.Errorf("snapshot last update: expected 1700000000, got %d", snap.LastUpdate)
	}
}

func TestStatus_Snapshot_Independent(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 1000.0},
	})

	snap := s.Snapshot()

	// Mutate the status after taking snapshot
	s.SetResult(Result{Success: false})

	// Snapshot should still reflect old state
	if !snap.Alive {
		t.Error("snapshot should be independent of subsequent mutations")
	}
	if _, ok := snap.Metrics["latency_us"]; !ok {
		t.Error("snapshot metrics should be independent of subsequent mutations")
	}
}

func TestStatus_Snapshot_MetricsMapIndependent(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{"latency_us": 1000.0},
	})

	snap := s.Snapshot()

	// Mutate the snapshot's metrics map
	snap.Metrics["latency_us"] = 9999.0

	// Status should be unaffected
	v, ok := s.Metric("latency_us")
	if !ok || v != 1000.0 {
		t.Errorf("mutating snapshot should not affect status, got %f", v)
	}
}

func TestStatus_ConcurrentAccess(t *testing.T) {
	s := NewStatus()
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.SetResult(Result{
				Success: true,
				Metrics: map[string]float64{"latency_us": float64(n)},
			})
			s.SetLastUpdate(int64(n))
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Alive()
			_, _ = s.Metric("latency_us")
			_ = s.LastUpdate()
			_ = s.Snapshot()
		}()
	}

	wg.Wait()
}
