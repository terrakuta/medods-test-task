package task

import (
	"fmt"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const defaultMaterializeDays = 90

const maxMaterializeDays = 366

func materializationWindow(rule taskdomain.RecurrenceRule, materializeDays int) (from time.Time, to time.Time, err error) {
	if materializeDays <= 0 {
		materializeDays = defaultMaterializeDays
	}

	if materializeDays > maxMaterializeDays {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: materialize_days must be between 1 and %d", ErrInvalidInput, maxMaterializeDays)
	}

	from = normalizeUTCDate(rule.StartDate)
	to = from.AddDate(0, 0, materializeDays-1)

	if rule.EndDate != nil {
		endCap := normalizeUTCDate(*rule.EndDate)
		if endCap.Before(to) {
			to = endCap
		}
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: recurrence window is empty for the configured horizon/end_date", ErrInvalidInput)
	}

	return from, to, nil
}

func normalizeUTCDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
