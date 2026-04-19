package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	CreateWithRecurrence(
		ctx context.Context,
		template *taskdomain.Task,
		rule taskdomain.RecurrenceRule,
		occurrenceDates []time.Time,
		materializedUntil time.Time,
	) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	GetStoredRecurrence(ctx context.Context, templateID int64) (taskdomain.StoredRecurrence, error)
	MaterializeRecurrence(ctx context.Context, templateID int64, newOccurrences []time.Time, newMaterializedUntil time.Time) error
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	MaterializeSeries(ctx context.Context, templateID int64, untilUTC time.Time) error
}

type CreateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status

	Recurrence *taskdomain.RecurrenceRule
	// MaterializeDays is how many calendar days forward from start_date (inclusive) to generate instances on create.
	// Defaults to 90, capped at 366.
	MaterializeDays int
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
}
