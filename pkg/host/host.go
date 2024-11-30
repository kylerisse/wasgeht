package host

import (
	"time"
)

// Host represents the configuration and status of a host
type Host struct {
	Name       string        // Name of the host
	Address    string        `json:"address,omitempty"` // Optional address
	Radios     []string      `json:"radios,omitempty"`  // Optional list of radios
	Alive      bool          `json:"alive"`             // Whether the host is reachable
	Latency    time.Duration `json:"latency"`           // Latest ping latency
	LastUpdate int64         `json:"lastupdate"`        // Cached LastUpdate
}
