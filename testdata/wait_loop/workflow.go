package wait_loop

import (
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	StartSignal  = "start"
	ResumeSignal = "resumeJob"
)

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

func Register(w worker.Worker) { w.RegisterWorkflow(Poll) }
