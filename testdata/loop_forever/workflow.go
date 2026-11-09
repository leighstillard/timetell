package loop_forever

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// LoopForever polls for jobs until there are none, continuing as new when
// the history grows too long.
func LoopForever(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	for {
		var done bool
		if err := workflow.ExecuteActivity(ctx, FetchJob).Get(ctx, &done); err != nil {
			return err
		}
		if done {
			break
		}
		if err := workflow.ExecuteActivity(ctx, RunJob).Get(ctx, nil); err != nil {
			return err
		}
		//timetell:decision History Full
		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			return workflow.NewContinueAsNewError(ctx, LoopForever)
		}
	}
	return workflow.ExecuteActivity(ctx, Finish).Get(ctx, nil)
}

func FetchJob(ctx context.Context) (bool, error) { return true, nil }
func RunJob(ctx context.Context) error           { return nil }
func Finish(ctx context.Context) error           { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(LoopForever)
	w.RegisterActivity(FetchJob)
	w.RegisterActivity(RunJob)
	w.RegisterActivity(Finish)
}
