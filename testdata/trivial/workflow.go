// Package trivial is a workflow with no Temporal calls: Start straight to End.
package trivial

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func Trivial(ctx workflow.Context) error { return nil }

func Register(w worker.Worker) { w.RegisterWorkflow(Trivial) }
