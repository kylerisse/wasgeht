package check

import (
	"sync"
	"testing"
	"time"
)

func TestNewStatus_ZeroValues(t *testing.T) {
	s := NewStatus()
	if s.Alive() {
		t.Error("new status should not be alive")
	}
	if s.Latency() != 0 {
		t.Errorf("new status should have zero latency, got %v", s.Latency())
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
	expected := time.Duration(1234) * time.Microsecond
	if s.Latency() != expected {
		t.Errorf("expected latency %v, got %v", expected, s.Latency())
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
}

func TestStatus_SetResult_SuccessWithoutLatency(t *testing.T) {
	s := NewStatus()
	s.SetResult(Result{
		Success: true,
		Metrics: map[string]float64{},
	})

	if !s.Alive() {
		t.Error("expected alive after successful result")
	}
	if s.Latency() != 0 {
		t.Errorf("expected zero latency when metric absent, got %v", s.Latency())
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
	expected := time.Duration(5678) * time.Microsecond
	if snap.Latency != expected {
		t.Errorf("snapshot latency: expected %v, got %v", expected, snap.Latency)
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
			_ = s.Latency()
			_ = s.LastUpdate()
			_ = s.Snapshot()
		}()
	}

	wg.Wait()
}
