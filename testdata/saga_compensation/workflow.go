package saga_compensation

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// SagaCompensation releases the reservation when the charge fails. That
// error arm does work, so it is drawn as an 'error' edge, not a diamond.
func SagaCompensation(ctx workflow.Context, orderID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, ReserveInventory, orderID).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, ChargePayment, orderID).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, ReleaseInventory, orderID).Get(ctx, nil)
		return err
	}
	return workflow.ExecuteActivity(ctx, ShipOrder, orderID).Get(ctx, nil)
}

func ReserveInventory(ctx context.Context, id string) error { return nil }
func ChargePayment(ctx context.Context, id string) error    { return nil }
func ReleaseInventory(ctx context.Context, id string) error { return nil }
func ShipOrder(ctx context.Context, id string) error        { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(SagaCompensation)
	w.RegisterActivity(ReserveInventory)
	w.RegisterActivity(ChargePayment)
	w.RegisterActivity(ReleaseInventory)
	w.RegisterActivity(ShipOrder)
}
