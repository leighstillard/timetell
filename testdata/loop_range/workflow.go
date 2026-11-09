package loop_range

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func LoopRange(ctx workflow.Context, items []string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	for _, item := range items {
		if err := workflow.ExecuteActivity(ctx, ProcessItem, item).Get(ctx, nil); err != nil {
			return err
		}
	}
	return workflow.ExecuteActivity(ctx, Summarize, len(items)).Get(ctx, nil)
}

func ProcessItem(ctx context.Context, item string) error { return nil }
func Summarize(ctx context.Context, n int) error         { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(LoopRange)
	w.RegisterActivity(ProcessItem)
	w.RegisterActivity(Summarize)
}
