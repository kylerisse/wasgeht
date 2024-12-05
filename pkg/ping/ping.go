package ping

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/graph"
	"github.com/kylerisse/wasgeht/pkg/rrd"
	"github.com/kylerisse/wasgeht/pkg/shell"
)

// PingCheck performs a ping to a target host and records the latency.
type PingCheck struct {
	target string
	result check.Result
	mutex  sync.RWMutex
	rrd    *rrd.RRD
	graphs []*graph.Graph
}

// NewPingCheck initializes a new PingCheck for the specified target host.
//
// It returns a pointer to a PingCheck with an initial unknown status and zero latency.
func NewPingCheck(target string) *PingCheck {
	result := check.Result{
		Status: check.Unknown,
		Metrics: check.Metrics{
			"latency": 0,
		},
		LastUpdated: time.Unix(0, 0),
	}
	return &PingCheck{
		target: target,
		result: result,
		mutex:  sync.RWMutex{},
	}
}

// Name returns the name of the check, which is "ping".
func (p *PingCheck) Name() string {
	return "ping"
}

// Result returns the latest Result for the ping operation.
//
// It acquires a read lock to ensure thread-safe access to the result.
func (p *PingCheck) Result() check.Result {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.result
}

// GraphPaths returns a list of all the graph paths.
func (p *PingCheck) GraphPaths() []string {
	var paths []string
	for _, graph := range p.graphs {
		paths = append(paths, graph.UrlPath)
	}
	return paths
}

// Run executes the ping command to the target host and updates the Result.
//
// It returns the updated Result and an error if the ping fails.
func (p *PingCheck) Run() (check.Result, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	timeout := 5 * time.Second
	pingCmd := fmt.Sprintf("ping -c 1 -W %.0f %s", timeout.Seconds(), p.target)
	output, err := shell.RunCommand(strings.Split(" ", pingCmd), timeout)
	if err != nil {
		p.result = failedResult(p.result)
		return p.result, err
	}
	latency, err := parsePingOutput(string(output))
	if err != nil {
		p.result = failedResult(p.result)
		return p.result, err
	}
	p.result = check.Result{
		Status: check.Healthy,
		Metrics: check.Metrics{
			"latency": int64(latency),
		},
		LastUpdated: time.Now(),
	}
	return p.result, nil
}

// failedResult updates the Result to an error status if it's currently unknown.
//
// It preserves the existing metrics and last updated time.
func failedResult(current check.Result) check.Result {
	if current.Status != check.Unknown {
		return current
	}
	return check.Result{
		Status:      check.Error,
		Metrics:     current.Metrics,
		LastUpdated: current.LastUpdated,
	}
}

// parsePingOutput extracts the round-trip time (RTT) from the ping command's output.
//
// It returns the latency as a time.Duration or an error if parsing fails.
func parsePingOutput(output string) (time.Duration, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "time=") {
			// Unix-like output: time=14.1 ms
			start := strings.Index(line, "time=") + len("time=")
			end := strings.IndexAny(line[start:], " ")
			if end == -1 {
				end = len(line[start:])
			}
			rttStr := line[start : start+end]

			unitStart := start + end
			unit := strings.TrimSpace(line[unitStart:])

			rtt, err := strconv.ParseFloat(strings.TrimSpace(rttStr), 64)
			if err != nil {
				return 0, fmt.Errorf("could not parse RTT: %s", rttStr)
			}

			// determine rtt unit and return proper time type
			if strings.Contains(unit, "ms") {
				// return in milliseconds
				return time.Duration(rtt * float64(time.Millisecond)), nil
			} else if strings.Contains(unit, "us") || strings.Contains(unit, "µs") {
				// return in microseconds
				return time.Duration(rtt * float64(time.Microsecond)), nil
			} else if strings.Contains(unit, "s") {
				// return in seconds
				return time.Duration(rtt * float64(time.Second)), nil
			}

			// Error out if we can't figure out the time unit
			return 0, fmt.Errorf("could not determine time unit of RTT")

		}
	}
	return 0, fmt.Errorf("RTT not found in ping output")
}
