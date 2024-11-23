package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
)

func main() {
	// Load hosts from JSON file
	hosts, err := loadHosts("sample-hosts.json")
	if err != nil {
		log.Fatalf("Failed to load hosts: %v\n", err)
	}

	// Print loaded hosts for debugging
	fmt.Println("Loaded hosts:")
	for name, h := range hosts {
		fmt.Printf("- %s: Address=%s, Radios=%v\n", name, h.Address, h.Radios)
	}

	// Ping each host and print results
	fmt.Println("Pinging hosts:")
	for name, h := range hosts {
		latency, err := h.Ping(name, 1*time.Second) // Ping with 3-second timeout
		if err != nil {
			fmt.Printf("- %s: Ping failed (%v)\n", name, err)
		} else {
			fmt.Printf("- %s: Latency=%v\n", name, latency)
		}
	}

	// Graceful shutdown setup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Define HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello")
	})

	// Start HTTP server
	server := &http.Server{Addr: ":1984"}
	go func() {
		fmt.Println("Starting server on port 1984...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on port 1984: %v\n", err)
		}
	}()

	// Wait for termination signal
	<-stop
	fmt.Println("Shutting down server...")

	// Clean up and gracefully shut down the server
	if err := server.Close(); err != nil {
		log.Fatalf("Error shutting down the server: %v\n", err)
	}

	fmt.Println("Server stopped.")
}

func loadHosts(filePath string) (map[string]host.Host, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", filePath, err)
	}

	var hosts map[string]host.Host
	if err := json.Unmarshal(file, &hosts); err != nil {
		return nil, fmt.Errorf("could not parse JSON: %w", err)
	}

	return hosts, nil
}
