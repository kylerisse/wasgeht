package check

import (
	"sync"
)

// Status tracks the latest result of a check execution.
// It is safe for concurrent reads via the exported accessor methods,
// but writes should be done through SetResult.
type Status struct {
	mu         sync.RWMutex
	lastResult Result
	lastUpdate int64
}

// NewStatus creates a Status with zero values (not alive, no metrics).
func NewStatus() *Status {
	return &Status{}
}

// Alive returns whether the check's last execution was successful.
func (s *Status) Alive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastResult.Success
}

// Metric returns the value of a named metric from the last result.
// Returns the value and true if found and non-nil, or 0 and false otherwise.
func (s *Status) Metric(key string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.lastResult.Success || s.lastResult.Metrics == nil {
		return 0, false
	}
	v, ok := s.lastResult.Metrics[key]
	if !ok || v == nil {
		return 0, false
	}
	return *v, true
}

// LastUpdate returns the unix timestamp of the last successful RRD update.
func (s *Status) LastUpdate() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdate
}

// SetResult stores the latest check result.
func (s *Status) SetResult(result Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastResult = result
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

	// Deep copy the metrics map so the snapshot is independent
	var metrics map[string]*int64
	if s.lastResult.Metrics != nil {
		metrics = make(map[string]*int64, len(s.lastResult.Metrics))
		for k, v := range s.lastResult.Metrics {
			if v != nil {
				cv := *v
				metrics[k] = &cv
			} else {
				metrics[k] = nil
			}
		}
	}

	return StatusSnapshot{
		Alive:      s.lastResult.Success,
		Metrics:    metrics,
		LastUpdate: s.lastUpdate,
	}
}

// StatusSnapshot is a point-in-time copy of Status fields.
type StatusSnapshot struct {
	Alive      bool
	Metrics    map[string]*int64
	LastUpdate int64
}
