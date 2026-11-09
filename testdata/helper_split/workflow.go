package helper_split

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// HelperSplit spreads its activities over unexported helpers, as agents do.
func HelperSplit(ctx workflow.Context, orderID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := reserve(ctx, orderID); err != nil {
		return err
	}
	return ship(ctx, orderID)
}

func reserve(ctx workflow.Context, orderID string) error {
	return workflow.ExecuteActivity(ctx, Reserve, orderID).Get(ctx, nil)
}

func ship(ctx workflow.Context, orderID string) error {
	if err := workflow.ExecuteActivity(ctx, Pack, orderID).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, Dispatch, orderID).Get(ctx, nil)
}

func Reserve(ctx context.Context, id string) error  { return nil }
func Pack(ctx context.Context, id string) error     { return nil }
func Dispatch(ctx context.Context, id string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(HelperSplit)
	w.RegisterActivity(Reserve)
	w.RegisterActivity(Pack)
	w.RegisterActivity(Dispatch)
}
