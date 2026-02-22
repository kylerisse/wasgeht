package rrd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/sirupsen/logrus"
)

// Standard colors for graph elements, cycled for multi-metric graphs.
var stackColors = []string{
	GREEN,     // first metric
	BLUE,      // second
	ORANGE,    // third
	VIOLET,    // fourth
	TURQUOISE, // fifth
}

// graph represents an RRD graph, including metadata and synchronization tools.
type graph struct {
	rrdPath               string            // Path to supporting RRD
	filePath              string            // Path to the graph output file
	title                 string            // Title of the graph
	timeLength            string            // Time length for the graph (e.g., "4h" "1d")
	metrics               []check.MetricDef // Metrics to draw (one per DS in the RRD)
	consolidationFunction string            // Consolidation function (e.g., "AVERAGE" "MAX")
	logger                *logrus.Logger
}

// newGraph creates and initializes a new graph struct.
//
// Parameters:
//   - host: The name of the host.
//   - graphDir: The path to the graphs directory.
//   - rrdPath: The path to the RRD file.
//   - timeLength: The time range for the graph (e.g., "4h").
//   - consolidationFunction: The RRD consolidation function ("AVERAGE", "MAX", etc.).
//   - checkType: The check type name, used for graph file naming (e.g., "ping").
//   - metrics: The metric definitions for data sources in the RRD.
//   - logger: The logger instance.
//
// Returns:
//   - *graph: A pointer to the newly created graph struct.
//   - error: An error if something went wrong during the initialization.
func newGraph(host string, graphDir string, rrdPath string, timeLength string, consolidationFunction string, checkType string, metrics []check.MetricDef, logger *logrus.Logger) (*graph, error) {

	// Define directory and file paths
	dirPath := fmt.Sprintf("%s/imgs/%s", graphDir, host)
	filePath := fmt.Sprintf("%s/%s_%s_%s.png", dirPath, host, checkType, timeLength)

	// Ensure the directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Build title from the first metric's label (shared unit across all metrics)
	label := metrics[0].Label
	title := fmt.Sprintf("%s %s over the last %s", host, label, expandTimeLength(timeLength))

	g := &graph{
		rrdPath:               rrdPath,
		filePath:              filePath,
		title:                 title,
		timeLength:            timeLength,
		metrics:               metrics,
		consolidationFunction: consolidationFunction,
		logger:                logger,
	}

	logger.Debugf("Initializing graph for host %s, check type %s, time length %s.", host, checkType, timeLength)
	err := g.draw()
	if err != nil {
		return g, err
	}
	logger.Debugf("Graph initialized and drawn for host %s, check type %s, time length %s.", host, checkType, timeLength)
	return g, nil
}

// displayVarName returns the RRD variable name used for display of a given metric.
// When scaling is applied, the variable includes the unit suffix (e.g., "latency_ms").
// When no scaling is needed, it uses the raw variable directly (e.g., "latency_raw").
func displayVarName(m check.MetricDef) string {
	if needsScaling(m) {
		return fmt.Sprintf("%s_%s", m.DSName, m.Unit)
	}
	return fmt.Sprintf("%s_raw", m.DSName)
}

// needsScaling returns true if the raw value needs to be divided by a scale factor for display.
func needsScaling(m check.MetricDef) bool {
	return m.Scale > 1
}

// draw draws a graph based on the current parameters of the graph struct.
// For single-metric checks, it draws a single AREA.
// For multi-metric checks, it draws stacked AREAs (first AREA, rest STACK).
// It returns an error if the graph generation fails.
func (g *graph) draw() error {
	// Shared unit and label come from the first metric (all metrics in a
	// check share the same unit by convention).
	unit := g.metrics[0].Unit
	label := g.metrics[0].Label

	var defs []string
	var cdefs []string
	var lines []string
	var gprints []string

	for i, m := range g.metrics {
		rawVar := fmt.Sprintf("%s_raw", m.DSName)
		dispVar := displayVarName(m)
		color := stackColors[i%len(stackColors)]

		// DEF: read the raw data source from the RRD file
		defs = append(defs, fmt.Sprintf("DEF:%s=%s:%s:%s", rawVar, g.rrdPath, m.DSName, g.consolidationFunction))

		// CDEF: apply scaling if needed
		if needsScaling(m) {
			cdefs = append(cdefs, fmt.Sprintf("CDEF:%s=%s,%d,/", dispVar, rawVar, m.Scale))
		}

		// AREA for first metric, STACK for subsequent (creates stacked graph)
		if i == 0 {
			lines = append(lines, fmt.Sprintf("AREA:%s#%s:%s", dispVar, color, m.Label))
		} else {
			lines = append(lines, fmt.Sprintf("STACK:%s#%s:%s", dispVar, color, m.Label))
		}

		// GPRINT stats for each metric
		gfmt := "%.2lf"
		gprints = append(gprints,
			fmt.Sprintf("GPRINT:%s:LAST:  %s last\\: %s %s", dispVar, m.Label, gfmt, unit),
		)
	}

	// For single-metric graphs, include the full stats line (backward compatible)
	if len(g.metrics) == 1 {
		dispVar := displayVarName(g.metrics[0])
		gfmt := "%.2lf"
		// Replace the simple GPRINT with the detailed one
		gprints = []string{
			fmt.Sprintf("GPRINT:%s:MIN:Min\\: %s %s", dispVar, gfmt, unit),
			fmt.Sprintf("GPRINT:%s:MAX:Max\\: %s %s", dispVar, gfmt, unit),
			fmt.Sprintf("GPRINT:%s:AVERAGE:Average\\: %s %s", dispVar, gfmt, unit),
			fmt.Sprintf("GPRINT:%s:LAST:Last\\: %s %s", dispVar, gfmt, unit),
		}
	}

	comment := fmt.Sprintf("%s %s over last %s", g.consolidationFunction, label, g.timeLength)
	commentStrings := []string{
		"COMMENT:\\n",
		fmt.Sprintf("COMMENT:%s", comment),
	}

	// Use the shared label for vertical axis
	verticalLabel := fmt.Sprintf("%s (%s)", label, unit)

	// Prepare the command for generating the graph.
	args := []string{
		g.filePath,
		"--start", fmt.Sprintf("-%s", g.timeLength),
		"--title", g.title,
		"--vertical-label", verticalLabel,
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
