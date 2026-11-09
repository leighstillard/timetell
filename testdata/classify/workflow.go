// Package classify exercises SDK call classification: an aliased import, a
// method value, const names and a dynamic activity name.
package classify

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	wf "go.temporal.io/sdk/workflow"
)

type Activities struct{}

func (a *Activities) Charge(ctx context.Context, id string) error { return nil }

const (
	RefundName    = "Refund"
	ApproveSignal = "approve"
)

func Classify(ctx wf.Context, kind string) error {
	ctx = wf.WithActivityOptions(ctx, wf.ActivityOptions{StartToCloseTimeout: time.Minute})
	var a *Activities
	_ = wf.ExecuteActivity(ctx, a.Charge, "x").Get(ctx, nil)
	_ = wf.ExecuteActivity(ctx, RefundName, "x").Get(ctx, nil)
	_ = wf.ExecuteActivity(ctx, "Notify", "x").Get(ctx, nil)
	_ = wf.ExecuteActivity(ctx, kind, "x").Get(ctx, nil)
	wf.GetSignalChannel(ctx, ApproveSignal).Receive(ctx, nil)
	sel := wf.NewSelector(ctx)
	sel.AddReceive(wf.GetSignalChannel(ctx, "stop"), func(c wf.ReceiveChannel, more bool) {})
	sel.Select(ctx)
	return nil
}

func Register(w worker.Worker) {
	w.RegisterWorkflow(Classify)
	w.RegisterActivity(&Activities{})
}
