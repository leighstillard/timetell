package loop_nested

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// LoopNested charges each order with up to three retries, ships the ones
// that need it, then sends a report.
func LoopNested(ctx workflow.Context, orders []string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	for _, order := range orders {
		var err error
		//timetell:decision retry charge
		for attempt := 1; attempt <= 3; attempt++ {
			err = workflow.ExecuteActivity(ctx, ChargeCard, order).Get(ctx, nil)
			if err == nil {
				break
			}
		}
		if err != nil {
			return err
		}
		var ship bool
		if err := workflow.ExecuteActivity(ctx, NeedsShipping, order).Get(ctx, &ship); err != nil {
			return err
		}
		if ship {
			if err := workflow.ExecuteActivity(ctx, ShipOrder, order).Get(ctx, nil); err != nil {
				return err
			}
		}
	}
	return workflow.ExecuteActivity(ctx, SendReport).Get(ctx, nil)
}

func ChargeCard(ctx context.Context, order string) error            { return nil }
func NeedsShipping(ctx context.Context, order string) (bool, error) { return false, nil }
func ShipOrder(ctx context.Context, order string) error             { return nil }
func SendReport(ctx context.Context) error                          { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(LoopNested)
	w.RegisterActivity(ChargeCard)
	w.RegisterActivity(NeedsShipping)
	w.RegisterActivity(ShipOrder)
	w.RegisterActivity(SendReport)
}
