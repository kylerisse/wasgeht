package server

import (
	"math/rand"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/kylerisse/wasgeht/pkg/rrd"
)

// worker periodically pings the assigned host
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

	// Initialize RRD file for the host
	rrdFile, err := rrd.NewRRD(h, s.rrdDir, s.graphDir, "latency", s.logger)
	if err != nil {
		s.logger.Errorf("Worker for host %s: Failed to initialize RRD (%v)", name, err)
		return
	}

	// Define the performPing function as an anonymous function
	performPing := func() {
		latencyUpdate := []float64{}
		latency, err := h.Ping(name, 3*time.Second)
		if err != nil {
			s.logger.Warningf("Worker for host %s: Ping failed (%v)", name, err)
			h.Alive = false
		} else {
			s.logger.Infof("Worker for host %s: Latency=%v (Ping successful)", name, latency)
			h.Alive = true
			h.Latency = latency
			latencyUpdate = []float64{float64(latency.Microseconds())}
		}
		// Update the RRD file with the fetched latency and timestamp
		// If the slice is empty, SafeUpdate will handle it
		s.logger.Debugf("Worker for host %s: Updating RRD with latency %f microseconds.", name, float64(latency.Microseconds()))
		err = rrdFile.SafeUpdate(time.Now(), latencyUpdate)
		if err != nil {
			s.logger.Errorf("Worker for host %s: Failed to update RRD (%v)", name, err)
		} else {
			s.logger.Debugf("Worker for host %s: RRD update successful.", name)
		}
	}

	// Perform the initial ping before the ticket starts
	performPing()

	// Run periodic pings every minute
	for {
		select {
		case <-s.done:
			s.logger.Infof("Worker for host %s received shutdown signal.", name)
			return
		default:
			// Perform the work synchronously
			performPing()

			// Once the work is done, wait for the interval.
			// If a shutdown signal arrives during this wait, we exit early.
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
