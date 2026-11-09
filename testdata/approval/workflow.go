package approval

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const ApproveSignal = "approve"

// Approval waits up to 24h for an approve signal; ok feeds the if.
func Approval(ctx workflow.Context, req string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, SubmitRequest, req).Get(ctx, nil); err != nil {
		return err
	}
	ch := workflow.GetSignalChannel(ctx, ApproveSignal)
	//timetell:decision Wait For Approval
	ok, err := workflow.AwaitWithTimeout(ctx, 24*time.Hour, func() bool { return ch.ReceiveAsync(nil) })
	if err != nil {
		return err
	}
	if ok {
		return workflow.ExecuteActivity(ctx, ProcessApproved, req).Get(ctx, nil)
	}
	return workflow.ExecuteActivity(ctx, HandleTimeout, req).Get(ctx, nil)
}

func SubmitRequest(ctx context.Context, req string) error   { return nil }
func ProcessApproved(ctx context.Context, req string) error { return nil }
func HandleTimeout(ctx context.Context, req string) error   { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(Approval)
	w.RegisterActivity(SubmitRequest)
	w.RegisterActivity(ProcessApproved)
	w.RegisterActivity(HandleTimeout)
}
