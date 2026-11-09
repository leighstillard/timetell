package getversion_no_default

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// GetVersionNoDefault switches on the version with no case for
// DefaultVersion, so histories recorded before the change skip both checks.
func GetVersionNoDefault(ctx workflow.Context, id string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	v := workflow.GetVersion(ctx, "add-fraud-check", workflow.DefaultVersion, 2)
	switch v {
	case 1:
		if err := workflow.ExecuteActivity(ctx, FraudCheck, id).Get(ctx, nil); err != nil {
			return err
		}
	case 2:
		if err := workflow.ExecuteActivity(ctx, FraudCheckV2, id).Get(ctx, nil); err != nil {
			return err
		}
	}
	return workflow.ExecuteActivity(ctx, Charge, id).Get(ctx, nil)
}

func FraudCheck(ctx context.Context, id string) error   { return nil }
func FraudCheckV2(ctx context.Context, id string) error { return nil }
func Charge(ctx context.Context, id string) error       { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(GetVersionNoDefault)
	w.RegisterActivity(FraudCheck)
	w.RegisterActivity(FraudCheckV2)
	w.RegisterActivity(Charge)
}
