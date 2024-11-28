package server

import (
	"log"
	"math/rand"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/kylerisse/wasgeht/pkg/rrd"
)

// worker periodically pings the assigned host
func (s *Server) worker(name string, h *host.Host) {
	defer s.wg.Done()

	// Initialize RRD file for the host
	rrdFile, err := rrd.NewRRD(name, "./rrds", "latency")
	if err != nil {
		log.Printf("Worker for host %s: Failed to initialize RRD (%v)\n", name, err)
		return
	}

	// Define the performPing function as an anonymous function
	performPing := func() {
		latency, err := h.Ping(name, 3*time.Second)
		if err != nil {
			log.Printf("Worker for host %s: Ping failed (%v)\n", name, err)
			h.Alive = false
		} else {
			log.Printf("Worker for host %s: Latency=%v (Ping successful)\n", name, latency)
			h.Alive = true
			h.Latency = latency
			// Update the RRD file with the fetched latency and timestamp
			err = rrdFile.SafeUpdate(time.Now(), []float64{float64(latency.Microseconds())})
			if err != nil {
				log.Printf("Worker for host %s: Failed to update RRD (%v)\n", name, err)
			}
		}
	}

	// Add a random delay of 1-59 seconds before starting
	startDelay := time.Duration(rand.Intn(59)+1) * time.Second
	log.Printf("Worker for host %s will start in %v\n", name, startDelay)
	select {
	case <-time.After(startDelay):
		// Perform the initial ping
		performPing()
	case <-s.done:
		log.Printf("Worker for host %s received shutdown signal before starting\n", name)
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
			log.Printf("Worker for host %s received shutdown signal.\n", name)
			return
		}
	}
}
