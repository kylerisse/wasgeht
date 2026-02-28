package rrd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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
	PINK,
	LIME,
	BROWN,
	TEAL,
	RED,
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
	drawInterval          time.Duration     // Minimum time between redraws
	lastDrawn             time.Time         // Time of last successful draw
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
func newGraph(host string, graphDir string, rrdPath string, timeLength string, consolidationFunction string, checkType string, metrics []check.MetricDef, descLabel string, drawInterval time.Duration, logger *logrus.Logger) (*graph, error) {

	dirPath := fmt.Sprintf("%s/imgs/%s", graphDir, host)
	filePath := fmt.Sprintf("%s/%s_%s_%s.png", dirPath, host, checkType, timeLength)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

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
		drawInterval:          drawInterval,
		logger:                logger,
	}

	logger.Debugf("Initializing graph for host %s, check type %s, time length %s.", host, checkType, timeLength)
	err := g.draw()
	if err != nil {
		return g, err
	}
	g.lastDrawn = time.Now()
	logger.Debugf("Graph initialized and drawn for host %s, check type %s, time length %s.", host, checkType, timeLength)
	return g, nil
}

// displayVarName returns the RRD variable name used for display of a given metric.
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
func rrdEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `:`, `\:`)
	return s
}

// draw draws a graph based on the current parameters of the graph struct.
// All metrics are rendered as colored LINE2s.
func (g *graph) draw() error {
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

		defs = append(defs, fmt.Sprintf("DEF:%s=%s:%s:%s", rawVar, g.rrdPath, m.DSName, g.consolidationFunction))

		if needsScaling(m) {
			cdefs = append(cdefs, fmt.Sprintf("CDEF:%s=%s,%d,/", dispVar, rawVar, m.Scale))
		}

		lines = append(lines, fmt.Sprintf("LINE2:%s#%s:%s", dispVar, color, escapedLabel))

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

	verticalLabel := fmt.Sprintf("%s (%s)", label, unit)

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
