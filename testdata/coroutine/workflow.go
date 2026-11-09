package coroutine

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Coroutine sends a reminder from a workflow.Go coroutine while the main
// path processes the order.
func Coroutine(ctx workflow.Context, orderID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	workflow.Go(ctx, func(gctx workflow.Context) {
		_ = workflow.ExecuteActivity(gctx, SendReminder, orderID).Get(gctx, nil)
	})
	return workflow.ExecuteActivity(ctx, ProcessOrder, orderID).Get(ctx, nil)
}

func SendReminder(ctx context.Context, id string) error { return nil }
func ProcessOrder(ctx context.Context, id string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(Coroutine)
	w.RegisterActivity(SendReminder)
	w.RegisterActivity(ProcessOrder)
}
