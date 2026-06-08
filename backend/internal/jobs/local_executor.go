package jobs

import (
	"context"
	"errors"
	"time"

	"a-series-oracle/backend/internal/store"
)

type LocalExecutor struct {
	store *store.Store
	step  time.Duration
}

func NewLocalExecutor(store *store.Store) *LocalExecutor {
	return &LocalExecutor{
		store: store,
		step:  450 * time.Millisecond,
	}
}

func (e *LocalExecutor) Execute(ctx context.Context, jobID string) error {
	if _, err := e.store.StartJob(jobID); err != nil {
		return ignoreConflict(err)
	}
	if err := e.pause(ctx); err != nil {
		return err
	}

	if _, err := e.store.MarkJobWaitingOCI(jobID); err != nil {
		return ignoreConflict(err)
	}
	if err := e.pause(ctx); err != nil {
		return err
	}

	if _, err := e.store.MarkJobVerifying(jobID); err != nil {
		return ignoreConflict(err)
	}
	if err := e.pause(ctx); err != nil {
		return err
	}

	if _, err := e.store.CompleteJob(jobID, map[string]any{
		"message":       "execution completed by local executor",
		"verified":      true,
		"executionMode": "local",
	}); err != nil {
		return ignoreConflict(err)
	}
	return nil
}

func (e *LocalExecutor) pause(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(e.step):
		return nil
	}
}

func ignoreConflict(err error) error {
	if errors.Is(err, store.ErrConflict) {
		return nil
	}
	return err
}
