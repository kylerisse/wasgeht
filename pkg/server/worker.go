package server

import (
	"context"
	"math/rand"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/check/ping"
	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/kylerisse/wasgeht/pkg/rrd"
)

// worker periodically runs a check against the assigned host
func (s *Server) worker(name string, h *host.Host) {
	defer s.wg.Done()

	// Add a random delay of 1-59 seconds before starting to reduce initial filesystem activity
	startDelay := time.Duration(rand.Intn(59)+1) * time.Second
	s.logger.Infof("Worker for host %s will start in %v", name, startDelay)
	select {
	case <-time.After(startDelay):
		s.logger.Debugf("Worker for host %s starting", name)
	case <-s.done:
		s.logger.Infof("Worker for host %s received shutdown signal before starting", name)
		return
	}

	// Initialize the ping check for this host
	target := h.Address
	if target == "" {
		target = name
	}
	pingCheck, err := ping.New(target, ping.WithTimeout(3*time.Second))
	if err != nil {
		s.logger.Errorf("Worker for host %s: Failed to create ping check (%v)", name, err)
		return
	}

	// Initialize RRD file for the host (now takes name string, not *host.Host)
	// checkType names the file (ping.rrd), dsName names the data source inside it (latency)
	rrdFile, err := rrd.NewRRD(name, s.rrdDir, s.graphDir, pingCheck.Type(), "latency", s.logger)
	if err != nil {
		s.logger.Errorf("Worker for host %s: Failed to initialize RRD (%v)", name, err)
		return
	}

	// Define the performCheck function
	performCheck := func() {
		result := pingCheck.Run(context.Background())

		// Translate check.Result back into host state for backward compatibility
		applyPingResult(h, name, result)

		// Build RRD update from result metrics
		latencyUpdate := rrdValuesFromResult(result)

		s.logger.Debugf("Worker for host %s: Updating RRD with values %v.", name, latencyUpdate)
		lastUpdate, err := rrdFile.SafeUpdate(result.Timestamp, latencyUpdate)
		if err != nil {
			s.logger.Errorf("Worker for host %s: Failed to update RRD (%v)", name, err)
		} else {
			// Track LastUpdate on host for backward compatibility with API
			h.LastUpdate = lastUpdate
			s.logger.Debugf("Worker for host %s: RRD update successful.", name)
		}

		if result.Success {
			s.logger.Infof("Worker for host %s: Latency=%v (check successful)", name, h.Latency)
		} else {
			s.logger.Warningf("Worker for host %s: Check failed (%v)", name, result.Err)
		}
	}

	// Run periodic checks every minute
	for {
		select {
		case <-s.done:
			s.logger.Infof("Worker for host %s received shutdown signal.", name)
			return
		default:
			performCheck()

			select {
			case <-time.After(time.Minute):
				// continue with the next iteration
			case <-s.done:
				s.logger.Infof("Worker for host %s received shutdown signal.", name)
				return
			}
		}
	}
}

// applyPingResult translates a check.Result into host state fields
// for backward compatibility with the existing API and UI.
func applyPingResult(h *host.Host, name string, result check.Result) {
	if result.Success {
		h.Alive = true
		if latencyUs, ok := result.Metrics["latency_us"]; ok {
			h.Latency = time.Duration(latencyUs) * time.Microsecond
		}
	} else {
		h.Alive = false
	}
}

// rrdValuesFromResult extracts the latency metric from a check.Result
// as a float64 slice suitable for RRD update. Returns an empty slice
// if the check failed or the metric is absent.
func rrdValuesFromResult(result check.Result) []float64 {
	if !result.Success {
		return []float64{}
	}
	if latencyUs, ok := result.Metrics["latency_us"]; ok {
		return []float64{latencyUs}
	}
	return []float64{}
}
