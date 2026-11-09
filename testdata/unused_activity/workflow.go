package unused_activity

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Audit is registered but no workflow invokes it.
func Unused(ctx workflow.Context, id string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	return workflow.ExecuteActivity(ctx, Charge, id).Get(ctx, nil)
}

func Charge(ctx context.Context, id string) error { return nil }
func Audit(ctx context.Context, id string) error  { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(Unused)
	w.RegisterActivity(Charge)
	w.RegisterActivity(Audit)
}
