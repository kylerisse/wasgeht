// pkg/checks/check.go
package check

// Check defines the behavior that all check types must implement.
type Check interface {
	// Name returns the name of the check.
	Name() string

	// Run executes the check against the provided host.
	// It performs the necessary actions and updates the internal state with the result.
	// The context allows for cancellation and timeouts.
	// Returns an error if the execution fails.
	Run() (Result, error)

	// Result returns the latest result of the check execution.
	// It provides access to the status, metrics, and last update time.
	Result() Result

	// GraphPaths returns a list of paths for all graphs related to the check
	GraphPaths() []string
}
