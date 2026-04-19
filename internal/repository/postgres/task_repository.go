package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, created_at, updated_at, series_template_id, occurrence_date)
		VALUES ($1, $2, $3, $4, $5, NULL, NULL)
		RETURNING id, title, description, status, created_at, updated_at, series_template_id, occurrence_date
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.CreatedAt, task.UpdatedAt)
	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) CreateWithRecurrence(
	ctx context.Context,
	template *taskdomain.Task,
	rule taskdomain.RecurrenceRule,
	occurrenceDates []time.Time,
	materializedUntil time.Time,
) (*taskdomain.Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const insertTemplate = `
		INSERT INTO tasks (title, description, status, created_at, updated_at, series_template_id, occurrence_date)
		VALUES ($1, $2, $3, $4, $5, NULL, NULL)
		RETURNING id, title, description, status, created_at, updated_at, series_template_id, occurrence_date
	`

	row := tx.QueryRow(ctx, insertTemplate, template.Title, template.Description, template.Status, template.CreatedAt, template.UpdatedAt)
	createdTemplate, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	if err := insertRecurrenceRow(ctx, tx, createdTemplate.ID, rule, materializedUntil); err != nil {
		return nil, err
	}

	const insertInstance = `
		INSERT INTO tasks (title, description, status, series_template_id, occurrence_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (series_template_id, occurrence_date) DO NOTHING
	`

	for _, day := range occurrenceDates {
		dayUTC := normalizeDateUTC(day)
		if _, err := tx.Exec(ctx, insertInstance,
			template.Title,
			template.Description,
			template.Status,
			createdTemplate.ID,
			dayUTC,
			template.CreatedAt,
			template.UpdatedAt,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, createdTemplate.ID)
}

func (r *Repository) MaterializeRecurrence(
	ctx context.Context,
	templateID int64,
	newOccurrences []time.Time,
	newMaterializedUntil time.Time,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const insertInstance = `
		INSERT INTO tasks (title, description, status, series_template_id, occurrence_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (series_template_id, occurrence_date) DO NOTHING
	`

	const updateRule = `
		UPDATE task_recurrence
		SET materialized_until = $2
		WHERE template_task_id = $1
	`

	template, err := loadTaskTx(ctx, tx, templateID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return taskdomain.ErrNotFound
		}

		return err
	}

	if template.SeriesTemplateID != nil {
		return taskdomain.ErrNotFound
	}

	for _, day := range newOccurrences {
		dayUTC := normalizeDateUTC(day)
		if _, err := tx.Exec(ctx, insertInstance,
			template.Title,
			template.Description,
			template.Status,
			templateID,
			dayUTC,
			template.CreatedAt,
			template.UpdatedAt,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, updateRule, templateID, newMaterializedUntil); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetStoredRecurrence(ctx context.Context, templateID int64) (taskdomain.StoredRecurrence, error) {
	const query = `
		SELECT recurrence_type, interval_days, day_of_month, specific_dates, month_day_parity, start_date, end_date, materialized_until
		FROM task_recurrence
		WHERE template_task_id = $1
	`

	row := r.pool.QueryRow(ctx, query, templateID)

	var (
		recurrenceType string
		interval       pgtype.Int4
		dayOfMonth     pgtype.Int4
		specificDates  []string
		parity         pgtype.Text
		startDate      pgtype.Date
		endDate        pgtype.Date
		materialized   pgtype.Date
	)

	if err := row.Scan(
		&recurrenceType,
		&interval,
		&dayOfMonth,
		&specificDates,
		&parity,
		&startDate,
		&endDate,
		&materialized,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return taskdomain.StoredRecurrence{}, taskdomain.ErrNotFound
		}

		return taskdomain.StoredRecurrence{}, err
	}

	rule, err := scanRecurrenceRule(
		recurrenceType,
		interval,
		dayOfMonth,
		specificDates,
		parity,
		startDate,
		endDate,
	)
	if err != nil {
		return taskdomain.StoredRecurrence{}, err
	}

	if !materialized.Valid {
		return taskdomain.StoredRecurrence{}, errors.New("materialized_until is invalid")
	}

	return taskdomain.StoredRecurrence{
		Rule:              rule,
		MaterializedUntil: dateFromPgtype(materialized),
	}, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT
			t.id,
			t.title,
			t.description,
			t.status,
			t.created_at,
			t.updated_at,
			t.series_template_id,
			t.occurrence_date,
			tr.recurrence_type,
			tr.interval_days,
			tr.day_of_month,
			tr.specific_dates,
			tr.month_day_parity,
			tr.start_date,
			tr.end_date
		FROM tasks t
		LEFT JOIN task_recurrence tr ON tr.template_task_id = t.id
		WHERE t.id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTaskWithOptionalRecurrence(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, status, created_at, updated_at, series_template_id, occurrence_date
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return r.GetByID(ctx, updated.ID)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT
			t.id,
			t.title,
			t.description,
			t.status,
			t.created_at,
			t.updated_at,
			t.series_template_id,
			t.occurrence_date,
			tr.recurrence_type,
			tr.interval_days,
			tr.day_of_month,
			tr.specific_dates,
			tr.month_day_parity,
			tr.start_date,
			tr.end_date
		FROM tasks t
		LEFT JOIN task_recurrence tr ON tr.template_task_id = t.id
		ORDER BY t.id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTaskWithOptionalRecurrence(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func loadTaskTx(ctx context.Context, tx pgx.Tx, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, created_at, updated_at, series_template_id, occurrence_date
		FROM tasks
		WHERE id = $1
	`

	row := tx.QueryRow(ctx, query, id)

	return scanTask(row)
}

func insertRecurrenceRow(ctx context.Context, tx pgx.Tx, templateID int64, rule taskdomain.RecurrenceRule, materializedUntil time.Time) error {
	const query = `
		INSERT INTO task_recurrence (
			template_task_id,
			recurrence_type,
			interval_days,
			day_of_month,
			specific_dates,
			month_day_parity,
			start_date,
			end_date,
			materialized_until
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	recurrenceType := string(rule.Type)

	var interval any
	if rule.Type == taskdomain.RecurrenceDailyInterval {
		interval = rule.IntervalDays
	}

	var dom any
	if rule.Type == taskdomain.RecurrenceMonthlyDay {
		dom = rule.DayOfMonth
	}

	var dates any
	if rule.Type == taskdomain.RecurrenceFixedDates {
		arr := make([]string, 0, len(rule.SpecificDates))
		for i := range rule.SpecificDates {
			arr = append(arr, normalizeDateUTC(rule.SpecificDates[i]).Format("2006-01-02"))
		}

		dates = arr
	}

	var parity any
	if rule.Type == taskdomain.RecurrenceMonthDayParity {
		parity = string(rule.MonthDayParity)
	}

	var end any
	if rule.EndDate != nil {
		end = normalizeDateUTC(*rule.EndDate)
	}

	_, err := tx.Exec(ctx, query,
		templateID,
		recurrenceType,
		interval,
		dom,
		dates,
		parity,
		normalizeDateUTC(rule.StartDate),
		end,
		normalizeDateUTC(materializedUntil),
	)

	return err
}

func normalizeDateUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task            taskdomain.Task
		status          string
		seriesID        pgtype.Int8
		occurrence      pgtype.Date
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.CreatedAt,
		&task.UpdatedAt,
		&seriesID,
		&occurrence,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)

	if seriesID.Valid {
		v := seriesID.Int64
		task.SeriesTemplateID = &v
	}

	if occurrence.Valid {
		t := dateFromPgtype(occurrence)
		task.OccurrenceDate = &t
	}

	return &task, nil
}

func scanTaskWithOptionalRecurrence(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task            taskdomain.Task
		status          string
		seriesID        pgtype.Int8
		occurrence      pgtype.Date
		recurrenceType  pgtype.Text
		interval        pgtype.Int4
		dayOfMonth      pgtype.Int4
		specificDates   []string
		parity          pgtype.Text
		startDate       pgtype.Date
		endDate         pgtype.Date
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.CreatedAt,
		&task.UpdatedAt,
		&seriesID,
		&occurrence,
		&recurrenceType,
		&interval,
		&dayOfMonth,
		&specificDates,
		&parity,
		&startDate,
		&endDate,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)

	if seriesID.Valid {
		v := seriesID.Int64
		task.SeriesTemplateID = &v
	}

	if occurrence.Valid {
		t := dateFromPgtype(occurrence)
		task.OccurrenceDate = &t
	}

	if recurrenceType.Valid {
		rule, err := scanRecurrenceRule(
			recurrenceType.String,
			interval,
			dayOfMonth,
			specificDates,
			parity,
			startDate,
			endDate,
		)
		if err != nil {
			return nil, err
		}

		task.Recurrence = &rule
	}

	return &task, nil
}

func scanRecurrenceRule(
	recurrenceType string,
	interval pgtype.Int4,
	dayOfMonth pgtype.Int4,
	specificDates []string,
	parity pgtype.Text,
	startDate pgtype.Date,
	endDate pgtype.Date,
) (taskdomain.RecurrenceRule, error) {
	rule := taskdomain.RecurrenceRule{
		Type: taskdomain.RecurrenceType(recurrenceType),
	}

	if !startDate.Valid {
		return taskdomain.RecurrenceRule{}, errors.New("recurrence start_date is invalid")
	}

	rule.StartDate = dateFromPgtype(startDate)

	if endDate.Valid {
		end := dateFromPgtype(endDate)
		rule.EndDate = &end
	}

	switch rule.Type {
	case taskdomain.RecurrenceDailyInterval:
		if !interval.Valid {
			return taskdomain.RecurrenceRule{}, errors.New("interval_days is required")
		}

		rule.IntervalDays = int(interval.Int32)
	case taskdomain.RecurrenceMonthlyDay:
		if !dayOfMonth.Valid {
			return taskdomain.RecurrenceRule{}, errors.New("day_of_month is required")
		}

		rule.DayOfMonth = int(dayOfMonth.Int32)
	case taskdomain.RecurrenceFixedDates:
		rule.SpecificDates = make([]time.Time, 0, len(specificDates))
		for i := range specificDates {
			parsed, err := time.Parse("2006-01-02", specificDates[i])
			if err != nil {
				return taskdomain.RecurrenceRule{}, err
			}

			rule.SpecificDates = append(rule.SpecificDates, parsed.UTC())
		}
	case taskdomain.RecurrenceMonthDayParity:
		if !parity.Valid {
			return taskdomain.RecurrenceRule{}, errors.New("month_day_parity is required")
		}

		rule.MonthDayParity = taskdomain.MonthDayParity(parity.String)
	default:
		return taskdomain.RecurrenceRule{}, errors.New("unknown recurrence type")
	}

	return rule, nil
}

func dateFromPgtype(d pgtype.Date) time.Time {
	y, m, day := d.Time.Date()

	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}
