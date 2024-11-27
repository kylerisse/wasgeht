package server

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/kylerisse/wasgeht/pkg/rrd"
)

type HostAPIResponse struct {
	Address    string        `json:"address,omitempty"`
	Radios     []string      `json:"radios,omitempty"`
	Alive      bool          `json:"alive"`
	Latency    time.Duration `json:"latency"`
	LastUpdate int64         `json:"lastupdate"`
}

func (s *Server) startAPI() {
	// Serve index.html for "/"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("html", "index.html"))
	})

	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		s.handleAPI(w, r)
	})

	go func() {
		log.Println("Starting API server on port 1982...")
		if err := http.ListenAndServe(":1982", nil); err != nil {
			log.Fatalf("Failed to start API server: %v\n", err)
		}
	}()
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	hosts := make(map[string]HostAPIResponse)
	for name, h := range s.hosts {
		// Fetch last update time from RRD
		rrdFile, err := rrd.NewRRD(name, "./rrds")
		if err != nil {
			log.Printf("API Handler: Failed to initialize RRD for host %s (%v)\n", name, err)
			continue
		}
		lastUpdate, err := rrdFile.GetLastUpdate()
		if err != nil {
			log.Printf("API Handler: Failed to get last update for host %s (%v)\n", name, err)
			continue
		}

		// Create a response struct with the last update time
		hostResponse := HostAPIResponse{
			Address:    h.Address,
			Radios:     h.Radios,
			Alive:      h.Alive,
			Latency:    h.Latency,
			LastUpdate: lastUpdate,
		}
		hosts[name] = hostResponse
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
