package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
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

	// Add a random delay of 1-59 seconds before starting
	startDelay := time.Duration(rand.Intn(59)+1) * time.Second
	log.Printf("Worker for host %s will start in %v\n", name, startDelay)
	select {
	case <-time.After(startDelay):
		// After the random delay, perform the first ping
		latency, err := h.Ping(name, 3*time.Second)
		h.LastUpdate = time.Now().Unix()
		if err != nil {
			log.Printf("Worker for host %s: Initial ping failed (%v)\n", name, err)
			h.Alive = false
		} else {
			log.Printf("Worker for host %s: Initial ping successful, Latency=%v\n", name, latency)
			h.Alive = true
			h.Latency = latency
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

func (s *Server) startAPI() {
	// Serve index.html for "/"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("html", "index.html"))
	})

	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		s.handleAPI(w, r)
	})

	go func() {
		log.Println("Starting API server on port 1982...")
		if err := http.ListenAndServe(":1982", nil); err != nil {
			log.Fatalf("Failed to start API server: %v\n", err)
		}
	}()
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	hosts := make(map[string]host.Host)
	for name, h := range s.hosts {
		hosts[name] = *h
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
