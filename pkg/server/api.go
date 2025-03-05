package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
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

//go:embed static/*
var staticFlies embed.FS

func (s *Server) startAPI() {
	// Serve api
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		s.handleAPI(w, r)
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		s.handlePrometheus(w, r)
	})

	content, err := fs.Sub(staticFlies, "static")
	if err != nil {
		s.logger.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Serve generated graphs from the graphDir
	imgFS := http.FileServer(http.Dir(s.graphDir))
	http.Handle("/imgs/", noCacheMiddleware(imgFS))

	// Serve static content
	htmlFS := http.FileServer(http.FS(content))
	http.Handle("/", noCacheMiddleware((http.StripPrefix("/", htmlFS))))

	go func() {
		s.logger.Infof("Starting API server on port %v...", s.listenPort)
		if err := http.ListenAndServe(":"+s.listenPort, nil); err != nil {
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

func (s *Server) handlePrometheus(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("# HELP ping_latency_ns Latency in nanosesconds to ping host.\n"))
	w.Write([]byte("# TYPE ping_latency_ns counter\n"))
	for name, h := range s.hosts {
		w.Write(fmt.Appendf([]byte{},
			"ping_latency_ns{host=\"%s\", address=\"%s\", alive=\"%t\"} %d\n",
			name,
			h.Address,
			h.Alive,
			h.Latency.Nanoseconds(),
		))
	}
	w.Header().Set("Content-Type", "text/plain")
}

// noCacheMiddleware sets headers to prevent caching
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Surrogate-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
