package unregistered_activity

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Notify is invoked but never registered with the worker.
func Unregistered(ctx workflow.Context, id string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, Charge, id).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, Notify, id).Get(ctx, nil)
}

func Charge(ctx context.Context, id string) error { return nil }
func Notify(ctx context.Context, id string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(Unregistered)
	w.RegisterActivity(Charge)
}
