package check

// CheckMetrics holds various metrics collected by a check.
// The key represents the metric name, and the value represents the metric value.
// Depending on the metric, the value type may vary. Currently using float64 for flexibility.
type CheckMetrics map[string]int64
