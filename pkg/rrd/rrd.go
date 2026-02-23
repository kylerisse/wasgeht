package rrd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/sirupsen/logrus"
)

// RRD represents an RRD file, including metadata and synchronization tools.
// It contains the file pointer, a mutex for thread safety, metric definitions,
// and graph instances for visualization.
type RRD struct {
	name       string
	checkTyp   string            // check type name, used for file naming
	metrics    []check.MetricDef // metrics stored as data sources in this RRD
	graphStyle string            // graph rendering style ("line" or "stack"/empty)
	descLabel  string            // descriptor-level label for graph title/axis (may be empty)
	file       *os.File          // Pointer to the actual RRD file
	mutex      *sync.RWMutex     // Wrap file access
	graphs     []*graph
	logger     *logrus.Logger
	graphDir   string
}

// NewRRD creates and initializes a new RRD struct for the specified name.
// If the specified RRD file does not exist, it will be created using rrdtool
// with one data source per metric in the provided slice.
//
// RRD files are stored under {rrdDir}/{name}/{checkType}.rrd and graphs under {graphDir}/imgs/{name}/.
//
// Parameters:
//   - name: The identifier (typically host name) for which the RRD file will be created.
//   - rrdDir: The directory where the RRD file should be stored.
//   - graphDir: The directory where the graphs should be stored.
//   - checkType: The check type name, used for the RRD filename (e.g. "ping").
//   - metrics: The metric definitions describing the data sources to create.
//   - graphStyle: The graph rendering style from the check Descriptor.
//   - logger: The logger instance.
//
// Returns:
//   - *RRD: A pointer to the newly created RRD struct.
//   - error: An error if something went wrong during the initialization or creation of the RRD file.
func NewRRD(name string, rrdDir string, graphDir string, checkType string, metrics []check.MetricDef, graphStyle string, descLabel string, logger *logrus.Logger) (*RRD, error) {
	if len(metrics) == 0 {
		return nil, fmt.Errorf("at least one metric definition is required")
	}

	// verify rrdDir exists
	if _, err := os.Stat(rrdDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", rrdDir)
	}

	// Create per-name subdirectory under rrdDir
	nameDir := fmt.Sprintf("%s/%s", rrdDir, name)
	if err := os.MkdirAll(nameDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", nameDir, err)
	}

	// Construct the RRD file path: {rrdDir}/{name}/{checkType}.rrd
	rrdPath := fmt.Sprintf("%s/%s.rrd", nameDir, checkType)
	logger.Debugf("RRD path for %s check type %s: %s", name, checkType, rrdPath)

	if _, err := os.Stat(rrdPath); os.IsNotExist(err) {
		logger.Debugf("RRD file %s does not exist. Creating new RRD file.", rrdPath)

		// Build rrdtool create args with one DS per metric
		args := []string{
			"create", rrdPath,
			"--step", "60",
		}
		for _, m := range metrics {
			args = append(args, fmt.Sprintf("DS:%s:GAUGE:120:0:U", m.DSName))
		}
		args = append(args,
			"RRA:MAX:0.5:1:10080",      // 1-minute max for 1 week (10080 data points)
			"RRA:AVERAGE:0.5:1:10080",  // 1-minute average for 1 week (10080 data points)
			"RRA:AVERAGE:0.5:5:8928",   // 5-minute average for 31 days (8928 data points)
			"RRA:AVERAGE:0.5:15:8736",  // 15-minute average for 13 weeks (8736 data points)
			"RRA:AVERAGE:0.5:60:8784",  // 1-hour average for 1 year (8784 data points)
			"RRA:AVERAGE:0.5:480:5490", // 8-hour average for 5 years (5490 data points)
		)

		cmd := exec.Command("rrdtool", args...)
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
		name:       name,
		checkTyp:   checkType,
		metrics:    metrics,
		graphStyle: graphStyle,
		descLabel:  descLabel,
		file:       file,
		mutex:      &sync.RWMutex{},
		graphs:     []*graph{},
		logger:     logger,
		graphDir:   graphDir,
	}

	rrd.initGraphs()

	logger.Debugf("RRD struct initialized for %s check type %s with %d data source(s).", name, checkType, len(metrics))
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
	// Format: "timestamp: val1 val2 val3" or "timestamp:val1:val2:val3"
	lastLine := lines[len(lines)-1]
	parts := strings.SplitN(lastLine, ":", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected format in the last line: %s", lastLine)
	}

	// Trim any extra spaces and convert the timestamp to int64.
	lastUpdateStr := strings.TrimSpace(parts[0])
	lastUpdate, err := strconv.ParseInt(lastUpdateStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse last update timestamp: %w", err)
	}

	// Check if any value column contains valid data.
	valuePart := strings.TrimSpace(parts[1])
	values := strings.Fields(valuePart)
	allUndefined := true
	for _, v := range values {
		if v != "U" && v != "-nan" {
			allUndefined = false
			break
		}
	}

	if allUndefined {
		r.logger.Debugf("RRD file %s has no defined values yet (all U/-nan), returning 0.", r.file.Name())
		return 0, nil
	}

	r.logger.Debugf("Last update time for RRD file %s: %d", r.file.Name(), lastUpdate)
	return lastUpdate, nil
}

// SafeUpdate safely updates the RRD file with the specified timestamp and values.
// It locks the RRD file for thread safety, checks the update timestamp is newer
// than the last, and then performs the update.
//
// Returns the Unix timestamp of the update and any error encountered.
func (r *RRD) SafeUpdate(timestamp time.Time, values []int64) (int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	timestampUnix := timestamp.Unix()

	// Check if the timestamp is newer than the last update.
	lastUpdate, err := r.getLastUpdate()
	if err != nil {
		return 0, fmt.Errorf("failed to get last update for RRD file %s: %w", r.file.Name(), err)
	}

	if lastUpdate > 0 && timestampUnix <= lastUpdate {
		r.logger.Debugf("Skipping update for RRD file %s: timestamp %d is not newer than last update %d.", r.file.Name(), timestamp.Unix(), lastUpdate)
		return 0, fmt.Errorf("skipping update as timestamp %d is not newer than last update %d", timestamp.Unix(), lastUpdate)
	}

	if len(values) > 0 {
		// Prepare the update string: "<timestamp>:<value1>:<value2>:..."
		updateStr := fmt.Sprintf("%d", timestamp.Unix())
		for _, value := range values {
			updateStr += fmt.Sprintf(":%d", value)
		}

		r.logger.Debugf("Updating RRD file %s with update string: %s", r.file.Name(), updateStr)

		// Execute the "rrdtool update" command to add the new data point.
		cmd := exec.Command("rrdtool", "update", r.file.Name(), updateStr)

		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("failed to update RRD file %s with rrdtool: %w", r.file.Name(), err)
		}

		r.logger.Debugf("RRD file %s updated successfully.", r.file.Name())
	}

	for _, graph := range r.graphs {
		err := graph.draw()
		if err != nil {
			r.logger.Errorf("Failed to draw graph for RRD file %s: %v", r.file.Name(), err)
		}
	}

	return timestampUnix, nil
}

// initGraphs initializes a list of graphs for different time lengths and consolidation functions.
// This method adds multiple graphs (e.g., hourly, daily, weekly, etc.) to the RRD.
func (r *RRD) initGraphs() {
	// Define the map of time lengths and consolidation functions for each graph.
	timeLengths := map[string]string{
		"15m": "MAX",
		"1h":  "MAX",
		"4h":  "MAX",
		"8h":  "MAX",
		"1d":  "AVERAGE",
		"4d":  "AVERAGE",
		"1w":  "AVERAGE",
		"31d": "AVERAGE",
		"93d": "AVERAGE",
		"1y":  "AVERAGE",
		"2y":  "AVERAGE",
		"5y":  "AVERAGE",
	}

	// Loop over each time length to create graphs with specified consolidation function.
	for timeLength, conFunc := range timeLengths {
		graph, err := newGraph(r.name, r.graphDir, r.file.Name(), timeLength, conFunc, r.checkTyp, r.metrics, r.graphStyle, r.descLabel, r.logger)
		if err != nil {
			r.logger.Errorf("Failed to create %s graph for %s with time length %s: %v", conFunc, r.name, timeLength, err)
			continue
		}
		r.graphs = append(r.graphs, graph)
		r.logger.Debugf("Added %s graph for %s with time length %s.", conFunc, r.name, timeLength)
	}

	r.logger.Debugf("Total graphs initialized for %s: %d", r.name, len(r.graphs))
}
