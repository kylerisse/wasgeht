package host

// DefaultChecks is the check configuration applied to hosts that don't
// declare an explicit "checks" block — ping with default settings.
var DefaultChecks = map[string]map[string]any{
	"ping": {},
}

// Host represents the configuration of a monitored host.
// It holds identity and check configuration only — runtime state
// (alive, latency, etc.) is tracked per-check in check.Status.
type Host struct {
	Name    string                    // Name of the host
	Address string                    `json:"address,omitempty"` // Optional address
	Checks  map[string]map[string]any `json:"checks,omitempty"`  // Per-check-type configuration
}

// ApplyDefaults fills in zero-value fields with sensible defaults.
// If Checks is nil or empty, it is set to DefaultChecks.
func (h *Host) ApplyDefaults() {
	if len(h.Checks) == 0 {
		h.Checks = make(map[string]map[string]any, len(DefaultChecks))
		for k, v := range DefaultChecks {
			cp := make(map[string]any, len(v))
			for ck, cv := range v {
				cp[ck] = cv
			}
			h.Checks[k] = cp
		}
	}
}

// EnabledChecks returns the subset of Checks that are not explicitly
// disabled via "enabled": false.
func (h *Host) EnabledChecks() map[string]map[string]any {
	enabled := make(map[string]map[string]any)
	for name, cfg := range h.Checks {
		if v, ok := cfg["enabled"]; ok {
			if b, ok := v.(bool); ok && !b {
				continue
			}
		}
		enabled[name] = cfg
	}
	return enabled
}
