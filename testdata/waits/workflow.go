package waits

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	StartSignal  = "start"
	ResumeSignal = "resumeJob"
)

func Waits(ctx workflow.Context, d time.Duration) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	workflow.GetSignalChannel(ctx, StartSignal).Receive(ctx, nil)
	_ = workflow.Sleep(ctx, 90*time.Second)
	ch := workflow.GetSignalChannel(ctx, ResumeSignal)
	_ = workflow.Await(ctx, func() bool { return ch.Len() > 0 })
	_ = workflow.NewTimer(ctx, 24*time.Hour).Get(ctx, nil)
	_ = workflow.Sleep(ctx, d)
	ok, _ := workflow.AwaitWithTimeout(ctx, time.Hour, func() bool { return false })
	if !ok {
		_ = workflow.ExecuteActivity(ctx, Escalate).Get(ctx, nil)
	}
	return workflow.ExecuteActivity(ctx, Finish).Get(ctx, nil)
}

func Escalate(ctx context.Context) error { return nil }
func Finish(ctx context.Context) error   { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(Waits)
	w.RegisterActivity(Escalate)
	w.RegisterActivity(Finish)
}
