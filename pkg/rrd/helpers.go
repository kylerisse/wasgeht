package rrd

// Standard colors for RRD graph elements.
const RED = "FF0000"
const GREEN = "00FF00"
const BLUE = "0000FF"
const ORANGE = "FF8C00"
const VIOLET = "9400D3"
const TURQUOISE = "00CED1"

// expandTimeLength converts a short time duration code (e.g. "15m", "1h", "4d")
// into a human-readable string for use in graph titles and comments.
// Returns the input unchanged if it does not match a known code.
func expandTimeLength(timeLength string) string {
	switch timeLength {
	case "15m":
		return "fifteen minutes"
	case "1h":
		return "one hour"
	case "4h":
		return "four hours"
	case "8h":
		return "eight hours"
	case "1d":
		return "one day"
	case "4d":
		return "four days"
	case "1w":
		return "week"
	case "31d":
		return "month"
	case "93d":
		return "quarter"
	case "1y":
		return "year"
	case "2y":
		return "two years"
	case "5y":
		return "five years"
	}
	return timeLength
}
