package host

// Host represents the configuration for a single host
type Host struct {
	Address string   `json:"address,omitempty"` // Optional address
	Radios  []string `json:"radios,omitempty"`  // Optional list of radios
}
