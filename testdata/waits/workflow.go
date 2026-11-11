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
	ok = d > time.Hour // reassigned: the if below is a plain decision, not the wait's two arms
	if ok {
		_ = workflow.ExecuteActivity(ctx, Escalate).Get(ctx, nil)
	}
	_ = workflow.NewTimerWithOptions(ctx, time.Minute, workflow.TimerOptions{}).Get(ctx, nil)
	workflow.GetSignalChannelWithOptions(ctx, StartSignal, workflow.SignalChannelOptions{}).Receive(ctx, nil)
	return workflow.ExecuteActivity(ctx, Finish).Get(ctx, nil)
}

// Poll's wait branches although its Signaled edge is a back edge, and both
// arms of the if read the channel before rebinding it, so they are the same
// work and draw no decision.
func Poll(ctx workflow.Context, fast bool) error {
	ch := workflow.GetSignalChannel(ctx, StartSignal)
	if fast {
		ch.Receive(ctx, nil)
		ch = workflow.GetSignalChannel(ctx, ResumeSignal)
	} else {
		ch.Receive(ctx, nil)
		ch = workflow.GetSignalChannel(ctx, ResumeSignal)
	}
	ch.Receive(ctx, nil)
	for {
		ok, _ := workflow.AwaitWithTimeout(ctx, time.Hour, func() bool { return false })
		if !ok {
			break
		}
	}
	return nil
}

func Escalate(ctx context.Context) error { return nil }
func Finish(ctx context.Context) error   { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(Waits)
	w.RegisterWorkflow(Poll)
	w.RegisterActivity(Escalate)
	w.RegisterActivity(Finish)
}
