package storytime

// Fuzzy time formatting library

import (
	"fmt"
	"strings"
	"time"
)

func fuzzyTime(t time.Time) string {
	since := time.Since(t)
	if since < 0 {
		return "some time in the future"
	} else if since < 5*time.Second {
		return "moments ago"
	} else if since < 90*time.Second {
		return fmtPlural(int(since.Seconds()), "a second")
	} else if since < 90*time.Minute {
		return fmtPlural(int(since.Minutes()), "a minute")
	} else if since < day {
		return fmtPlural(int(since.Hours()), "an hour")
	} else if since < week {
		return fmtPlural(int(since/day), "a day")
	} else if since < month {
		return fmtPlural(int(since/week), "a week")
	} else if since < year {
		return fmtPlural(int(since/month), "a month")
	} else {
		return fmtPlural(int(since/year), "a year")
	}
}

const (
	day   time.Duration = 24 * time.Hour
	week                = 7 * day
	year                = 365*day + 6*time.Hour
	month               = year / 12
)

func fmtPlural(count int, unit string) string {
	if count == 1 {
		return unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", count, strings.SplitN(unit, " ", 2)[1])
}
