package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/kylerisse/wasgeht/pkg/rrd"
)

// Server represents the ping server
type Server struct {
	hosts map[string]*host.Host
	done  chan struct{}
	wg    sync.WaitGroup
}

// NewServer initializes a new server with the given host file
func NewServer(hostFile string) (*Server, error) {
	hosts, err := loadHosts(hostFile)
	if err != nil {
		return nil, err
	}

	return &Server{
		hosts: hosts,
		done:  make(chan struct{}),
	}, nil
}

// Start begins a worker for each host
func (s *Server) Start() {
	log.Println("Starting workers for each host...")

	s.startAPI()

	for name, host := range s.hosts {
		s.wg.Add(1)
		go s.worker(name, host)
	}
}

// Stop gracefully shuts down all workers
func (s *Server) Stop() {
	close(s.done)
	s.wg.Wait()
	log.Println("All workers stopped.")
}

// worker periodically pings the assigned host
func (s *Server) worker(name string, h *host.Host) {
	defer s.wg.Done()

	// Initialize RRD file for the host
	rrdFile, err := rrd.NewRRD(name, "./rrds")
	if err != nil {
		log.Printf("Worker for host %s: Failed to initialize RRD (%v)\n", name, err)
		return
	}

	// Add a random delay of 1-59 seconds before starting
	startDelay := time.Duration(rand.Intn(59)+1) * time.Second
	log.Printf("Worker for host %s will start in %v\n", name, startDelay)
	select {
	case <-time.After(startDelay):
		// After the random delay, perform the first ping
		latency, err := h.Ping(name, 3*time.Second)
		if err != nil {
			log.Printf("Worker for host %s: Initial ping failed (%v)\n", name, err)
			h.Alive = false
		} else {
			log.Printf("Worker for host %s: Initial ping successful, Latency=%v\n", name, latency)
			h.Alive = true
			h.Latency = latency
			// Update the RRD file with the fetched latency and timestamp
			err = rrdFile.SafeUpdate(time.Now(), []float64{float64(latency.Microseconds())})
			if err != nil {
				log.Printf("Worker for host %s: Failed to update RRD (%v)\n", name, err)
			}
		}
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
		case <-s.done:
			log.Printf("Worker for host %s received shutdown signal.\n", name)
			return
		}
	}
}

// loadHosts reads the JSON file and populates a map of host configurations
func loadHosts(filePath string) (map[string]*host.Host, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", filePath, err)
	}

	var hosts map[string]host.Host
	if err := json.Unmarshal(file, &hosts); err != nil {
		return nil, fmt.Errorf("could not parse JSON: %w", err)
	}

	// Convert to a map of pointers
	hostPointers := make(map[string]*host.Host)
	for name, h := range hosts {
		h := h // Create a new instance for the pointer
		hostPointers[name] = &h
	}

	return hostPointers, nil
}
