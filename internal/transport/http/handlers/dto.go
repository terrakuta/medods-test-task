package handlers

import (
	"errors"
	"fmt"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type taskMutationDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
}

type taskCreateDTO struct {
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Status            taskdomain.Status `json:"status"`
	Recurrence        *recurrenceDTO    `json:"recurrence,omitempty"`
	MaterializeDays   int               `json:"materialize_days,omitempty"`
}

type recurrenceDTO struct {
	Type             string   `json:"type"`
	IntervalDays     int      `json:"interval_days,omitempty"`
	DayOfMonth       int      `json:"day_of_month,omitempty"`
	SpecificDates    []string `json:"specific_dates,omitempty"`
	MonthDayParity   string   `json:"month_day_parity,omitempty"`
	StartDate        string   `json:"start_date"`
	EndDate          *string  `json:"end_date,omitempty"`
}

type materializeDTO struct {
	Until string `json:"until"`
}

func (r *taskCreateDTO) recurrenceToDomain() (*taskdomain.RecurrenceRule, error) {
	if r.Recurrence == nil {
		return nil, nil
	}

	rd := r.Recurrence

	rule := taskdomain.RecurrenceRule{
		Type:           taskdomain.RecurrenceType(rd.Type),
		IntervalDays:   rd.IntervalDays,
		DayOfMonth:     rd.DayOfMonth,
		MonthDayParity: taskdomain.MonthDayParity(rd.MonthDayParity),
	}

	if rd.StartDate == "" {
		return nil, errors.New("recurrence.start_date is required")
	}

	start, err := time.Parse("2006-01-02", rd.StartDate)
	if err != nil {
		return nil, fmt.Errorf("recurrence.start_date: %w", err)
	}

	rule.StartDate = start.UTC()

	if rd.EndDate != nil && *rd.EndDate != "" {
		end, err := time.Parse("2006-01-02", *rd.EndDate)
		if err != nil {
			return nil, fmt.Errorf("recurrence.end_date: %w", err)
		}

		endUTC := end.UTC()
		rule.EndDate = &endUTC
	}

	if rule.Type == taskdomain.RecurrenceFixedDates {
		rule.SpecificDates = make([]time.Time, 0, len(rd.SpecificDates))
		for i := range rd.SpecificDates {
			d, err := time.Parse("2006-01-02", rd.SpecificDates[i])
			if err != nil {
				return nil, fmt.Errorf("recurrence.specific_dates[%d]: %w", i, err)
			}

			rule.SpecificDates = append(rule.SpecificDates, d.UTC())
		}
	}

	return &rule, nil
}

type taskDTO struct {
	ID               int64             `json:"id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Status           taskdomain.Status `json:"status"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	SeriesTemplateID *int64            `json:"series_template_id,omitempty"`
	OccurrenceDate   *string           `json:"occurrence_date,omitempty"`
	Recurrence       *recurrenceDTO    `json:"recurrence,omitempty"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	dto := taskDTO{
		ID:               task.ID,
		Title:            task.Title,
		Description:      task.Description,
		Status:           task.Status,
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
		SeriesTemplateID: task.SeriesTemplateID,
	}

	if task.OccurrenceDate != nil {
		s := task.OccurrenceDate.UTC().Format("2006-01-02")
		dto.OccurrenceDate = &s
	}

	if task.Recurrence != nil {
		dto.Recurrence = recurrenceFromDomain(task.Recurrence)
	}

	return dto
}

func recurrenceFromDomain(rule *taskdomain.RecurrenceRule) *recurrenceDTO {
	if rule == nil {
		return nil
	}

	dto := &recurrenceDTO{
		Type:      string(rule.Type),
		StartDate: rule.StartDate.UTC().Format("2006-01-02"),
	}

	if rule.EndDate != nil {
		s := rule.EndDate.UTC().Format("2006-01-02")
		dto.EndDate = &s
	}

	switch rule.Type {
	case taskdomain.RecurrenceDailyInterval:
		dto.IntervalDays = rule.IntervalDays
	case taskdomain.RecurrenceMonthlyDay:
		dto.DayOfMonth = rule.DayOfMonth
	case taskdomain.RecurrenceFixedDates:
		dto.SpecificDates = make([]string, 0, len(rule.SpecificDates))
		for i := range rule.SpecificDates {
			dto.SpecificDates = append(dto.SpecificDates, rule.SpecificDates[i].UTC().Format("2006-01-02"))
		}
	case taskdomain.RecurrenceMonthDayParity:
		dto.MonthDayParity = string(rule.MonthDayParity)
	}

	return dto
}
