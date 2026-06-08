package jobs

import (
	"context"
	"errors"
	"log/slog"

	"a-series-oracle/backend/internal/store"
)

type Executor interface {
	Execute(ctx context.Context, jobID string) error
}

type Runner struct {
	store    *store.Store
	queue    chan string
	executor Executor
}

func NewRunner(store *store.Store) *Runner {
	return NewRunnerWithExecutor(store, NewLocalExecutor(store))
}

func NewRunnerWithExecutor(store *store.Store, executor Executor) *Runner {
	if executor == nil {
		executor = NewLocalExecutor(store)
	}
	return &Runner{
		store:    store,
		queue:    make(chan string, 128),
		executor: executor,
	}
}

func (r *Runner) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-r.queue:
				if err := r.executor.Execute(ctx, id); err != nil && !errors.Is(err, store.ErrConflict) {
					slog.Warn("job execution failed", "jobID", id, "error", err)
				}
			}
		}
	}()
}

func (r *Runner) Enqueue(jobID string) {
	select {
	case r.queue <- jobID:
	default:
		if _, err := r.store.FailJob(jobID, "QUEUE_FULL", "job queue is full"); err != nil {
			slog.Warn("failed to mark queued job as failed", "jobID", jobID, "error", err)
		}
	}
}

func (r *Runner) RunOne(ctx context.Context, jobID string) error {
	return r.executor.Execute(ctx, jobID)
}
