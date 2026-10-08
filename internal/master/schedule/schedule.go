// Package schedule runs tasks on servers at set times: backup jobs, and policies such as
// nightly restarts. It stores the tasks and their runs, starts them when they are due and
// finds the servers they run on; what a task does is up to its kind.
package schedule

import (
	"fmt"
	"iter"
	"regexp"
	"slices"
	"time"
	_ "time/tzdata" // schedules use IANA time zones, which minimal systems don't have
)

const (
	maxTimes = 48
	maxDates = 100
	// lookahead covers the longest time between two days of a schedule: a day of the month
	// runs at least once a month.
	lookahead = 63
)

var timePattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// Schedule names the times of day a task runs at, in a time zone: on weekdays, on days of the
// month, or on single dates.
type Schedule struct {
	// Days are weekdays, 0 for Sunday; none means every day.
	Days []time.Weekday `json:"days"`
	// MonthDays are days of the month from 1 to 31, instead of weekdays. A month without one
	// of them, e.g. February without the 30th, runs on its last day instead.
	MonthDays []int `json:"monthDays,omitempty"`
	// Dates are single days as YYYY-MM-DD, instead of weekdays or days of the month. After
	// the last one, the task turns itself off.
	Dates []string `json:"dates,omitempty"`
	// Times are times of day as HH:MM.
	Times    []string `json:"times"`
	TimeZone string   `json:"timeZone"`
}

// Normalize sorts the days, dates and times and removes duplicates. It returns a message for
// the administrator if the schedule is invalid.
func (s *Schedule) Normalize() string {
	slices.Sort(s.Days)
	slices.Sort(s.MonthDays)
	slices.Sort(s.Dates)
	slices.Sort(s.Times)
	s.Days, s.MonthDays = slices.Compact(s.Days), slices.Compact(s.MonthDays)
	s.Dates, s.Times = slices.Compact(s.Dates), slices.Compact(s.Times)
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
	switch {
	case len(s.Days) > 0 && len(s.MonthDays) > 0 || len(s.Dates) > 0 && len(s.Days)+len(s.MonthDays) > 0:
		return "Choose weekdays, days of the month or dates."
	case len(s.Days) > 0 && (s.Days[0] < time.Sunday || s.Days[len(s.Days)-1] > time.Saturday):
		return "Choose days of the week."
	case len(s.MonthDays) > 0 && (s.MonthDays[0] < 1 || s.MonthDays[len(s.MonthDays)-1] > 31):
		return "Choose days of the month from 1 to 31."
	case len(s.Dates) > maxDates:
		return fmt.Sprintf("Enter up to %d dates.", maxDates)
	}
	for _, d := range s.Dates {
		if day, err := time.Parse(time.DateOnly, d); err != nil || day.Year() < 2000 {
			return fmt.Sprintf("%q is not a date like 2026-12-24.", d)
		}
	}
	return ""
}

// Next returns the first time after t that the schedule names, or the zero time once all its
// dates passed. A time of day that a day lacks, because the clocks are put forward, moves
// forward with them; one that a day has twice, as they are put back, counts once.
func (s Schedule) Next(t time.Time) time.Time {
	loc, err := time.LoadLocation(s.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)
	for day := range s.days(t) {
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
	return time.Time{} // all dates passed, or a schedule that Normalize rejects
}

// days yields the days that the schedule runs on from the day of t on, in t's time zone.
func (s Schedule) days(t time.Time) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		if len(s.Dates) > 0 {
			for _, d := range s.Dates {
				day, err := time.ParseInLocation(time.DateOnly, d, t.Location())
				if err == nil && d >= t.Format(time.DateOnly) && !yield(day) {
					return
				}
			}
			return
		}
		for i := range lookahead {
			if day := t.AddDate(0, 0, i); s.runsOn(day) && !yield(day) {
				return
			}
		}
	}
}

// runsOn tells whether the weekdays or days of the month of a schedule include a day.
func (s Schedule) runsOn(day time.Time) bool {
	if len(s.MonthDays) == 0 {
		return len(s.Days) == 0 || slices.Contains(s.Days, day.Weekday())
	}
	last := time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return slices.Contains(s.MonthDays, day.Day()) || day.Day() == last && slices.Max(s.MonthDays) > last
}
