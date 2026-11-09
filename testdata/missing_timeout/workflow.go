package missing_timeout

import (
	"context"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// MissingTimeout sets activity options with no StartToClose or
// ScheduleToClose timeout, which the SDK rejects at run time.
func MissingTimeout(ctx workflow.Context, id string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: "billing"})
	return workflow.ExecuteActivity(ctx, Charge, id).Get(ctx, nil)
}

func Charge(ctx context.Context, id string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(MissingTimeout)
	w.RegisterActivity(Charge)
}
