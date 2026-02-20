package rrd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// graph represents an RRD graph, including metadata and synchronization tools.
// It contains a pointer to the file, a mutex for thread safety, a title, time range, metrics, and unit of measurement.
type graph struct {
	rrdPath               string // Path to supporting RRD
	filePath              string // Pointer to the graph output file
	title                 string // Label of the graph
	label                 string // Label of the graph
	timeLength            string // Time length for the graph (e.g., "4h" "1d")
	dsName                string // RRD data source name (e.g., "latency")
	unit                  string // Unit of measurement (e.g., "ms")
	scale                 int    // Divisor to convert raw value to display unit (0 or 1 = no scaling)
	consolidationFunction string // Consolidation function (e.g., "AVERAGE" "MAX")
	color                 string // Metric color (e.g., "#FF0001" (red))
	comment               string // Comment at bottom of graph
	logger                *logrus.Logger
}

// newGraph creates and initializes a new Graph struct.
//
// Parameters:
//   - host: The name of the host.
//   - graphDir: The path to the graphs directory.
//   - rrdPath: The path to the RRD file.
//   - timeLength: The time range for the graph (e.g., "4h").
//   - consolidationFunction: The RRD consolidation function ("AVERAGE", "MAX", etc.).
//   - checkType: The check type name, used for graph file naming (e.g., "ping").
//   - dsName: The RRD data source name (e.g., "latency").
//   - label: The human-readable label for the metric (e.g., "latency").
//   - unit: The unit of measurement (e.g., "ms").
//   - scale: The divisor to convert the raw stored value to display unit (0 or 1 = no scaling).
//   - logger: The logger instance.
//
// Returns:
//   - *Graph: A pointer to the newly created Graph struct.
//   - error: An error if something went wrong during the initialization.
func newGraph(host string, graphDir string, rrdPath string, timeLength string, consolidationFunction string, checkType string, dsName string, label string, unit string, scale int, logger *logrus.Logger) (*graph, error) {

	// Define directory and file paths
	dirPath := fmt.Sprintf("%s/imgs/%s", graphDir, host)
	filePath := fmt.Sprintf("%s/%s_%s_%s.png", dirPath, host, checkType, timeLength)

	// Ensure the directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	title := fmt.Sprintf("%s %s over the last %s", host, label, expandTimeLength(timeLength))
	comment := fmt.Sprintf("%s %s over last %s", consolidationFunction, label, timeLength)

	graph := &graph{
		rrdPath:               rrdPath,
		filePath:              filePath,
		label:                 label,
		title:                 title,
		timeLength:            timeLength,
		dsName:                dsName,
		unit:                  unit,
		scale:                 scale,
		consolidationFunction: consolidationFunction,
		color:                 GREEN,
		comment:               comment,
		logger:                logger,
	}

	logger.Debugf("Initializing graph for host %s, check type %s, time length %s.", host, checkType, timeLength)
	err := graph.draw()
	if err != nil {
		return graph, err
	}
	logger.Debugf("Graph initialized and drawn for host %s, check type %s, time length %s.", host, checkType, timeLength)
	return graph, nil
}

// displayVarName returns the RRD variable name used for display.
// When scaling is applied, the variable includes the unit suffix (e.g., "latency_ms").
// When no scaling is needed, it uses the raw variable directly (e.g., "latency_raw").
func (g *graph) displayVarName() string {
	if g.needsScaling() {
		return fmt.Sprintf("%s_%s", g.dsName, g.unit)
	}
	return fmt.Sprintf("%s_raw", g.dsName)
}

// needsScaling returns true if the raw value needs to be divided by a scale factor for display.
func (g *graph) needsScaling() bool {
	return g.scale > 1
}

// draw draws a graph based on the current parameters of the Graph struct.
// It returns an error if the graph generation fails.
func (g *graph) draw() error {

	/*
		rrdtool graph example_latency_4h.png \
			--start -4h \
			--title "Latency Over the Last 4 Hours" \
			--vertical-label "latency (ms)" \
			--width 800 \
			--height 200 \
			--lower-limit 0 \
			--rigid \
			DEF:latency_raw=rrds/example2.rrd:latency:MAX \
			CDEF:latency_ms=latency_raw,1000000,/ \
			LINE2:latency_ms#FF0000:"latency (ms)" \
			GPRINT:latency_ms:MIN:"Min\: %.2lf ms" \
			GPRINT:latency_ms:MAX:"Max\: %.2lf ms" \
			GPRINT:latency_ms:AVERAGE:"Average\: %.2lf ms" \
			GPRINT:latency_ms:LAST:"Last\: %.2lf ms" \
			COMMENT:"\n" \
			COMMENT:"\MAX latency over last 4h"
	*/

	displayVar := g.displayVarName()

	// Prepare the DEF string for the raw data source.
	defs := []string{
		fmt.Sprintf("DEF:%s_raw=%s:%s:%s", g.dsName, g.rrdPath, g.dsName, g.consolidationFunction),
	}

	// Prepare the CDEF string: apply scaling if needed, otherwise alias raw to display var.
	var cdefs []string
	if g.needsScaling() {
		cdefs = append(cdefs, fmt.Sprintf("CDEF:%s=%s_raw,%d,/", displayVar, g.dsName, g.scale))
	}
	// When no scaling is needed, displayVar is already "dsName_raw" which is the DEF name,
	// so no CDEF is required.

	lines := []string{
		fmt.Sprintf("AREA:%s#%s:%s", displayVar, g.color, g.label),
	}

	gfmt := "%.2lf"
	gprints := []string{
		fmt.Sprintf("GPRINT:%s:MIN:Min\\: %s %s", displayVar, gfmt, g.unit),
		fmt.Sprintf("GPRINT:%s:MAX:Max\\: %s %s", displayVar, gfmt, g.unit),
		fmt.Sprintf("GPRINT:%s:AVERAGE:Average\\: %s %s", displayVar, gfmt, g.unit),
		fmt.Sprintf("GPRINT:%s:LAST:Last\\: %s %s", displayVar, gfmt, g.unit),
	}

	commentStrings := []string{
		"COMMENT:\\n",
		fmt.Sprintf("COMMENT:%s", g.comment),
	}

	// Prepare the command for generating the graph.
	args := []string{
		g.filePath,
		"--start", fmt.Sprintf("-%s", g.timeLength),
		"--title", g.title,
		"--vertical-label", g.label,
		"--width", "800",
		"--height", "200",
		"--lower-limit", "0",
	}
	if g.consolidationFunction == "MAX" {
		args = append(args, "--rigid")
	}
	args = append(args, defs...)
	args = append(args, cdefs...)
	args = append(args, lines...)
	args = append(args, gprints...)
	args = append(args, commentStrings...)

	g.logger.Debugf("Generating graph with command arguments: %v", args)

	// Execute the "rrdtool graph" command to generate the graph.
	cmd := exec.Command("rrdtool", append([]string{"graph"}, args...)...)
	if err := cmd.Run(); err != nil {
		g.logger.Errorf("Failed to update graph %s: %v", g.filePath, err)
		return fmt.Errorf("failed to update graph %s: %w", g.filePath, err)
	}

	g.logger.Debugf("Graph %s generated successfully.", g.filePath)
	return nil
}
