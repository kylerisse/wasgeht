package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/kylerisse/wasgeht/pkg/server"
	"github.com/sirupsen/logrus"
)

func main() {
	logLevel := flag.String("log-level", "info", "Set the logging level (debug, info, warn, error, fatal, panic)")
	hostFile := flag.String("host-file", "sample-hosts.json", "Path to the host configuration file")
	dataDir := flag.String("data-dir", "./data", "Path to the data directory containing 'rrds' and 'graphs' folders")
	listenPort := flag.String("port", "1982", "Port to listen on")
	flag.Parse()

	// Configure logrus to log to stdout with appropriate log level
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.DebugLevel) // Set default log level to INFO
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatalf("Invalid log level '%s': %v", *logLevel, err)
	}
	logger.SetLevel(level)

	var wg sync.WaitGroup

	rrdDir := fmt.Sprintf("%s/rrds", *dataDir)
	graphDir := fmt.Sprintf("%s/graphs", *dataDir)

	// ensure the root data directory exists
	if _, err := os.Stat(*dataDir); os.IsNotExist(err) {
		logger.Fatalf("data directory %v does not exist, %v", dataDir, err)
	}

	// Create the rrds and graphs directories if they don't exist
	if _, err := os.Stat(rrdDir); os.IsNotExist(err) {
		if err := os.Mkdir(rrdDir, 0755); err != nil {
			logger.Fatalf("Failed to create rrds directory: %v", err)
		}
	}
	if _, err := os.Stat(graphDir); os.IsNotExist(err) {
		if err := os.Mkdir(graphDir, 0755); err != nil {
			logger.Fatalf("Failed to create graphs directory: %v", err)
		}
	}

	// Load the server with hosts and configuration
	srv, err := server.NewServer(*hostFile, rrdDir, graphDir, *listenPort, logger)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
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

	logger.Info("Server is running. Press Ctrl+C to stop.")
	<-stop
	logger.Info("Shutting down server...")
	srv.Stop()

	// Wait for the server to finish
	wg.Wait()
	logger.Info("Server stopped.")
}
