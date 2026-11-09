package money_transfer

import (
	"context"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type Transfer struct {
	From, To       string
	SourceCurrency string
	DestCurrency   string
	TFNKnown       bool
}

// MoneyTransfer checks the error after every activity, as idiomatic Go does.
// None of those checks may become a decision.
func MoneyTransfer(ctx workflow.Context, t Transfer) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	if err := workflow.ExecuteActivity(ctx, WithdrawFunds, t).Get(ctx, nil); err != nil {
		return err
	}
	//timetell:decision Need To Convert
	if t.SourceCurrency != t.DestCurrency {
		if err := workflow.ExecuteActivity(ctx, CurrencyConvert, t).Get(ctx, nil); err != nil {
			return err
		}
	}
	//timetell:decision Is TFN Known
	if t.TFNKnown {
		if err := workflow.ExecuteActivity(ctx, NotifyATO, t).Get(ctx, nil); err != nil {
			return err
		}
	} else {
		if err := workflow.ExecuteActivity(ctx, TakeNonResidentTax, t).Get(ctx, nil); err != nil {
			return err
		}
	}
	return workflow.ExecuteActivity(ctx, DepositFunds, t).Get(ctx, nil)
}

func WithdrawFunds(ctx context.Context, t Transfer) error      { return nil }
func CurrencyConvert(ctx context.Context, t Transfer) error    { return nil }
func NotifyATO(ctx context.Context, t Transfer) error          { return nil }
func TakeNonResidentTax(ctx context.Context, t Transfer) error { return nil }
func DepositFunds(ctx context.Context, t Transfer) error       { return nil }

func Register(w worker.Worker) {
	w.RegisterWorkflow(MoneyTransfer)
	w.RegisterActivity(WithdrawFunds)
	w.RegisterActivity(CurrencyConvert)
	w.RegisterActivity(NotifyATO)
	w.RegisterActivity(TakeNonResidentTax)
	w.RegisterActivity(DepositFunds)
}
