package task

import (
	"fmt"
	"time"
)

// RecurrenceType defines how occurrence dates are chosen for a recurring series.
type RecurrenceType string

const (
	RecurrenceDailyInterval RecurrenceType = "daily_interval"
	RecurrenceMonthlyDay    RecurrenceType = "monthly_day"
	RecurrenceFixedDates    RecurrenceType = "fixed_dates"
	RecurrenceMonthDayParity RecurrenceType = "month_day_parity"
)

func (t RecurrenceType) Valid() bool {
	switch t {
	case RecurrenceDailyInterval, RecurrenceMonthlyDay, RecurrenceFixedDates, RecurrenceMonthDayParity:
		return true
	default:
		return false
	}
}

// MonthDayParity selects calendar days of month by parity (1..31).
type MonthDayParity string

const (
	ParityEven MonthDayParity = "even"
	ParityOdd  MonthDayParity = "odd"
)

func (p MonthDayParity) Valid() bool {
	switch p {
	case ParityEven, ParityOdd:
		return true
	default:
		return false
	}
}

// StoredRecurrence is the persisted schedule for a template task.
type StoredRecurrence struct {
	Rule              RecurrenceRule
	MaterializedUntil time.Time
}

// RecurrenceRule describes when instances of a recurring task should exist.
type RecurrenceRule struct {
	Type RecurrenceType `json:"type"`

	// IntervalDays: every n-th calendar day starting from StartDate (inclusive), n >= 1.
	IntervalDays int `json:"interval_days,omitempty"`

	// DayOfMonth: 1..30 — task on that day-of-month each month (months with fewer days skip silently).
	DayOfMonth int `json:"day_of_month,omitempty"`

	// SpecificDates: explicit occurrence calendar dates.
	SpecificDates []time.Time `json:"specific_dates,omitempty"`

	// MonthDayParity: even or odd day-of-month numbers.
	MonthDayParity MonthDayParity `json:"month_day_parity,omitempty"`

	StartDate time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
}

// DateMatches reports whether the UTC calendar date should spawn an instance.
func (r RecurrenceRule) DateMatches(dayUTC time.Time) bool {
	d := normalizeToUTCDate(dayUTC)
	start := normalizeToUTCDate(r.StartDate)
	if d.Before(start) {
		return false
	}

	if r.EndDate != nil {
		end := normalizeToUTCDate(*r.EndDate)
		if d.After(end) {
			return false
		}
	}

	switch r.Type {
	case RecurrenceDailyInterval:
		if r.IntervalDays <= 0 {
			return false
		}

		diff := calendarDaysBetweenUTC(start, d)
		if diff < 0 {
			return false
		}

		return diff%r.IntervalDays == 0

	case RecurrenceMonthlyDay:
		if r.DayOfMonth < 1 || r.DayOfMonth > 30 {
			return false
		}

		last := lastDayOfMonthUTC(d)
		target := r.DayOfMonth
		if target > last {
			return false
		}

		return d.Day() == target

	case RecurrenceFixedDates:
		for i := range r.SpecificDates {
			if sameCalendarDateUTC(d, normalizeToUTCDate(r.SpecificDates[i])) {
				return true
			}
		}

		return false

	case RecurrenceMonthDayParity:
		dom := d.Day()
		switch r.MonthDayParity {
		case ParityEven:
			return dom%2 == 0
		case ParityOdd:
			return dom%2 == 1
		default:
			return false
		}

	default:
		return false
	}
}

// Validate checks recurrence payload for a non-zero rule.
func (r RecurrenceRule) Validate() error {
	if !r.Type.Valid() {
		return fmt.Errorf("%w: unknown recurrence type", ErrInvalidRecurrence)
	}

	if r.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidRecurrence)
	}

	if r.EndDate != nil {
		end := normalizeToUTCDate(*r.EndDate)
		start := normalizeToUTCDate(r.StartDate)
		if end.Before(start) {
			return fmt.Errorf("%w: end_date must not be before start_date", ErrInvalidRecurrence)
		}
	}

	switch r.Type {
	case RecurrenceDailyInterval:
		if r.IntervalDays < 1 {
			return fmt.Errorf("%w: interval_days must be >= 1", ErrInvalidRecurrence)
		}

	case RecurrenceMonthlyDay:
		if r.DayOfMonth < 1 || r.DayOfMonth > 30 {
			return fmt.Errorf("%w: day_of_month must be between 1 and 30", ErrInvalidRecurrence)
		}

	case RecurrenceFixedDates:
		if len(r.SpecificDates) == 0 {
			return fmt.Errorf("%w: specific_dates must not be empty", ErrInvalidRecurrence)
		}

		for i := range r.SpecificDates {
			if r.SpecificDates[i].IsZero() {
				return fmt.Errorf("%w: specific_dates contains invalid date", ErrInvalidRecurrence)
			}
		}

	case RecurrenceMonthDayParity:
		if !r.MonthDayParity.Valid() {
			return fmt.Errorf("%w: month_day_parity must be even or odd", ErrInvalidRecurrence)
		}

	default:
		return fmt.Errorf("%w: unknown recurrence type", ErrInvalidRecurrence)
	}

	return nil
}

func normalizeToUTCDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sameCalendarDateUTC(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()

	return ay == by && am == bm && ad == bd
}

func calendarDaysBetweenUTC(from, to time.Time) int {
	f := normalizeToUTCDate(from)
	t := normalizeToUTCDate(to)

	return int(t.Sub(f).Hours() / 24)
}

func lastDayOfMonthUTC(day time.Time) int {
	y, m, _ := day.UTC().Date()

	firstNextMonth := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
	lastThisMonth := firstNextMonth.AddDate(0, 0, -1)

	return lastThisMonth.Day()
}

// EnumerateOccurrenceDates returns each UTC calendar date in [fromInclusive, toInclusive] that matches the rule.
func EnumerateOccurrenceDates(rule RecurrenceRule, fromInclusive, toInclusive time.Time) []time.Time {
	from := normalizeToUTCDate(fromInclusive)
	to := normalizeToUTCDate(toInclusive)
	if to.Before(from) {
		return nil
	}

	out := make([]time.Time, 0)

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if rule.DateMatches(d) {
			out = append(out, d)
		}
	}

	return out
}
