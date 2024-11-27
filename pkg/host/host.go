package host

import (
	"time"
)

// Host represents the configuration and status of a host
type Host struct {
	Address string        `json:"address,omitempty"` // Optional address
	Radios  []string      `json:"radios,omitempty"`  // Optional list of radios
	Alive   bool          `json:"alive"`             // Whether the host is reachable
	Latency time.Duration `json:"latency"`           // Latest ping latency
}
