package convert

import "time"

// toSecondsOrDefault converts a duration string in seconds and defaults to a
// given value if the duration is nil.
// The supported units are us, ms, s, m and h.
func toSecondsOrDefault(duration *time.Duration, defaultValue int32) int32 { //nolint: unparam
	if duration == nil {
		return defaultValue
	}

	return int32(duration.Seconds())
}
