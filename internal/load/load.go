// Package load type-checks Go packages and classifies Temporal SDK calls by
// their types.Object, so import aliases, method values and wrappers never
// matter. timetell itself never imports the SDK.
package load

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

const (
	workflowPkg = "go.temporal.io/sdk/workflow"
	internalPkg = "go.temporal.io/sdk/internal"
	// v1.50.0 declares Context and ReceiveChannel here; internal re-aliases
	// them.
	contextPkg = "go.temporal.io/sdk/internal/common/workflowcontext"
	workerPkg  = "go.temporal.io/sdk/worker"
)

// Kind is a Temporal SDK operation.
type Kind int

const (
	None Kind = iota
	ExecuteActivity
	ExecuteLocalActivity
	ExecuteChildWorkflow
	GetSignalChannel
	Receive
	Await
	AwaitWithTimeout
	NewTimer
	Sleep
	GetVersion
	NewSelector
	AddReceive
	AddFuture
	AddDefault
	Select
	Go
	SetQueryHandler
	SetUpdateHandler
	SignalExternalWorkflow
	NewContinueAsNewError
	WithActivityOptions
	RegisterWorkflow
	RegisterActivity
)

var kindNames = [...]string{"None", "ExecuteActivity", "ExecuteLocalActivity", "ExecuteChildWorkflow",
	"GetSignalChannel", "Receive", "Await", "AwaitWithTimeout", "NewTimer", "Sleep", "GetVersion",
	"NewSelector", "AddReceive", "AddFuture", "AddDefault", "Select", "Go", "SetQueryHandler",
	"SetUpdateHandler", "SignalExternalWorkflow", "NewContinueAsNewError", "WithActivityOptions",
	"RegisterWorkflow", "RegisterActivity"}

func (k Kind) String() string { return kindNames[k] }

// Package funcs in go.temporal.io/sdk/workflow. Variants map to one Kind.
var workflowFuncs = map[string]Kind{
	"ExecuteActivity":             ExecuteActivity,
	"ExecuteLocalActivity":        ExecuteLocalActivity,
	"ExecuteChildWorkflow":        ExecuteChildWorkflow,
	"GetSignalChannel":            GetSignalChannel,
	"GetSignalChannelWithOptions": GetSignalChannel,
	"Await":                       Await,
	"AwaitWithTimeout":            AwaitWithTimeout,
	"AwaitWithOptions":            AwaitWithTimeout,
	"NewTimer":                    NewTimer,
	"NewTimerWithOptions":         NewTimer,
	"Sleep":                       Sleep,
	"GetVersion":                  GetVersion,
	"NewSelector":                 NewSelector,
	"NewNamedSelector":            NewSelector,
	"Go":                          Go,
	"GoNamed":                     Go,
	"SetQueryHandler":             SetQueryHandler,
	"SetUpdateHandler":            SetUpdateHandler,
	"SetUpdateHandlerWithOptions": SetUpdateHandler,
	"SignalExternalWorkflow":      SignalExternalWorkflow,
	"NewContinueAsNewError":       NewContinueAsNewError,
	"WithActivityOptions":         WithActivityOptions,
}

// Methods on the internal interfaces that workflow.Selector and
// workflow.ReceiveChannel alias (matched by declaring package, so the alias
// chain does not matter).
var internalMethods = map[string]Kind{
	"Receive":    Receive,
	"AddReceive": AddReceive,
	"AddFuture":  AddFuture,
	"AddDefault": AddDefault,
	"Select":     Select,
}

// Methods on worker.WorkflowRegistry / worker.ActivityRegistry.
var workerMethods = map[string]Kind{
	"RegisterWorkflow":            RegisterWorkflow,
	"RegisterWorkflowWithOptions": RegisterWorkflow,
	"RegisterActivity":            RegisterActivity,
	"RegisterActivityWithOptions": RegisterActivity,
}

func isInternal(path string) bool { return path == internalPkg || path == contextPkg }

// IsWorkflowContext reports whether t is workflow.Context, whichever
// internal package the alias chain ends in.
func IsWorkflowContext(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && isInternal(n.Obj().Pkg().Path()) && n.Obj().Name() == "Context"
}

// Load type-checks patterns relative to dir.
func Load(dir string, patterns ...string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
		Dir:   dir,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		return nil, fmt.Errorf("%d package error(s)", n)
	}
	return pkgs, nil
}

// Callee is the object a call invokes, or nil (calls of func values,
// conversions, builtins).
func Callee(info *types.Info, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.IndexExpr: // generic instantiation
		return Callee(info, &ast.CallExpr{Fun: f.X})
	default:
		return nil
	}
	fn, _ := info.Uses[id].(*types.Func)
	return fn
}

// Classify reports which SDK operation call is, or None.
func Classify(info *types.Info, call *ast.CallExpr) Kind {
	fn := Callee(info, call)
	if fn == nil || fn.Pkg() == nil {
		return None
	}
	isMethod := fn.Signature().Recv() != nil
	switch path := fn.Pkg().Path(); {
	case path == workflowPkg && !isMethod:
		return workflowFuncs[fn.Name()]
	case isInternal(path) && isMethod:
		return internalMethods[fn.Name()]
	case path == workerPkg && isMethod:
		return workerMethods[fn.Name()]
	}
	return None
}

// Target is what an activity or child workflow call starts.
type Target struct {
	Name string      // "?" when not static
	Obj  *types.Func // nil for a string name
}

// ResolveTarget resolves the activity/workflow argument: a func, a method
// value (name = method name) or a constant string.
func ResolveTarget(info *types.Info, e ast.Expr) (Target, bool) {
	e = ast.Unparen(e)
	if s, ok := ConstString(info, e); ok {
		return Target{Name: s}, true
	}
	var id *ast.Ident
	switch x := e.(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	}
	if id != nil {
		if fn, ok := info.Uses[id].(*types.Func); ok {
			return Target{Name: fn.Name(), Obj: fn}, true
		}
	}
	return Target{Name: "?"}, false
}

// ConstString folds e to a string constant if the type checker could.
func ConstString(info *types.Info, e ast.Expr) (string, bool) {
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// Warning is a positioned note for the validation report.
type Warning struct {
	Pos token.Position
	Msg string
}

func (w Warning) String() string { return fmt.Sprintf("%s: %s", w.Pos, w.Msg) }
