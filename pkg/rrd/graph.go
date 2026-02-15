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
//   - logger: The logger instance.
//
// Returns:
//   - *Graph: A pointer to the newly created Graph struct.
//   - error: An error if something went wrong during the initialization.
func newGraph(host string, graphDir string, rrdPath string, timeLength string, consolidationFunction string, checkType string, dsName string, logger *logrus.Logger) (*graph, error) {

	// Define directory and file paths
	dirPath := fmt.Sprintf("%s/imgs/%s", graphDir, host)
	filePath := fmt.Sprintf("%s/%s_%s_%s.png", dirPath, host, checkType, timeLength)

	// Ensure the directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	label := "latency"
	title := fmt.Sprintf("%s %s over the last %s", host, "latency", expandTimeLength(timeLength))
	comment := fmt.Sprintf("%s %s over last %s", consolidationFunction, "latency", timeLength)

	graph := &graph{
		rrdPath:               rrdPath,
		filePath:              filePath,
		label:                 label,
		title:                 title,
		timeLength:            timeLength,
		dsName:                dsName,
		unit:                  "ms",
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

	// Prepare the DEF and CDEF strings for each metric.
	defs := []string{}
	def := fmt.Sprintf("DEF:%s_raw=%s:%s:%s", g.dsName, g.rrdPath, g.dsName, g.consolidationFunction)
	defs = append(defs, def)

	cdefs := []string{}
	cdef := fmt.Sprintf("CDEF:%s_%s=%s_raw,1000,/", g.dsName, g.unit, g.dsName)
	cdefs = append(cdefs, cdef)

	lines := []string{
		fmt.Sprintf("AREA:%s_%s#%s:%s", g.dsName, g.unit, g.color, g.label),
	}

	gprints := []string{}
	gfmt := "%.2lf"
	gprintsMinval := fmt.Sprintf("Min\\: %s %s", gfmt, g.unit)
	gprints = append(gprints, fmt.Sprintf("GPRINT:%s_%s:MIN:%s", g.dsName, g.unit, gprintsMinval))
	gprintsMaxval := fmt.Sprintf("Max\\: %s %s", gfmt, g.unit)
	gprints = append(gprints, fmt.Sprintf("GPRINT:%s_%s:MAX:%s", g.dsName, g.unit, gprintsMaxval))
	gprintsAverageval := fmt.Sprintf("Average\\: %s %s", gfmt, g.unit)
	gprints = append(gprints, fmt.Sprintf("GPRINT:%s_%s:AVERAGE:%s", g.dsName, g.unit, gprintsAverageval))
	gprintsLastval := fmt.Sprintf("Last\\: %s %s", gfmt, g.unit)
	gprints = append(gprints, fmt.Sprintf("GPRINT:%s_%s:LAST:%s", g.dsName, g.unit, gprintsLastval))

	commentStrings := []string{}
	commentStrings = append(commentStrings, "COMMENT:\\n")
	commentStrings = append(commentStrings, fmt.Sprintf("COMMENT:%s", g.comment))

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
