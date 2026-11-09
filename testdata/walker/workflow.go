// Package walker holds one small workflow per control-flow case in
// docs/control-flow.md. No Register func: roots come from the fallback.
package walker

import (
	"context"
	"errors"

	"go.temporal.io/sdk/workflow"
)

func A(ctx context.Context) error { return nil }
func B(ctx context.Context) error { return nil }
func C(ctx context.Context) error { return nil }

func isOK(err error) bool { return err == nil }

// An if with no Temporal calls in either arm draws no decision.
func NoEffectIf(ctx workflow.Context, x int) error {
	if x > 1 {
		x++
	} else {
		x--
	}
	return workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
}

// A call in Init comes before the decision; a && b is one decision.
func InitAndCond(ctx workflow.Context) error {
	var ok bool
	if err := workflow.ExecuteActivity(ctx, A).Get(ctx, &ok); err == nil && ok {
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	}
	return nil
}

// A call in Cond comes before the decision.
func CallInCond(ctx workflow.Context) error {
	if isOK(workflow.ExecuteActivity(ctx, A).Get(ctx, nil)) {
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	}
	return nil
}

// A call in the right operand of && is drawn, with a warning.
func RightOperand(ctx workflow.Context, x bool) error {
	if x && workflow.ExecuteActivity(ctx, A).Get(ctx, nil) == nil {
		return workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	}
	return nil
}

// A validation early return is a decision.
func Validate(ctx workflow.Context, in string) error {
	if in == "" {
		return errors.New("empty")
	}
	return workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
}

func Fallthrough(ctx workflow.Context, n int) error {
	switch n {
	case 1:
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
		fallthrough
	case 2:
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	}
	return nil
}

func Goto(ctx workflow.Context) error {
	_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	goto done
done:
	return workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
}

func AfterReturn(ctx workflow.Context) error {
	return workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	return nil
}

func Labelled(ctx workflow.Context, xs []int) error {
outer:
	for _, x := range xs {
		for i := 0; i < x; i++ {
			if i == 1 {
				continue outer
			}
			if i == 2 {
				break outer
			}
			_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
		}
	}
	return workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
}

// The break leaves the switch, not the loop.
func BreakInSwitch(ctx workflow.Context, xs []int) error {
	for _, x := range xs {
		switch x {
		case 1:
			if x > 0 {
				break
			}
			_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
		}
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	}
	return nil
}

// Open items (timetell-pas.25-27): each warns, none is drawn.
func Unsupported(ctx workflow.Context) error {
	defer func() { _ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil) }()
	compensate := func() { _ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil) }
	compensate()
	go func() { _ = workflow.ExecuteActivity(ctx, C).Get(ctx, nil) }()
	ch := make(chan int)
	select {
	case <-ch:
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	}
	return nil
}

// Arms with identical effects collapse: no decision.
func SameArms(ctx workflow.Context, x, y int) error {
	if x > 1 {
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	} else if y > 1 {
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	} else {
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	}
	switch x {
	case 1:
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	default:
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	}
	return nil
}

// Same calls, different structure or a missing default: decisions stay.
func DiffArms(ctx workflow.Context, x, y int) error {
	if x > 1 {
		if y > 1 {
			_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
		}
	} else {
		_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
	}
	switch x {
	case 1:
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	case 2:
		_ = workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
	}
	return nil
}

// Identical arms that both leave the loop collapse; the break still exits.
func SameArmsBreak(ctx workflow.Context, x int) error {
	for x > 0 {
		if x > 1 {
			_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
			break
		} else {
			_ = workflow.ExecuteActivity(ctx, A).Get(ctx, nil)
			break
		}
	}
	return workflow.ExecuteActivity(ctx, B).Get(ctx, nil)
}
