-- Recurring tasks: template row (no series_template_id) + optional rule + dated instances.

ALTER TABLE tasks
	ADD COLUMN IF NOT EXISTS series_template_id BIGINT REFERENCES tasks (id) ON DELETE CASCADE,
	ADD COLUMN IF NOT EXISTS occurrence_date DATE;

COMMENT ON COLUMN tasks.series_template_id IS 'If set, this task is a materialized occurrence of a recurring template task.';
COMMENT ON COLUMN tasks.occurrence_date IS 'Calendar date (UTC) of the occurrence for instance rows.';

ALTER TABLE tasks
	ADD CONSTRAINT chk_tasks_occurrence_pair
	CHECK (
		(series_template_id IS NULL AND occurrence_date IS NULL)
		OR (series_template_id IS NOT NULL AND occurrence_date IS NOT NULL)
	);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_series_occurrence
	ON tasks (series_template_id, occurrence_date);

CREATE TABLE IF NOT EXISTS task_recurrence (
	template_task_id BIGINT PRIMARY KEY REFERENCES tasks (id) ON DELETE CASCADE,
	recurrence_type TEXT NOT NULL,
	interval_days INT,
	day_of_month INT,
	specific_dates TEXT[],
	month_day_parity TEXT,
	start_date DATE NOT NULL,
	end_date DATE,
	materialized_until DATE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_task_recurrence_type ON task_recurrence (recurrence_type);
