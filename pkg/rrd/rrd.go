package rrd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RRD represents an RRD file, including metadata and synchronization tools.
// It contains the file pointer, a mutex for thread safety, a list of data sources, and archive definitions.
type RRD struct {
	host   string
	file   *os.File      // Pointer to the actual RRD file
	mutex  *sync.RWMutex // Wrap file access
	graphs []*graph
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
			"RRA:MAX:0.5:1:240",        // 1-minute max for 4 hour (60 data points)
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
		host:   host,
		file:   file,
		mutex:  &sync.RWMutex{},
		graphs: []*graph{},
	}

	rrd.initGraphs()

	return rrd, nil
}

// getLastUpdate retrieves the timestamp of the last update from the RRD file.
// It returns the Unix timestamp of the most recent entry.
func (r *RRD) getLastUpdate() (int64, error) {
	// Acquire a read lock for accessing the file.
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Execute the "rrdtool lastupdate" command to get the latest data point info.
	cmd := exec.Command("rrdtool", "lastupdate", r.file.Name())
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to execute rrdtool lastupdate: %w", err)
	}

	// Split the output into lines and get the last one.
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected output format from rrdtool lastupdate")
	}

	// Extract the last line and parse the timestamp.
	lastLine := lines[len(lines)-1]
	parts := strings.Split(lastLine, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected format in the last line: %s", lastLine)
	}

	// Trim any extra spaces and convert the timestamp to int64.
	lastUpdateStr := strings.TrimSpace(parts[0])
	lastUpdate, err := strconv.ParseInt(lastUpdateStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse last update timestamp: %w", err)
	}

	return lastUpdate, nil
}

// SafeUpdate updates the RRD file with the given timestamp and latency value if the timestamp is newer.
//
// This function acquires a write lock to ensure that only one update can be performed at a time.
// It checks if the given timestamp is newer than the latest existing update.
func (r *RRD) SafeUpdate(timestamp time.Time, values []float64) error {
	// Get the last update timestamp.
	lastUpdate, err := r.getLastUpdate()
	if err != nil {
		return fmt.Errorf("failed to get last update: %w", err)
	}

	// If the given timestamp is not newer, skip the update.
	if timestamp.Unix() <= lastUpdate {
		return fmt.Errorf("skipping update as timestamp %d is not newer than last update %d", timestamp.Unix(), lastUpdate)
	}

	// Acquire write lock for updating.
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Prepare the update string: "<timestamp>:<value1>:<value2>:..."
	updateStr := fmt.Sprintf("%d", timestamp.Unix())
	for _, value := range values {
		updateStr += fmt.Sprintf(":%f", value)
	}

	// Execute the "rrdtool update" command to add the new data point.
	cmd := exec.Command("rrdtool", "update", r.file.Name(), updateStr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update RRD file %s with rrdtool: %w", r.file.Name(), err)
	}

	for _, graph := range r.graphs {
		err := graph.draw()
		if err != nil {
			fmt.Println(err)
		}
	}

	return nil
}

// initGraphs initializes a list of graphs for different time lengths and consolidation functions.
// This method adds multiple graphs (e.g., hourly, daily, weekly, etc.) to the RRD.
func (r *RRD) initGraphs() {
	// Define the list of time lengths and consolidation functions for each graph.
	timeLengthsMax := []string{"1h", "4h", "8h"}                 // Only use "MAX" for these time lengths.
	timeLengthsAverage := []string{"1d", "4d", "1w", "1m", "1y"} // Only use "AVERAGE" for these time lengths.

	// Loop over each time length to create graphs with MAX consolidation function.
	for _, timeLength := range timeLengthsMax {
		graph, err := newGraph(r.host, r.file.Name(), timeLength, "MAX")
		if err != nil {
			fmt.Printf("Failed to create MAX graph for host %s with time length %s: %v\n", r.host, timeLength, err)
			continue
		}
		r.graphs = append(r.graphs, graph)
	}

	// Loop over each time length to create graphs with AVERAGE consolidation function.
	for _, timeLength := range timeLengthsAverage {
		graph, err := newGraph(r.host, r.file.Name(), timeLength, "AVERAGE")
		if err != nil {
			fmt.Printf("Failed to create AVERAGE graph for host %s with time length %s: %v\n", r.host, timeLength, err)
			continue
		}
		r.graphs = append(r.graphs, graph)
	}
}
