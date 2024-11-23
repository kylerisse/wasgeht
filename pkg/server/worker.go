package server

import (
	"log"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
)

// worker processes ping tasks from the task queue
func worker(taskQueue <-chan string, hosts map[string]host.Host, done <-chan struct{}, workerID int) {
	log.Printf("Worker %d started.\n", workerID)
	for {
		select {
		case name, ok := <-taskQueue:
			if !ok {
				log.Printf("Worker %d shutting down...\n", workerID)
				return
			}
			h, exists := hosts[name]
			if !exists {
				log.Printf("Worker %d: Host %s not found\n", workerID, name)
				continue
			}
			log.Printf("Worker %d processing: %s\n", workerID, name)
			latency, err := h.Ping(name, 3*time.Second) // Ping with 3-second timeout
			if err != nil {
				log.Printf("Worker %d: - %s: Ping failed (%v)\n", workerID, name, err)
			} else {
				log.Printf("Worker %d: - %s: Latency=%v (Ping successful)\n", workerID, name, latency)
			}
		case <-done:
			log.Printf("Worker %d received shutdown signal.\n", workerID)
			return
		}
	}
}
