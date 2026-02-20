package server

import (
	"fmt"
	"net/http"
)

// handlePrometheus writes Prometheus-formatted metrics for all hosts and their checks.
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
