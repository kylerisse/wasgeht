package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"time"
)

// CheckStatusResponse represents the status of a single check in the API response.
type CheckStatusResponse struct {
	Alive      bool             `json:"alive"`
	Metrics    map[string]int64 `json:"metrics,omitempty"`
	LastUpdate int64            `json:"lastupdate"`
}

// HostAPIResponse represents a host in the API response.
type HostAPIResponse struct {
	Status HostStatus                     `json:"status"`
	Tags   map[string]string              `json:"tags,omitempty"`
	Checks map[string]CheckStatusResponse `json:"checks"`
}

// handleAPI writes a JSON response containing the status of all hosts and their checks.
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	hosts := make(map[string]HostAPIResponse)
	for name := range s.hosts {
		checksResponse := make(map[string]CheckStatusResponse)

		snapshots := s.hostStatuses(name)
		for checkType, snap := range snapshots {
			checksResponse[checkType] = CheckStatusResponse{
				Alive:      snap.Alive,
				Metrics:    snap.Metrics,
				LastUpdate: snap.LastUpdate,
			}
		}

		hosts[name] = HostAPIResponse{
			Status: computeHostStatus(snapshots, now),
			Tags:   s.hosts[name].Tags,
			Checks: checksResponse,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// hostDetailHandler returns an http.Handler that renders the host detail page
// using the hostname query parameter.
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

// noCacheMiddleware wraps an http.Handler and sets headers to prevent caching.
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Surrogate-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
