package check

import (
	"sync"
	"time"
)

// Status tracks the latest result of a check execution.
// It is safe for concurrent reads via the exported accessor methods,
// but writes should be done through SetResult.
type Status struct {
	mu         sync.RWMutex
	alive      bool
	latency    time.Duration
	lastUpdate int64
}

// NewStatus creates a Status with zero values (not alive, no latency).
func NewStatus() *Status {
	return &Status{}
}

// Alive returns whether the check's last execution was successful.
func (s *Status) Alive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alive
}

// Latency returns the latency from the check's last execution.
func (s *Status) Latency() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latency
}

// LastUpdate returns the unix timestamp of the last successful RRD update.
func (s *Status) LastUpdate() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdate
}

// SetResult updates the status from a check Result.
// For successful results, it extracts latency from the "latency_us" metric
// if present. For failed results, alive is set to false.
func (s *Status) SetResult(result Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.alive = result.Success
	if result.Success {
		if latencyUs, ok := result.Metrics["latency_us"]; ok {
			s.latency = time.Duration(latencyUs) * time.Microsecond
		}
	}
}

// SetLastUpdate records the unix timestamp of the last successful RRD update.
func (s *Status) SetLastUpdate(ts int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUpdate = ts
}

// Snapshot returns a point-in-time copy of the status fields.
// This is useful for building API responses without holding the lock.
func (s *Status) Snapshot() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return StatusSnapshot{
		Alive:      s.alive,
		Latency:    s.latency,
		LastUpdate: s.lastUpdate,
	}
}

// StatusSnapshot is a point-in-time copy of Status fields.
type StatusSnapshot struct {
	Alive      bool
	Latency    time.Duration
	LastUpdate int64
}
