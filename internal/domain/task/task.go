package task

import "time"

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// SeriesTemplateID is set for materialized occurrences and points at the template task row.
	SeriesTemplateID *int64 `json:"series_template_id,omitempty"`
	// OccurrenceDate is the calendar date (UTC) for instance rows.
	OccurrenceDate *time.Time `json:"occurrence_date,omitempty"`
	// Recurrence is populated for template tasks that define a series.
	Recurrence *RecurrenceRule `json:"recurrence,omitempty"`
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}
