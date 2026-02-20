package server

import (
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

// HostStatus represents the aggregate status of a host across all its checks.
// It is computed from the check-level StatusSnapshots and included in API responses.
// The string values are stable and consumed by the web UI for color-coding:
// "unknown" (gray), "up" (green), "degraded" (yellow), "down" (red).
type HostStatus string

const (
	// HostStatusUnknown means no checks configured or no results within the staleness window.
	HostStatusUnknown HostStatus = "unknown"
	// HostStatusUp means all checks are up and have recent results.
	HostStatusUp HostStatus = "up"
	// HostStatusDegraded means some checks are up and some are down or stale.
	HostStatusDegraded HostStatus = "degraded"
	// HostStatusDown means all checks are down.
	HostStatusDown HostStatus = "down"
)

// stalenessWindow is how old a check result can be before it's considered stale.
const stalenessWindow = 5 * time.Minute

// computeHostStatus determines the aggregate status of a host from its check snapshots.
// A check is considered "fresh and up" if it is alive and its last update is strictly
// within the staleness window (i.e., more recent than now minus stalenessWindow).
// A check is considered "down" if it is not alive, or if its last update is at or
// beyond the staleness boundary (older than or equal to the cutoff), or zero.
func computeHostStatus(snapshots map[string]check.StatusSnapshot, now time.Time) HostStatus {
	if len(snapshots) == 0 {
		return HostStatusUnknown
	}

	cutoff := now.Add(-stalenessWindow).Unix()
	upCount := 0
	downCount := 0

	for _, snap := range snapshots {
		if snap.Alive && snap.LastUpdate > 0 && snap.LastUpdate > cutoff {
			upCount++
		} else {
			downCount++
		}
	}

	switch {
	case upCount == 0 && downCount == 0:
		return HostStatusUnknown
	case upCount > 0 && downCount == 0:
		return HostStatusUp
	case upCount > 0 && downCount > 0:
		return HostStatusDegraded
	default:
		// All checks are down - but distinguish between "all stale with no results ever"
		// (unknown) and "genuinely down"
		allZeroUpdate := true
		for _, snap := range snapshots {
			if snap.LastUpdate > 0 {
				allZeroUpdate = false
				break
			}
		}
		if allZeroUpdate {
			return HostStatusUnknown
		}
		return HostStatusDown
	}
}
