package selector_signal_timer

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const ApproveSignal = "approve"

// SelectorSignalTimer waits for an approve signal or a 24h timer.
func SelectorSignalTimer(ctx workflow.Context, req string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	timer := workflow.NewTimer(ctx, 24*time.Hour)
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(workflow.GetSignalChannel(ctx, ApproveSignal), func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, Approve, req).Get(ctx, nil)
	})
	sel.AddFuture(timer, func(f workflow.Future) {
		_ = workflow.ExecuteActivity(ctx, Escalate, req).Get(ctx, nil)
	})
	sel.Select(ctx)
	return workflow.ExecuteActivity(ctx, Close, req).Get(ctx, nil)
}

func Approve(ctx context.Context, req string) error  { return nil }
func Escalate(ctx context.Context, req string) error { return nil }
func Close(ctx context.Context, req string) error    { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(SelectorSignalTimer)
	w.RegisterActivity(Approve)
	w.RegisterActivity(Escalate)
	w.RegisterActivity(Close)
}
