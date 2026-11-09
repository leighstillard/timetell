package selector_loop

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	JobSignal  = "job"
	StopSignal = "stop"
)

// SelectorLoop runs a job per job signal until a stop signal arrives.
func SelectorLoop(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	stop := false
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(workflow.GetSignalChannel(ctx, JobSignal), func(c workflow.ReceiveChannel, more bool) {
		var job string
		c.Receive(ctx, &job)
		_ = workflow.ExecuteActivity(ctx, RunJob, job).Get(ctx, nil)
	})
	sel.AddReceive(workflow.GetSignalChannel(ctx, StopSignal), func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil)
		stop = true
	})
	for {
		sel.Select(ctx)
		if stop {
			break
		}
	}
	return workflow.ExecuteActivity(ctx, Cleanup).Get(ctx, nil)
}

func RunJob(ctx context.Context, job string) error { return nil }
func Cleanup(ctx context.Context) error            { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(SelectorLoop)
	w.RegisterActivity(RunJob)
	w.RegisterActivity(Cleanup)
}
