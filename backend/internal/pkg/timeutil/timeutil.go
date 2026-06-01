package timeutil

import "time"

const (
	DefaultTimezone = "Asia/Shanghai"
	ISO8601Format   = "2006-01-02T15:04:05Z07:00"
)

var shanghaiLoc *time.Location

func init() {
	var err error
	shanghaiLoc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		shanghaiLoc = time.FixedZone("CST", 8*3600)
	}
}

// StartOfDay returns the start of the calendar day in the given timezone,
// converted back to UTC.
func StartOfDay(t time.Time, tz string) time.Time {
	loc := getLocation(tz)
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
}

// EndOfDay returns the end of the calendar day (exclusive) in the given timezone,
// converted back to UTC.
func EndOfDay(t time.Time, tz string) time.Time {
	return StartOfDay(t, tz).Add(24 * time.Hour)
}

// FormatISO formats a time as ISO8601 with timezone.
func FormatISO(t time.Time) string {
	return t.Format(ISO8601Format)
}

// NowUTC returns the current time in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}

func getLocation(tz string) *time.Location {
	if tz == "" {
		return shanghaiLoc
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return shanghaiLoc
	}
	return loc
}
