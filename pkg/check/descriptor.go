package check

// MetricDef describes a single metric produced by a check type.
type MetricDef struct {
	// ResultKey is the key used in Result.Metrics (e.g. "latency_us").
	ResultKey string

	// DSName is the RRD data source name (e.g. "latency").
	DSName string

	// Label is a human-readable label for graphs and display (e.g. "latency").
	Label string

	// Unit is the unit of measurement for graphs and display (e.g. "ms").
	Unit string
}

// Descriptor declares static metadata about a check type, including
// what metrics it produces. This is registered alongside the Factory
// so the system can generically wire up storage and graphs without
// per-type knowledge.
type Descriptor struct {
	// Metrics lists the metrics this check type produces.
	Metrics []MetricDef
}
