package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

// CheckStatusResponse represents the status of a single check in the API response.
type CheckStatusResponse struct {
	Alive      bool               `json:"alive"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
	LastUpdate int64              `json:"lastupdate"`
}

// HostAPIResponse represents a host in the API response.
type HostAPIResponse struct {
	Address string                         `json:"address,omitempty"`
	Checks  map[string]CheckStatusResponse `json:"checks"`
}

//go:embed static/*
var staticFlies embed.FS

//go:embed templates/*
var templateFiles embed.FS

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

	http.Handle("/host-detail", hostDetailHandler(templateFiles))

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
		checksResponse := make(map[string]CheckStatusResponse)

		// Get all check statuses for this host
		snapshots := s.hostStatuses(name)
		for checkType, snap := range snapshots {
			checksResponse[checkType] = CheckStatusResponse{
				Alive:      snap.Alive,
				Metrics:    snap.Metrics,
				LastUpdate: snap.LastUpdate,
			}
		}

		hosts[name] = HostAPIResponse{
			Address: h.Address,
			Checks:  checksResponse,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) handlePrometheus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	w.Write([]byte("# HELP check_alive Whether the check target is reachable (1=up, 0=down).\n"))
	w.Write([]byte("# TYPE check_alive gauge\n"))
	w.Write([]byte("# HELP check_metric Check metric value.\n"))
	w.Write([]byte("# TYPE check_metric gauge\n"))

	for name, h := range s.hosts {
		snapshots := s.hostStatuses(name)
		for checkType, snap := range snapshots {
			aliveVal := 0
			if snap.Alive {
				aliveVal = 1
			}
			w.Write(fmt.Appendf([]byte{},
				"check_alive{host=\"%s\", address=\"%s\", check=\"%s\"} %d\n",
				name,
				h.Address,
				checkType,
				aliveVal,
			))
			for metricKey, metricVal := range snap.Metrics {
				w.Write(fmt.Appendf([]byte{},
					"check_metric{host=\"%s\", address=\"%s\", check=\"%s\", metric=\"%s\"} %f\n",
					name,
					h.Address,
					checkType,
					metricKey,
					metricVal,
				))
			}
		}
	}
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

func hostDetailHandler(templateFiles embed.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var hostPageTemplate = template.Must(template.ParseFS(templateFiles, "templates/host-page.html.tmpl"))
		var hostname = r.URL.Query().Get("hostname")

		hostPageTemplate.Execute(w, struct {
			Hostname string
		}{
			Hostname: hostname,
		})
	})
}
