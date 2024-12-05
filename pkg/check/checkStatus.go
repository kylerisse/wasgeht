package check

// CheckStatus represents the status of a check.
type CheckStatus int

const (
	// Unknown indicates the check has not been run yet or the status is indeterminate.
	Unknown CheckStatus = iota
	// Error indicates the check encountered a critical failure.
	Error
	// Warning indicates the check encountered a non-critical issue.
	Warning
	// Healthy indicates the check passed successfully.
	Healthy
)

// String returns the string representation of the CheckStatus.
// It satisfies the fmt.Stringer interface.
func (cs CheckStatus) String() string {
	switch cs {
	case Unknown:
		return "Unknown"
	case Error:
		return "Error"
	case Warning:
		return "Warning"
	case Healthy:
		return "Healthy"
	default:
		return "Invalid Status"
	}
}
