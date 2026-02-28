package server

import (
	"fmt"
	"net/http"
	"strings"
)

// handlePrometheus writes Prometheus-formatted metrics for all hosts and their checks.
func (s *Server) handlePrometheus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	w.Write([]byte("# HELP check_alive Whether the check target is reachable (1=up, 0=down).\n"))
	w.Write([]byte("# TYPE check_alive gauge\n"))
	w.Write([]byte("# HELP check_metric Check metric value.\n"))
	w.Write([]byte("# TYPE check_metric gauge\n"))

	for name := range s.hosts {
		sanitizedName := sanitizePrometheusLabel(name)
		snapshots := s.hostStatuses(name)
		for checkType, snap := range snapshots {
			sanitizedCheck := sanitizePrometheusLabel(checkType)
			aliveVal := 0
			if snap.Alive {
				aliveVal = 1
			}
			w.Write(fmt.Appendf([]byte{},
				"check_alive{host=\"%s\", check=\"%s\"} %d\n",
				sanitizedName,
				sanitizedCheck,
				aliveVal,
			))
			for metricKey, metricVal := range snap.Metrics {
				if metricVal == nil {
					continue
				}
				w.Write(fmt.Appendf([]byte{},
					"check_metric{host=\"%s\", check=\"%s\", metric=\"%s\"} %d\n",
					sanitizedName,
					sanitizedCheck,
					sanitizePrometheusLabel(metricKey),
					*metricVal,
				))
			}
		}
	}
}

// sanitizePrometheusLabel escapes backslash, double-quote, and newline
// characters in a Prometheus label value per the exposition format spec.
func sanitizePrometheusLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
