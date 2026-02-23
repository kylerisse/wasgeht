package rrd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/sirupsen/logrus"
)

// lineColors are cycled for multi-metric line graphs.
var lineColors = []string{
	GREEN,
	BLUE,
	ORANGE,
	VIOLET,
	TURQUOISE,
}

// graph represents an RRD graph, including metadata and synchronization tools.
type graph struct {
	rrdPath               string            // Path to supporting RRD
	filePath              string            // Path to the graph output file
	title                 string            // Title of the graph
	timeLength            string            // Time length for the graph (e.g., "4h" "1d")
	metrics               []check.MetricDef // Metrics to draw (one per DS in the RRD)
	descLabel             string            // descriptor-level label override (may be empty)
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
//   - descLabel: Descriptor-level label override for graph title/axis (may be empty).
//   - logger: The logger instance.
func newGraph(host string, graphDir string, rrdPath string, timeLength string, consolidationFunction string, checkType string, metrics []check.MetricDef, descLabel string, logger *logrus.Logger) (*graph, error) {

	// Define directory and file paths
	dirPath := fmt.Sprintf("%s/imgs/%s", graphDir, host)
	filePath := fmt.Sprintf("%s/%s_%s_%s.png", dirPath, host, checkType, timeLength)

	// Ensure the directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Use the descriptor-level label if set, otherwise fall back to first metric's label
	label := descLabel
	if label == "" {
		label = metrics[0].Label
	}
	title := fmt.Sprintf("%s %s over the last %s", host, label, expandTimeLength(timeLength))

	g := &graph{
		rrdPath:               rrdPath,
		filePath:              filePath,
		title:                 title,
		timeLength:            timeLength,
		metrics:               metrics,
		descLabel:             descLabel,
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

// rrdEscape escapes a string for use in rrdtool graph labels and comments.
// rrdtool uses colons as field delimiters, so literal colons must be escaped
// as \: in label text. Backslashes must also be escaped.
func rrdEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `:`, `\:`)
	return s
}

// draw draws a graph based on the current parameters of the graph struct.
// All metrics are rendered as colored LINE2s.
// It returns an error if the graph generation fails.
func (g *graph) draw() error {
	// Shared unit comes from the first metric (all metrics in a check share
	// the same unit by convention). Label for axis/comment uses the
	// descriptor-level override if set.
	unit := g.metrics[0].Unit
	label := g.descLabel
	if label == "" {
		label = g.metrics[0].Label
	}

	var defs []string
	var cdefs []string
	var lines []string
	var gprints []string

	for i, m := range g.metrics {
		rawVar := fmt.Sprintf("%s_raw", m.DSName)
		dispVar := displayVarName(m)
		color := lineColors[i%len(lineColors)]
		escapedLabel := rrdEscape(m.Label)

		// DEF: read the raw data source from the RRD file
		defs = append(defs, fmt.Sprintf("DEF:%s=%s:%s:%s", rawVar, g.rrdPath, m.DSName, g.consolidationFunction))

		// CDEF: apply scaling if needed
		if needsScaling(m) {
			cdefs = append(cdefs, fmt.Sprintf("CDEF:%s=%s,%d,/", dispVar, rawVar, m.Scale))
		}

		// Draw each metric as a colored line
		lines = append(lines, fmt.Sprintf("LINE2:%s#%s:%s", dispVar, color, escapedLabel))

		// GPRINT stats for each metric
		gfmt := "%.2lf"
		gprints = append(gprints,
			fmt.Sprintf("GPRINT:%s:LAST:  %s last\\: %s %s", dispVar, escapedLabel, gfmt, unit),
		)
	}

	// For single-metric graphs, include the full stats line (backward compatible)
	if len(g.metrics) == 1 {
		dispVar := displayVarName(g.metrics[0])
		gfmt := "%.2lf"
		gprints = []string{
			fmt.Sprintf("GPRINT:%s:MIN:Min\\: %s %s", dispVar, gfmt, unit),
			fmt.Sprintf("GPRINT:%s:MAX:Max\\: %s %s", dispVar, gfmt, unit),
			fmt.Sprintf("GPRINT:%s:AVERAGE:Average\\: %s %s", dispVar, gfmt, unit),
			fmt.Sprintf("GPRINT:%s:LAST:Last\\: %s %s", dispVar, gfmt, unit),
		}
	}

	comment := fmt.Sprintf("%s %s over last %s", g.consolidationFunction, rrdEscape(label), g.timeLength)
	commentStrings := []string{
		"COMMENT:\\n",
		fmt.Sprintf("COMMENT:%s", comment),
	}

	// Use the shared label for vertical axis
	verticalLabel := fmt.Sprintf("%s (%s)", label, unit)

	// Prepare the command for generating the graph.
	args := []string{
		"graph", g.filePath,
		"--title", g.title,
		"--vertical-label", verticalLabel,
		"--start", fmt.Sprintf("now-%s", g.timeLength),
		"--end", "now",
		"--width", "800",
		"--height", "200",
	}

	args = append(args, defs...)
	args = append(args, cdefs...)
	args = append(args, lines...)
	args = append(args, gprints...)
	args = append(args, commentStrings...)

	cmd := exec.Command("rrdtool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rrdtool graph failed for %s: %w\nOutput: %s", g.filePath, err, string(output))
	}

	g.logger.Debugf("Graph drawn successfully: %s", g.filePath)
	return nil
}
