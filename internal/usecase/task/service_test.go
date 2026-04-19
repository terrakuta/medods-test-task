package task_test

import (
	"context"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	"example.com/taskservice/internal/usecase/task"
)

type mockRepo struct {
	task.Repository
	createCalled bool
}

func (m *mockRepo) Create(ctx context.Context, t *taskdomain.Task) (*taskdomain.Task, error) {
	m.createCalled = true
	t.ID = 1
	return t, nil
}

func TestService_Create(t *testing.T) {
	repo := &mockRepo{}
	svc := task.NewService(repo)

	input := task.CreateInput{
		Title:       "Test task",
		Description: "Test description",
		Status:      taskdomain.StatusNew,
	}

	created, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !repo.createCalled {
		t.Errorf("expected Create to be called on repo")
	}

	if created.ID != 1 {
		t.Errorf("expected task ID 1, got %d", created.ID)
	}

	if created.Title != "Test task" {
		t.Errorf("expected title 'Test task', got %q", created.Title)
	}
}
