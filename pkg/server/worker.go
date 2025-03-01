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

	// Initialize RRD file for the host
	rrdFile, err := rrd.NewRRD(h, s.rrdDir, s.htmlDir, "latency", s.logger)
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

	// Add a random delay of 1-59 seconds before starting
	startDelay := time.Duration(rand.Intn(59)+1) * time.Second
	s.logger.Debugf("Worker for host %s will start in %v", name, startDelay)
	select {
	case <-time.After(startDelay):
		// Perform the initial ping
		performPing()
	case <-s.done:
		s.logger.Infof("Worker for host %s received shutdown signal before starting", name)
		return
	}

	// Run periodic pings every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Perform the periodic ping
			performPing()
		case <-s.done:
			s.logger.Infof("Worker for host %s received shutdown signal.", name)
			return
		}
	}
}
