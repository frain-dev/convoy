package exporter

import (
	"fmt"
	"time"
)

const DefaultBackupInterval = time.Hour

// ParseBackupInterval parses a duration string into a time.Duration,
// falling back to DefaultBackupInterval on error or empty input.
func ParseBackupInterval(s string) time.Duration {
	if s == "" {
		return DefaultBackupInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return DefaultBackupInterval
	}
	return d
}

// DurationToCron converts a time.Duration into a cron spec string.
// Sub-hour durations produce minute-level cron (e.g. */5 * * * *).
// Hour-or-above durations produce hour-level cron (e.g. 0 */6 * * *).
func DurationToCron(d time.Duration) string {
	minutes := int(d.Minutes())
	switch {
	case minutes <= 0:
		return "5 * * * *" // fallback: hourly at :05
	case minutes < 60:
		return fmt.Sprintf("*/%d * * * *", minutes)
	case minutes == 60:
		return "5 * * * *" // hourly at :05
	default:
		hours := int(d.Hours())
		if hours <= 0 {
			hours = 1
		}
		return fmt.Sprintf("0 */%d * * *", hours)
	}
}
