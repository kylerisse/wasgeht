package host

// Host represents the configuration of a monitored host.
// It holds identity and check configuration only — runtime state
// (alive, latency, etc.) is tracked per-check in check.Status.
// Hosts without an explicit checks block are inert (unknown status).
type Host struct {
	Name   string                    // Name of the host
	Checks map[string]map[string]any `json:"checks,omitempty"` // Per-check-type configuration
}
