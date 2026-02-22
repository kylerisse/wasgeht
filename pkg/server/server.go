package server

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/check/ping"
	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/sirupsen/logrus"
)

// Server represents the ping server
type Server struct {
	hosts      map[string]*host.Host
	statuses   map[string]map[string]*check.Status // host -> checkType -> status
	statusesMu sync.RWMutex                        // protects the statuses map structure
	registry   *check.Registry
	done       chan struct{}
	wg         sync.WaitGroup
	logger     *logrus.Logger
	rrdDir     string
	graphDir   string
	listenPort string
}

// NewServer initializes a new server with the given host file
func NewServer(hostFile string, rrdDir string, graphDir string, listenPort string, logger *logrus.Logger) (*Server, error) {
	hosts, err := loadHosts(hostFile)
	if err != nil {
		return nil, err
	}

	registry := check.NewRegistry()
	if err := registry.Register(ping.TypeName, ping.Factory); err != nil {
		return nil, fmt.Errorf("failed to register ping check: %w", err)
	}

	// Initialize the statuses map with an empty map per host
	statuses := make(map[string]map[string]*check.Status, len(hosts))
	for name := range hosts {
		statuses[name] = make(map[string]*check.Status)
	}

	return &Server{
		hosts:      hosts,
		statuses:   statuses,
		registry:   registry,
		done:       make(chan struct{}),
		logger:     logger,
		rrdDir:     rrdDir,
		graphDir:   graphDir,
		listenPort: listenPort,
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

// getOrCreateStatus returns the status for a host/check pair, creating it if needed.
func (s *Server) getOrCreateStatus(hostName, checkType string) *check.Status {
	s.statusesMu.Lock()
	defer s.statusesMu.Unlock()

	if _, ok := s.statuses[hostName]; !ok {
		s.statuses[hostName] = make(map[string]*check.Status)
	}
	if _, ok := s.statuses[hostName][checkType]; !ok {
		s.statuses[hostName][checkType] = check.NewStatus()
	}
	return s.statuses[hostName][checkType]
}

// hostStatuses returns a snapshot of all check statuses for a given host.
func (s *Server) hostStatuses(hostName string) map[string]check.StatusSnapshot {
	s.statusesMu.RLock()
	defer s.statusesMu.RUnlock()

	checks, ok := s.statuses[hostName]
	if !ok {
		return nil
	}

	snapshots := make(map[string]check.StatusSnapshot, len(checks))
	for checkType, status := range checks {
		snapshots[checkType] = status.Snapshot()
	}
	return snapshots
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
		newHost.ApplyDefaults()
		hostPointers[name] = &newHost
	}

	return hostPointers, nil
}
