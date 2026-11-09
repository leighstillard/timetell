package load

import (
	"go/ast"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	pkgs, err := Load("../../testdata", "./classify")
	if err != nil {
		t.Fatal(err)
	}
	p := pkgs[0]
	var got []string
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			k := Classify(p.TypesInfo, call)
			switch k {
			case None:
				return true
			case ExecuteActivity:
				tg, ok := ResolveTarget(p.TypesInfo, call.Args[1])
				if !ok {
					pos := p.Fset.Position(call.Args[1].Pos())
					got = append(got, k.String()+" ? line "+strconv.Itoa(pos.Line))
					return true
				}
				got = append(got, k.String()+" "+tg.Name)
			case GetSignalChannel:
				s, _ := ConstString(p.TypesInfo, call.Args[1])
				got = append(got, k.String()+" "+s)
			default:
				got = append(got, k.String())
			}
			return true
		})
	}
	want := []string{
		"WithActivityOptions",
		"ExecuteActivity Charge",
		"ExecuteActivity Refund",
		"ExecuteActivity Notify",
		"ExecuteActivity ? line 28",
		"Receive", "GetSignalChannel approve",
		"NewSelector",
		"AddReceive", "GetSignalChannel stop",
		"Select",
		"RegisterWorkflow", "RegisterActivity",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
