package simple_linear

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func SimpleLinear(ctx workflow.Context, in string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, ValidateInput, in).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, ProcessData, in).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, SaveResult, in).Get(ctx, nil)
}

func ValidateInput(ctx context.Context, in string) error { return nil }
func ProcessData(ctx context.Context, in string) error   { return nil }
func SaveResult(ctx context.Context, in string) error    { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(SimpleLinear)
	w.RegisterActivity(ValidateInput)
	w.RegisterActivity(ProcessData)
	w.RegisterActivity(SaveResult)
}
