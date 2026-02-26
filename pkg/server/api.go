package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
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

// APIResponse is the top-level envelope for the /api endpoint.
type APIResponse struct {
	GeneratedAt int64                      `json:"generated_at"`
	Hosts       map[string]HostAPIResponse `json:"hosts"`
}

// parseTagFilters parses ?tag=key:value query params into a map.
// Returns an error if any value is missing a colon, or has an empty key or value.
func parseTagFilters(r *http.Request) (map[string]string, error) {
	filters := make(map[string]string)
	for _, raw := range r.URL.Query()["tag"] {
		k, v, ok := strings.Cut(raw, ":")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("invalid tag filter %q: must be key:value", raw)
		}
		filters[k] = v
	}
	return filters, nil
}

// matchesTagFilters reports whether a host's tags contain all key:value pairs in filters.
func matchesTagFilters(tags map[string]string, filters map[string]string) bool {
	for k, v := range filters {
		if tags[k] != v {
			return false
		}
	}
	return true
}

// parseStatusFilters parses ?status=value query params into a set of HostStatus values.
// Returns an error if any value is not a recognized status.
func parseStatusFilters(r *http.Request) (map[HostStatus]bool, error) {
	valid := map[HostStatus]bool{
		HostStatusUp:       true,
		HostStatusDown:     true,
		HostStatusDegraded: true,
		HostStatusUnknown:  true,
	}
	filters := make(map[HostStatus]bool)
	for _, raw := range r.URL.Query()["status"] {
		s := HostStatus(raw)
		if !valid[s] {
			return nil, fmt.Errorf("invalid status filter %q: must be one of up, down, degraded, unknown", raw)
		}
		filters[s] = true
	}
	return filters, nil
}

// handleAPI writes a JSON response containing the status of all hosts and their checks.
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	tagFilters, err := parseTagFilters(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	statusFilters, err := parseStatusFilters(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hosts := make(map[string]HostAPIResponse)
	for name := range s.hosts {
		if len(tagFilters) > 0 && !matchesTagFilters(s.hosts[name].Tags, tagFilters) {
			continue
		}

		snapshots := s.hostStatuses(name)
		status := computeHostStatus(snapshots, now)

		if len(statusFilters) > 0 && !statusFilters[status] {
			continue
		}

		checksResponse := make(map[string]CheckStatusResponse)
		for checkType, snap := range snapshots {
			checksResponse[checkType] = CheckStatusResponse{
				Alive:      snap.Alive,
				Metrics:    snap.Metrics,
				LastUpdate: snap.LastUpdate,
			}
		}

		hosts[name] = HostAPIResponse{
			Status: status,
			Tags:   s.hosts[name].Tags,
			Checks: checksResponse,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(APIResponse{
		GeneratedAt: now.Unix(),
		Hosts:       hosts,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleHostAPI writes a JSON response for a single host looked up by name.
// Returns 404 if the hostname is not found.
func (s *Server) handleHostAPI(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	name := r.PathValue("hostname")

	if _, ok := s.hosts[name]; !ok {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	checksResponse := make(map[string]CheckStatusResponse)
	snapshots := s.hostStatuses(name)
	for checkType, snap := range snapshots {
		checksResponse[checkType] = CheckStatusResponse{
			Alive:      snap.Alive,
			Metrics:    snap.Metrics,
			LastUpdate: snap.LastUpdate,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(HostAPIResponse{
		Status: computeHostStatus(snapshots, now),
		Tags:   s.hosts[name].Tags,
		Checks: checksResponse,
	}); err != nil {
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
