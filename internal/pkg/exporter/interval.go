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
	return durationToCronWithOffset(d, 0)
}

// DurationToCronOffset returns a cron spec offset by the given minutes.
// Used to stagger tasks that depend on each other (e.g. enqueue at :00, process at :01).
func DurationToCronOffset(d time.Duration, offsetMinutes int) string {
	return durationToCronWithOffset(d, offsetMinutes)
}

func durationToCronWithOffset(d time.Duration, offset int) string {
	minutes := int(d.Minutes())
	switch {
	case minutes <= 0:
		return fmt.Sprintf("%d * * * *", 5+offset) // fallback: hourly
	case minutes < 60:
		return fmt.Sprintf("%d-59/%d * * * *", offset, minutes)
	case minutes == 60:
		return fmt.Sprintf("%d * * * *", offset) // hourly
	default:
		hours := int(d.Hours())
		if hours <= 0 {
			hours = 1
		}
		return fmt.Sprintf("%d */%d * * *", offset, hours)
	}
}
