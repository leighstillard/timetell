package repeated_activity

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// RepeatedActivity calls SendEmail at two call sites with no loop between
// them: two nodes, never one node with a cycle.
func RepeatedActivity(ctx workflow.Context, user string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, SendEmail, "welcome").Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, ChargeCard, user).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, SendEmail, "receipt").Get(ctx, nil)
}

func SendEmail(ctx context.Context, kind string) error  { return nil }
func ChargeCard(ctx context.Context, user string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(RepeatedActivity)
	w.RegisterActivity(SendEmail)
	w.RegisterActivity(ChargeCard)
}
