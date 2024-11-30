package server

import (
	"encoding/json"
	"net/http"
	"time"
)

type HostAPIResponse struct {
	Address    string        `json:"address,omitempty"`
	Radios     []string      `json:"radios,omitempty"`
	Alive      bool          `json:"alive"`
	Latency    time.Duration `json:"latency"`
	LastUpdate int64         `json:"lastupdate"`
}

func (s *Server) startAPI() {
	// Serve api
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		s.handleAPI(w, r)
	})

	// Serve static content
	fs := http.FileServer(http.Dir(s.htmlDir))
	http.Handle("/", fs)

	go func() {
		s.logger.Info("Starting API server on port 1982...")
		if err := http.ListenAndServe(":1982", nil); err != nil {
			s.logger.Fatalf("Failed to start API server: %v", err)
		}
	}()
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	hosts := make(map[string]HostAPIResponse)
	for name, h := range s.hosts {
		// Create a response struct with the last update time
		hostResponse := HostAPIResponse{
			Address:    h.Address,
			Radios:     h.Radios,
			Alive:      h.Alive,
			Latency:    h.Latency,
			LastUpdate: h.LastUpdate,
		}
		hosts[name] = hostResponse
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
