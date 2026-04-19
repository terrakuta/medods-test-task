package task

import "errors"

var (
	ErrNotFound         = errors.New("task not found")
	ErrInvalidRecurrence = errors.New("invalid recurrence")
)
