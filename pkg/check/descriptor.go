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

	// Scale is the divisor applied to convert the raw stored value to
	// the display unit. For example, ping stores microseconds but
	// displays milliseconds, so Scale is 1000.
	// A value of 0 or 1 means no scaling is applied.
	Scale int
}

// Descriptor declares metadata about a check instance, including what
// metrics it produces. Each check instance returns its own Descriptor
// via Check.Describe(), allowing config-dependent metric shapes (e.g.
// a wifi_stations check with a variable number of radios).
type Descriptor struct {
	// Label is a human-readable label used for graph titles and the
	// vertical axis. If empty, the first metric's Label is used.
	// Useful when individual metric labels are long or not suitable
	// for titles (e.g. full URLs).
	Label string

	// Metrics lists the metrics this check instance produces.
	Metrics []MetricDef
}
