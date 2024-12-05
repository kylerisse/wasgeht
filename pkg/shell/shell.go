// Package shell provides utilities to execute shell commands with timeout support.
package shell

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RunCommand executes a shell command with the specified arguments and enforces a timeout.
// It returns the combined standard output and standard error of the command execution.
//
// Parameters:
// - command: The command and arguments to run
// - timeout: The maximum duration to allow for the command's execution before timing out.
//
// Returns:
// - A byte slice containing the combined standard output and standard error produced by the command.
// - An error if the command fails to start, encounters an execution error, or if the context times out.
//
// Usage Example:
//
//	package main
//
//	import (
//		"fmt"
//		"log"
//		"time"
//
//		"github.com/kylerisse/wasgeht/pkgs/shell"
//	)
//
//	func main() {
//		output, err := shell.RunCommand([]string{"echo", "Hello, World!"}, 2*time.Second)
//		if err != nil {
//			log.Fatalf(err)
//		}
//		fmt.Printf("Command Output: %s\n", output)
//	}
//
//	// Output:
//	// Command Output: Hello, World!
func RunCommand(command []string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command '%s' timed out after %v", joinCmd(command), timeout)
	}

	if err != nil {
		return output, fmt.Errorf("failed to execute command '%s %s': %w\nOutput: %s", command, joinCmd(command), err, string(output))
	}

	return output, nil
}

// joinCmd concatenates command and arguments into a single string for better error messages.
func joinCmd(command []string) string {
	return strings.Join(command, " ")
}
