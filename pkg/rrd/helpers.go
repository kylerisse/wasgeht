package rrd

const RED = "FF0000"
const GREEN = "00FF00"

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
