package rrd

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// RRD represents an RRD file, including metadata and synchronization tools.
// It contains the file pointer, a mutex for thread safety, a list of data sources, and archive definitions.
type RRD struct {
	file           *os.File    // Pointer to the actual RRD file
	mutex          *sync.Mutex // Wrap file access
	dataSourceName []string    // Ordered list of data sources in the rrd - ex: latency, wifi2ghz, wifi4ghz, etc
	archiveDefs    []string    // List of archive defs - ex: hourly, daily, monthly, etc
}

// NewRRD creates and initializes a new RRD struct for the specified host.
// If the specified RRD file does not exist, it will be created using rrdtool with predefined data sources and archives.
//
// Parameters:
//   - host: The name of the host for which the RRD file will be created.
//   - rrdDir: The directory where the RRD file should be stored.
//
// Returns:
//   - *RRD: A pointer to the newly created RRD struct.
//   - error: An error if something went wrong during the initialization or creation of the RRD file.
func NewRRD(host string, rrdDir string) (*RRD, error) {
	// verify root Dir exists
	if _, err := os.Stat(rrdDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("root directory %s does not exist", rrdDir)
	}

	// Open the RRD file, create it if it doesn't exist
	rrdPath := fmt.Sprintf("%s/%s.rrd", rrdDir, host)
	if _, err := os.Stat(rrdPath); os.IsNotExist(err) {
		cmd := exec.Command("rrdtool", "create", rrdPath,
			"--step", "60",
			"DS:latency:GAUGE:120:0:U",
			"RRA:MAX:0.5:1:60",         // 1-minute max for 1 hour (60 data points)
			"RRA:MAX:0.5:1:480",        // 1-minute max for 8 hours (480 data points)
			"RRA:AVERAGE:0.5:1:1440",   // 1-minute average for 1 day (1440 data points)
			"RRA:AVERAGE:0.5:1:5760",   // 1-minute average for 4 days (5760 data points)
			"RRA:AVERAGE:0.5:1:10080",  // 1-minute average for 1 week (10080 data points)
			"RRA:AVERAGE:0.5:60:720",   // 1-hour average for 1 month (720 data points, 30 days)
			"RRA:AVERAGE:0.5:1440:365", // 1-day average for 1 year (365 data points)
		)

		// Run the command
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to create RRD file %s with rrdtool: %w", rrdPath, err)
		}
	}

	file, err := os.OpenFile(rrdPath, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open RRD file %s: %w", rrdPath, err)
	}

	// Initialize the RRD struct
	rrd := &RRD{
		file:           file,
		mutex:          &sync.Mutex{},
		dataSourceName: []string{"latency"},
		archiveDefs:    []string{"hourly", "8hours", "daily", "4days", "weekly", "monthly", "yearly"},
	}

	return rrd, nil
}
