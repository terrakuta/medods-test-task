package task

import (
	"testing"
	"time"
)

func TestRecurrenceRule_DailyInterval(t *testing.T) {
	start := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	rule := RecurrenceRule{
		Type:         RecurrenceDailyInterval,
		IntervalDays: 2,
		StartDate:    start,
	}

	assertMatch(t, rule, "2026-04-01", true)
	assertMatch(t, rule, "2026-04-02", false)
	assertMatch(t, rule, "2026-04-03", true)
	assertMatch(t, rule, "2026-03-31", false)
}

func TestRecurrenceRule_MonthlyDay(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rule := RecurrenceRule{
		Type:         RecurrenceMonthlyDay,
		DayOfMonth:   15,
		StartDate:    start,
	}

	assertMatch(t, rule, "2026-02-15", true)
	assertMatch(t, rule, "2026-02-14", false)
}

func TestRecurrenceRule_MonthlyDay_February30Skipped(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rule := RecurrenceRule{
		Type:         RecurrenceMonthlyDay,
		DayOfMonth:   30,
		StartDate:    start,
	}

	assertMatch(t, rule, "2026-02-28", false)
	assertMatch(t, rule, "2026-03-30", true)
}

func TestRecurrenceRule_FixedDates(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rule := RecurrenceRule{
		Type: RecurrenceFixedDates,
		SpecificDates: []time.Time{
			time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 6, 15, 23, 59, 59, 0, time.UTC),
		},
		StartDate: start,
	}

	assertMatch(t, rule, "2026-05-01", true)
	assertMatch(t, rule, "2026-06-15", true)
	assertMatch(t, rule, "2026-06-14", false)
}

func TestRecurrenceRule_Parity(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	even := RecurrenceRule{
		Type:            RecurrenceMonthDayParity,
		MonthDayParity:  ParityEven,
		StartDate:       start,
	}

	assertMatch(t, even, "2026-04-02", true)
	assertMatch(t, even, "2026-04-03", false)

	odd := RecurrenceRule{
		Type:            RecurrenceMonthDayParity,
		MonthDayParity:  ParityOdd,
		StartDate:       start,
	}

	assertMatch(t, odd, "2026-04-01", true)
	assertMatch(t, odd, "2026-04-02", false)
}

func TestRecurrenceRule_EndDate(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	rule := RecurrenceRule{
		Type:         RecurrenceDailyInterval,
		IntervalDays: 1,
		StartDate:    start,
		EndDate:      &end,
	}

	assertMatch(t, rule, "2026-04-03", true)
	assertMatch(t, rule, "2026-04-04", false)
}

func assertMatch(t *testing.T, rule RecurrenceRule, day string, want bool) {
	t.Helper()

	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatal(err)
	}

	if got := rule.DateMatches(d); got != want {
		t.Fatalf("DateMatches(%s) = %v, want %v for %#v", day, got, want, rule)
	}
}
