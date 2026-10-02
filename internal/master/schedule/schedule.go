// Package schedule runs tasks on servers at set times: backup jobs, and policies such as
// nightly restarts. It stores the tasks, starts them when they are due and finds the
// servers they run on; what a task does is up to its kind.
package schedule

import (
	"fmt"
	"regexp"
	"slices"
	"time"
	_ "time/tzdata" // schedules use IANA time zones, which minimal systems don't have
)

const maxTimes = 48

var timePattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// Schedule names the times of day and the weekdays a task runs at, in a time zone.
type Schedule struct {
	// Days are weekdays, 0 for Sunday; none means every day.
	Days []time.Weekday `json:"days"`
	// Times are times of day as HH:MM.
	Times    []string `json:"times"`
	TimeZone string   `json:"timeZone"`
}

// normalize sorts the days and times and removes duplicates. It returns a message for the
// administrator if the schedule is invalid.
func (s *Schedule) normalize() string {
	slices.Sort(s.Days)
	slices.Sort(s.Times)
	s.Days, s.Times = slices.Compact(s.Days), slices.Compact(s.Times)
	if len(s.Days) == 7 || s.Days == nil {
		s.Days = []time.Weekday{}
	}
	if _, err := time.LoadLocation(s.TimeZone); err != nil || s.TimeZone == "" {
		return "Choose a time zone."
	}
	if len(s.Times) == 0 || len(s.Times) > maxTimes {
		return fmt.Sprintf("Enter 1 to %d times of day.", maxTimes)
	}
	for _, t := range s.Times {
		if !timePattern.MatchString(t) {
			return fmt.Sprintf("%q is not a time of day like 04:30.", t)
		}
	}
	if len(s.Days) > 0 && (s.Days[0] < time.Sunday || s.Days[len(s.Days)-1] > time.Saturday) {
		return "Choose days of the week."
	}
	return ""
}

// Next returns the first time after t that the schedule names. A time of day that a day
// lacks, because the clocks are put forward, moves forward with them.
func (s Schedule) Next(t time.Time) time.Time {
	loc, err := time.LoadLocation(s.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)
	for i := range 8 {
		day := t.AddDate(0, 0, i)
		if len(s.Days) > 0 && !slices.Contains(s.Days, day.Weekday()) {
			continue
		}
		var next time.Time
		for _, hm := range s.Times {
			var hour, minute int
			if _, err := fmt.Sscanf(hm, "%d:%d", &hour, &minute); err != nil {
				continue
			}
			at := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc)
			if at.After(t) && (next.IsZero() || at.Before(next)) {
				next = at
			}
		}
		if !next.IsZero() {
			return next
		}
	}
	return time.Time{} // only for schedules that normalize rejects
}
