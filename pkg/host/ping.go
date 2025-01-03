package host

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Ping sends an ICMP Echo Request to the host and returns the round-trip time in milliseconds
func (h *Host) Ping(name string, timeout time.Duration) (time.Duration, error) {
	// Determine the target
	target := h.Address
	if target == "" {
		target = name
	}

	// Prepare the ping command for Unix-like systems
	count := "1"
	timeoutSec := fmt.Sprintf("%.0f", timeout.Seconds())
	cmd := exec.Command("ping", "-c", count, "-W", timeoutSec, target)

	// Execute the ping command
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("ping failed for %s: %v", target, err)
	}

	// Parse the output to extract the round-trip time
	rtt, err := parsePingOutput(out.String())
	if err != nil {
		return 0, fmt.Errorf("failed to parse ping output for %s: %v", target, err)
	}

	return rtt, nil
}

// parsePingOutput extracts the round-trip time (RTT) from the ping command's output
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
