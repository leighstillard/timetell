// Package discover finds workflow roots, registered activities and the
// signals each workflow receives.
package discover

import (
	"cmp"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"github.com/leighstillard/timetell/internal/load"
	"golang.org/x/tools/go/packages"
)

// Func is a function declaration with the package that type-checked it.
type Func struct {
	Obj  *types.Func
	Decl *ast.FuncDecl
	Pkg  *packages.Package
}

// Index is what discovery found across the loaded packages.
type Index struct {
	Roots      []*Func // workflows, sorted by name
	Activities []*types.Func
	// Fallback is set when no RegisterWorkflow call was found and roots are
	// the exported funcs whose first parameter is workflow.Context.
	Fallback bool
	// Registered is false when the packages contain no Register* call at
	// all; the registration lints are skipped then.
	Registered bool
	Funcs      map[*types.Func]*Func // every func declaration with a body
	Signals    map[*types.Func][]string
}

// Discover indexes pkgs.
func Discover(pkgs []*packages.Package) *Index {
	ix := &Index{Funcs: map[*types.Func]*Func{}, Signals: map[*types.Func][]string{}}
	roots := map[*types.Func]bool{}
	acts := map[*types.Func]bool{}
	sawWorkflowReg := false
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					if obj, ok := p.TypesInfo.Defs[fd.Name].(*types.Func); ok {
						ix.Funcs[obj] = &Func{Obj: obj, Decl: fd, Pkg: p}
					}
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				switch load.Classify(p.TypesInfo, call) {
				case load.RegisterWorkflow:
					sawWorkflowReg = true
					ix.Registered = true
					if t, ok := load.ResolveTarget(p.TypesInfo, call.Args[0]); ok && t.Obj != nil {
						roots[t.Obj] = true
					}
				case load.RegisterActivity:
					ix.Registered = true
					if t, ok := load.ResolveTarget(p.TypesInfo, call.Args[0]); ok && t.Obj != nil {
						acts[t.Obj] = true
					} else if typ := p.TypesInfo.TypeOf(call.Args[0]); typ != nil {
						// A struct (pointer): every exported method is an activity.
						ms := types.NewMethodSet(typ)
						for i := range ms.Len() {
							if fn, ok := ms.At(i).Obj().(*types.Func); ok && fn.Exported() {
								acts[fn] = true
							}
						}
					}
				}
				return true
			})
		}
	}
	if !sawWorkflowReg {
		ix.Fallback = true
		for obj := range ix.Funcs {
			if obj.Exported() && obj.Signature().Recv() == nil && takesWorkflowContext(obj) {
				roots[obj] = true
			}
		}
	}
	for obj := range roots {
		if fn := ix.Funcs[obj]; fn != nil {
			ix.Roots = append(ix.Roots, fn)
			ix.Signals[obj] = receivedSignals(fn)
		}
	}
	slices.SortFunc(ix.Roots, func(a, b *Func) int {
		return cmp.Or(strings.Compare(a.Obj.Name(), b.Obj.Name()), strings.Compare(a.Pkg.PkgPath, b.Pkg.PkgPath))
	})
	for obj := range acts {
		ix.Activities = append(ix.Activities, obj)
	}
	slices.SortFunc(ix.Activities, func(a, b *types.Func) int { return strings.Compare(a.Name(), b.Name()) })
	return ix
}

// takesWorkflowContext reports whether fn's first parameter is
// workflow.Context.
func takesWorkflowContext(fn *types.Func) bool {
	ps := fn.Signature().Params()
	return ps.Len() > 0 && load.IsWorkflowContext(ps.At(0).Type())
}

// receivedSignals lists the constant signal names fn opens with
// GetSignalChannel, sorted and deduplicated.
func receivedSignals(fn *Func) []string {
	var names []string
	ast.Inspect(fn.Decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && len(call.Args) > 1 && load.Classify(fn.Pkg.TypesInfo, call) == load.GetSignalChannel {
			if s, ok := load.ConstString(fn.Pkg.TypesInfo, call.Args[1]); ok {
				names = append(names, s)
			}
		}
		return true
	})
	slices.Sort(names)
	return slices.Compact(names)
}
