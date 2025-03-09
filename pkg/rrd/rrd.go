package rrd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/sirupsen/logrus"
)

// RRD represents an RRD file, including metadata and synchronization tools.
// It contains the file pointer, a mutex for thread safety, a list of data sources, and archive definitions.
type RRD struct {
	host     *host.Host
	metric   string
	file     *os.File      // Pointer to the actual RRD file
	mutex    *sync.RWMutex // Wrap file access
	graphs   []*graph
	logger   *logrus.Logger
	graphDir string
}

// NewRRD creates and initializes a new RRD struct for the specified host.
// If the specified RRD file does not exist, it will be created using rrdtool with predefined data sources and archives.
//
// Parameters:
//   - host: The name of the host for which the RRD file will be created.
//   - rrdDir: The directory where the RRD file should be stored.
//   - graphDir: The directory where the graphs should be stored.
//   - metric: The metric name.
//   - logger: The logger instance.
//
// Returns:
//   - *RRD: A pointer to the newly created RRD struct.
//   - error: An error if something went wrong during the initialization or creation of the RRD file.
func NewRRD(host *host.Host, rrdDir string, graphDir string, metric string, logger *logrus.Logger) (*RRD, error) {
	// verify rrdDir exists
	if _, err := os.Stat(rrdDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", rrdDir)
	}

	// Construct the RRD file path including the metric name
	rrdPath := fmt.Sprintf("%s/%s_%s.rrd", rrdDir, host.Name, metric)
	logger.Debugf("RRD path for host %s and metric %s: %s", host.Name, metric, rrdPath)

	if _, err := os.Stat(rrdPath); os.IsNotExist(err) {
		logger.Debugf("RRD file %s does not exist. Creating new RRD file.", rrdPath)
		cmd := exec.Command("rrdtool", "create", rrdPath,
			"--step", "60",
			fmt.Sprintf("DS:%s:GAUGE:120:0:U", metric),
			"RRA:MAX:0.5:1:10080", // 1-minute max for 1 week (10080 data points)
		)

		// Run the command
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to create RRD file %s with rrdtool: %w", rrdPath, err)
		}
		logger.Debugf("RRD file %s created successfully.", rrdPath)
	} else {
		logger.Debugf("RRD file %s already exists.", rrdPath)
	}

	file, err := os.OpenFile(rrdPath, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open RRD file %s: %w", rrdPath, err)
	}

	// Initialize the RRD struct
	rrd := &RRD{
		host:     host,
		metric:   metric,
		file:     file,
		mutex:    &sync.RWMutex{},
		graphs:   []*graph{},
		logger:   logger,
		graphDir: graphDir,
	}

	rrd.initGraphs()

	logger.Debugf("RRD struct initialized for host %s and metric %s.", host.Name, metric)
	return rrd, nil
}

// getLastUpdate retrieves the timestamp of the last update from the RRD file.
// It returns the Unix timestamp of the most recent entry.
func (r *RRD) getLastUpdate() (int64, error) {

	r.logger.Debugf("Getting last update time for RRD file %s.", r.file.Name())

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

	// Check if the value in the second column is a valid number.
	valueStr := strings.TrimSpace(parts[1])
	if valueStr == "" || valueStr == "NaN" || valueStr == "U" {
		// Value is not a valid number, return 0 and nil
		r.logger.Debugf("Last update value is invalid or undefined for RRD file %s.", r.file.Name())
		return 0, nil
	}

	if _, err := strconv.ParseFloat(valueStr, 64); err != nil {
		// Value is not a valid number, return 0 and nil
		r.logger.Debugf("Last update value is not a valid float for RRD file %s.", r.file.Name())
		return 0, nil
	}

	return lastUpdate, nil
}

// SafeUpdate updates the RRD file with the given timestamp and latency value if the timestamp is newer.
//
// This function acquires a write lock to ensure that only one update can be performed at a time.
// It checks if the given timestamp is newer than the latest existing update.
func (r *RRD) SafeUpdate(timestamp time.Time, values []float64) error {
	r.logger.Debugf("Attempting to update RRD file %s at timestamp %d with values %v.", r.file.Name(), timestamp.Unix(), values)

	// Acquire write lock for updating.
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get the last update timestamp.
	lastUpdate, err := r.getLastUpdate()
	if err != nil {
		return fmt.Errorf("failed to get last update: %w", err)
	}

	// If the given timestamp is not newer, skip the update.
	timestampUnix := timestamp.Unix()
	if timestampUnix <= lastUpdate {
		r.logger.Debugf("Skipping update for RRD file %s: provided timestamp %d is not newer than last update %d.", r.file.Name(), timestamp.Unix(), lastUpdate)
		return fmt.Errorf("skipping update as timestamp %d is not newer than last update %d", timestamp.Unix(), lastUpdate)
	}

	if len(values) > 0 {
		// Prepare the update string: "<timestamp>:<value1>:<value2>:..."
		updateStr := fmt.Sprintf("%d", timestamp.Unix())
		for _, value := range values {
			updateStr += fmt.Sprintf(":%f", value)
		}

		r.logger.Debugf("Updating RRD file %s with update string: %s", r.file.Name(), updateStr)

		// Execute the "rrdtool update" command to add the new data point.
		cmd := exec.Command("rrdtool", "update", r.file.Name(), updateStr)
		r.host.LastUpdate = timestampUnix

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update RRD file %s with rrdtool: %w", r.file.Name(), err)
		}

		r.logger.Debugf("RRD file %s updated successfully.", r.file.Name())
	}

	for _, graph := range r.graphs {
		err := graph.draw()
		if err != nil {
			r.logger.Errorf("Failed to draw graph for RRD file %s: %v", r.file.Name(), err)
		}
	}

	return nil
}

// initGraphs initializes a list of graphs for different time lengths and consolidation functions.
// This method adds multiple graphs (e.g., hourly, daily, weekly, etc.) to the RRD.
func (r *RRD) initGraphs() {
	// Define the map of time lengths and consolidation functions for each graph.
	timeLengths := map[string]string{
		"15m": "MAX",
		"4h":  "MAX",
		"8h":  "MAX",
		"1d":  "AVERAGE",
		"4d":  "AVERAGE",
		"1w":  "AVERAGE",
	}

	// Loop over each time length to create graphs with specified consolidation function.
	for timeLength, conFunc := range timeLengths {
		graph, err := newGraph(r.host.Name, r.graphDir, r.file.Name(), timeLength, conFunc, r.metric, r.logger)
		if err != nil {
			r.logger.Errorf("Failed to create %s graph for host %s with time length %s: %v", conFunc, r.host.Name, timeLength, err)
			continue
		}
		r.graphs = append(r.graphs, graph)
		r.logger.Debugf("Added %s graph for host %s with time length %s.", conFunc, r.host.Name, timeLength)
	}

	r.logger.Debugf("Total graphs initialized for host %s: %d", r.host.Name, len(r.graphs))
}
