package server

import (
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

// HostStatus represents the aggregate status of a host across all its checks.
// It is computed from the check-level StatusSnapshots and included in API responses.
// The string values are stable and consumed by the web UI for color-coding.
type HostStatus string

const (
	// HostStatusUnconfigured means the host has no checks defined.
	HostStatusUnconfigured HostStatus = "unconfigured"
	// HostStatusPending means checks are defined but none have run yet.
	HostStatusPending HostStatus = "pending"
	// HostStatusStale means checks have run before but all results are beyond the staleness window.
	HostStatusStale HostStatus = "stale"
	// HostStatusUp means all checks are up and have recent results.
	HostStatusUp HostStatus = "up"
	// HostStatusDegraded means some checks are up and some are down, stale, or pending.
	HostStatusDegraded HostStatus = "degraded"
	// HostStatusDown means all checks have fresh results and all are down.
	HostStatusDown HostStatus = "down"
)

// stalenessWindow is how old a check result can be before it's considered stale.
const stalenessWindow = 5 * time.Minute

// computeHostStatus determines the aggregate status of a host from its check snapshots.
// Each check is classified into one of four buckets:
//   - never_run:  LastUpdate == 0
//   - fresh_up:   LastUpdate > cutoff && Alive
//   - fresh_down: LastUpdate > cutoff && !Alive
//   - stale:      LastUpdate > 0 && LastUpdate <= cutoff
func computeHostStatus(snapshots map[string]check.StatusSnapshot, now time.Time) HostStatus {
	if len(snapshots) == 0 {
		return HostStatusUnconfigured
	}

	cutoff := now.Add(-stalenessWindow).Unix()

	var neverRun, freshUp, freshDown, staleCount int
	for _, snap := range snapshots {
		switch {
		case snap.LastUpdate == 0:
			neverRun++
		case snap.LastUpdate > cutoff && snap.Alive:
			freshUp++
		case snap.LastUpdate > cutoff && !snap.Alive:
			freshDown++
		default:
			staleCount++
		}
	}

	switch {
	case neverRun == len(snapshots):
		return HostStatusPending
	case freshUp > 0 && freshDown == 0 && staleCount == 0 && neverRun == 0:
		return HostStatusUp
	case freshUp > 0:
		return HostStatusDegraded
	case freshDown > 0 && staleCount == 0 && neverRun == 0:
		return HostStatusDown
	default:
		return HostStatusStale
	}
}
