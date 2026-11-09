package signal_flow

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	ShipOrderSignal     = "ship_order"
	NotifyShippedSignal = "notify_shipped"
)

// Order -> Shipping -> Notification, chained by SignalExternalWorkflow.
func OrderWorkflow(ctx workflow.Context, orderID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, ProcessOrder, orderID).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.SignalExternalWorkflow(ctx, "shipping-"+orderID, "", ShipOrderSignal, orderID).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, CompleteOrder, orderID).Get(ctx, nil)
}

func ShippingWorkflow(ctx workflow.Context, shippingID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	var orderID string
	workflow.GetSignalChannel(ctx, ShipOrderSignal).Receive(ctx, &orderID)
	if err := workflow.ExecuteActivity(ctx, ShipPackage, orderID).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.SignalExternalWorkflow(ctx, "notification-"+orderID, "", NotifyShippedSignal, orderID).Get(ctx, nil)
}

func NotificationWorkflow(ctx workflow.Context, notificationID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	var orderID string
	workflow.GetSignalChannel(ctx, NotifyShippedSignal).Receive(ctx, &orderID)
	return workflow.ExecuteActivity(ctx, SendNotification, orderID).Get(ctx, nil)
}

func ProcessOrder(ctx context.Context, id string) error     { return nil }
func CompleteOrder(ctx context.Context, id string) error    { return nil }
func ShipPackage(ctx context.Context, id string) error      { return nil }
func SendNotification(ctx context.Context, id string) error { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(OrderWorkflow)
	w.RegisterWorkflow(ShippingWorkflow)
	w.RegisterWorkflow(NotificationWorkflow)
	w.RegisterActivity(ProcessOrder)
	w.RegisterActivity(CompleteOrder)
	w.RegisterActivity(ShipPackage)
	w.RegisterActivity(SendNotification)
}
