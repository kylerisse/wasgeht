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
	name      string
	checkTyp  string            // check type name, used for file naming
	metrics   []check.MetricDef // metrics stored as data sources in this RRD
	descLabel string            // descriptor-level label for graph title/axis (may be empty)
	file      *os.File          // Pointer to the actual RRD file
	mutex     *sync.RWMutex     // Wrap file access
	graphs    []*graph
	logger    *logrus.Logger
	graphDir  string
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
//   - descLabel: Descriptor-level label for graph title/axis (may be empty).
//   - logger: The logger instance.
func NewRRD(name string, rrdDir string, graphDir string, checkType string, metrics []check.MetricDef, descLabel string, logger *logrus.Logger) (*RRD, error) {
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
		name:      name,
		checkTyp:  checkType,
		metrics:   metrics,
		descLabel: descLabel,
		file:      file,
		mutex:     &sync.RWMutex{},
		graphs:    []*graph{},
		logger:    logger,
		graphDir:  graphDir,
	}

	rrd.initGraphs()

	logger.Debugf("RRD struct initialized for %s check type %s with %d data source(s).", name, checkType, len(metrics))
	return rrd, nil
}

// getLastUpdate retrieves the timestamp of the last update from the RRD file.
// It returns the Unix timestamp of the most recent entry.
func (r *RRD) getLastUpdate() (int64, error) {

	r.logger.Debugf("Getting last update time for RRD file %s.", r.file.Name())

	cmd := exec.Command("rrdtool", "lastupdate", r.file.Name())
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to execute rrdtool lastupdate: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected output format from rrdtool lastupdate")
	}

	lastLine := lines[len(lines)-1]
	parts := strings.SplitN(lastLine, ":", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected format in the last line: %s", lastLine)
	}

	timestampStr := strings.TrimSpace(parts[0])
	timestampUnix, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse timestamp %q: %w", timestampStr, err)
	}

	return timestampUnix, nil
}

// SafeUpdate updates the RRD file with the provided values at the given timestamp.
// It checks if the new timestamp is newer than the last update to avoid duplicates.
// Values are pre-formatted strings; use "U" for UNKNOWN/NaN on missing metrics.
// Returns the Unix timestamp of the update, or an error.
func (r *RRD) SafeUpdate(t time.Time, values []string) (int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	timestampUnix := t.Unix()

	if len(values) > 0 {
		lastUpdate, err := r.getLastUpdate()
		if err != nil {
			return 0, fmt.Errorf("failed to get last update time: %w", err)
		}

		if timestampUnix <= lastUpdate {
			return 0, fmt.Errorf("new timestamp %d is not newer than last update %d", timestampUnix, lastUpdate)
		}

		// Build the update string: timestamp:val1:val2:...
		parts := make([]string, len(values)+1)
		parts[0] = strconv.FormatInt(timestampUnix, 10)
		copy(parts[1:], values)
		updateStr := strings.Join(parts, ":")

		cmd := exec.Command("rrdtool", "update", r.file.Name(), updateStr)

		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("failed to update RRD file %s with rrdtool: %w", r.file.Name(), err)
		}

		r.logger.Debugf("RRD file %s updated successfully.", r.file.Name())
	}

	for _, graph := range r.graphs {
		if time.Since(graph.lastDrawn) < graph.drawInterval {
			continue
		}
		if err := graph.draw(); err != nil {
			r.logger.Errorf("Failed to draw graph for RRD file %s: %v", r.file.Name(), err)
			continue
		}
		graph.lastDrawn = time.Now()
	}

	return timestampUnix, nil
}

// initGraphs initializes a list of graphs for different time lengths and consolidation functions.
func (r *RRD) initGraphs() {
	type graphSpec struct {
		conFunc  string
		interval time.Duration
	}

	specs := map[string]graphSpec{
		"15m": {"MAX", 1 * time.Minute},
		"1h":  {"MAX", 1 * time.Minute},
		"4h":  {"MAX", 5 * time.Minute},
		"8h":  {"MAX", 5 * time.Minute},
		"1d":  {"AVERAGE", 10 * time.Minute},
		"4d":  {"AVERAGE", 30 * time.Minute},
		"1w":  {"AVERAGE", 30 * time.Minute},
		"31d": {"AVERAGE", 1 * time.Hour},
		"93d": {"AVERAGE", 1 * time.Hour},
		"1y":  {"AVERAGE", 6 * time.Hour},
		"2y":  {"AVERAGE", 6 * time.Hour},
		"5y":  {"AVERAGE", 6 * time.Hour},
	}

	for timeLength, spec := range specs {
		graph, err := newGraph(r.name, r.graphDir, r.file.Name(), timeLength, spec.conFunc, r.checkTyp, r.metrics, r.descLabel, spec.interval, r.logger)
		if err != nil {
			r.logger.Errorf("Failed to create %s graph for %s with time length %s: %v", spec.conFunc, r.name, timeLength, err)
			continue
		}
		r.graphs = append(r.graphs, graph)
		r.logger.Debugf("Added %s graph for %s with time length %s.", spec.conFunc, r.name, timeLength)
	}

	r.logger.Debugf("Total graphs initialized for %s: %d", r.name, len(r.graphs))
}
