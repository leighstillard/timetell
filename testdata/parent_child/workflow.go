package parent_child

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type Order struct {
	ID       string
	Amount   int
	Needs3DS bool
}

func OrderWorkflow(ctx workflow.Context, o Order) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, ValidateOrder, o).Get(ctx, nil); err != nil {
		return err
	}
	//timetell:decision High Value
	if o.Amount > 10000 {
		if err := workflow.ExecuteActivity(ctx, ManagerApproval, o).Get(ctx, nil); err != nil {
			return err
		}
	}
	if err := workflow.ExecuteChildWorkflow(ctx, PaymentWorkflow, o).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, SendConfirmation, o).Get(ctx, nil)
}

func PaymentWorkflow(ctx workflow.Context, o Order) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, ProcessPayment, o).Get(ctx, nil); err != nil {
		return err
	}
	//timetell:decision Requires 3DS
	if o.Needs3DS {
		return workflow.ExecuteActivity(ctx, Verify3DS, o).Get(ctx, nil)
	}
	return nil
}

func ValidateOrder(ctx context.Context, o Order) error    { return nil }
func ManagerApproval(ctx context.Context, o Order) error  { return nil }
func SendConfirmation(ctx context.Context, o Order) error { return nil }
func ProcessPayment(ctx context.Context, o Order) error   { return nil }
func Verify3DS(ctx context.Context, o Order) error        { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(OrderWorkflow)
	w.RegisterWorkflow(PaymentWorkflow)
	w.RegisterActivity(ValidateOrder)
	w.RegisterActivity(ManagerApproval)
	w.RegisterActivity(SendConfirmation)
	w.RegisterActivity(ProcessPayment)
	w.RegisterActivity(Verify3DS)
}
