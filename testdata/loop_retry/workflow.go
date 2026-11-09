package loop_retry

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// LoopRetry tries the charge up to three times, then sends a receipt.
func LoopRetry(ctx workflow.Context, user string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = workflow.ExecuteActivity(ctx, ChargeCard, user).Get(ctx, nil)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, SendReceipt, user).Get(ctx, nil)
}

func ChargeCard(ctx context.Context, user string) error  { return nil }
func SendReceipt(ctx context.Context, user string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(LoopRetry)
	w.RegisterActivity(ChargeCard)
	w.RegisterActivity(SendReceipt)
}
