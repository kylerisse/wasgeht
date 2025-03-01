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
		return "one week"
	case "1m":
		return "one month"
	case "1y":
		return "one year"
	}
	return timeLength
}
