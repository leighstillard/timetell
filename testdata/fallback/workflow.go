// Package fallback has no worker registration, so roots are the exported
// funcs whose first parameter is workflow.Context.
package fallback

import (
	"context"
	"time"

	"go.temporal.io/sdk/workflow"
)

func Fallback(ctx workflow.Context, id string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	return workflow.ExecuteActivity(ctx, Charge, id).Get(ctx, nil)
}

// helper is unexported, so it is not a root.
func helper(ctx workflow.Context) {}

func Charge(ctx context.Context, id string) error { return nil }
