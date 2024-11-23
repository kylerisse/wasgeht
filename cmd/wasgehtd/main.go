package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/kylerisse/wasgeht/pkg/server"
)

func main() {
	// Set up logging
	setupLogging()

	var wg sync.WaitGroup

	// Load the server with hosts and configuration
	srv, err := server.NewServer("sample-hosts.json")
	if err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}

	// Start the server
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Start()
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	log.Println("Server is running. Press Ctrl+C to stop.")
	<-stop
	log.Println("Shutting down server...")
	srv.Stop()

	// Wait for the server to finish
	wg.Wait()
	log.Println("Server stopped.")
}

func setupLogging() {
	// Create a log file
	logFile, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// Set logs to output only to the file
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Logging initialized. All logs will be written to server.log")
}
