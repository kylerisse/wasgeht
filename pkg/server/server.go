package server

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/sirupsen/logrus"
)

// Server represents the ping server
type Server struct {
	hosts    map[string]*host.Host
	done     chan struct{}
	wg       sync.WaitGroup
	logger   *logrus.Logger
	rrdDir   string
	graphDir string
}

// NewServer initializes a new server with the given host file
func NewServer(hostFile string, rrdDir string, graphDir string, logger *logrus.Logger) (*Server, error) {
	hosts, err := loadHosts(hostFile)
	if err != nil {
		return nil, err
	}

	return &Server{
		hosts:    hosts,
		done:     make(chan struct{}),
		logger:   logger,
		rrdDir:   rrdDir,
		graphDir: graphDir,
	}, nil
}

// Start begins a worker for each host
func (s *Server) Start() {
	s.logger.Info("Starting workers for each host...")

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
	s.logger.Info("All workers stopped.")
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
		newHost := h // Create a new instance for the pointer
		newHost.Name = name
		hostPointers[name] = &newHost
	}

	return hostPointers, nil
}
