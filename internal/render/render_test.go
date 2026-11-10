package render

import (
	"strings"
	"testing"

	"github.com/leighstillard/timetell/internal/discover"
	"github.com/leighstillard/timetell/internal/flow"
	"github.com/leighstillard/timetell/internal/golden"
	"github.com/leighstillard/timetell/internal/load"
)

// Single-workflow fixtures the builder covers so far. Later tasks add the
// rest of testdata/.
func TestGolden(t *testing.T) {
	for _, fx := range []string{"trivial", "simple_linear", "money_transfer", "saga_compensation",
		"repeated_activity", "loop_retry", "loop_range", "loop_forever", "loop_nested"} {
		t.Run(fx, func(t *testing.T) {
			pkgs, err := load.Load("../../testdata", "./"+fx)
			if err != nil {
				t.Fatal(err)
			}
			roots := discover.Discover(pkgs).Roots
			if len(roots) != 1 {
				t.Fatalf("%d roots", len(roots))
			}
			golden.Check(t, "../../testdata/"+fx+"/expected.mmd", Mermaid(flow.Build(roots[0])))
		})
	}
}

func TestLabelAndID(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"NotifyATO", "Notify ATO"}, {"Requires3DS", "Requires3DS"}, {"my-activity", `"my-activity"`},
	} {
		if got := label(&flow.Node{Kind: flow.Activity, Label: c.in}); got != c.want {
			t.Errorf("label(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	if got := label(&flow.Node{Kind: flow.Decision, Label: `say "hi"`}); got != `"say #quot;hi#quot;"` {
		t.Errorf("quote escape: %s", got)
	}
	if id("my-activity") != "my_activity" || id("end") != "end_" {
		t.Errorf("id sanitising")
	}
}

// An inner loop's done or break edge that returns to the outer loop is a
// back edge too: it gets the exit style only.
func TestExitBackEdge(t *testing.T) {
	g := &flow.Graph{
		Nodes: []*flow.Node{{ID: "l0", Kind: flow.Loop, Label: "outer"}, {ID: "l1", Kind: flow.Loop, Label: "inner", In: "l0"}, {ID: "A", Kind: flow.Activity, Label: "A", In: "l1"}},
		Edges: []flow.Edge{{From: "l0", To: "l1", Label: "do"}, {From: "l1", To: "l0", Label: "done", Back: true},
			{From: "l1", To: "A", Label: "do"}, {From: "A", To: "l0", Back: true, Break: true}},
	}
	out := string(Mermaid(g))
	for _, want := range []string{"l1 -. done .-> l0\n", "A -.-> l0\n", "linkStyle 1 stroke:#2e7d32", "linkStyle 3 stroke:#333"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "linkStyle 1,") || strings.Contains(out, "linkStyle 3 stroke:#c77700") {
		t.Errorf("exit edge also styled as back edge:\n%s", out)
	}
}

// A loop subgraph must not take the ID of an activity node.
func TestSubgraphIDClash(t *testing.T) {
	g := &flow.Graph{Nodes: []*flow.Node{{ID: "l0", Kind: flow.Loop, Label: "x"}, {ID: "L0", Kind: flow.Activity, Label: "L0", In: "l0"}}}
	if out := string(Mermaid(g)); !strings.Contains(out, "subgraph L0_ [") || !strings.Contains(out, "style L0_ ") {
		t.Errorf("subgraph ID clash:\n%s", out)
	}
}

// Edge labels that are not plain text are quoted, so "-->" or ".-" in a case
// label cannot end the arrow early.
func TestEdgeLabelQuoted(t *testing.T) {
	g := &flow.Graph{
		Nodes: []*flow.Node{{ID: "l0", Kind: flow.Loop, Label: "x"}, {ID: "d0", Kind: flow.Decision, Label: "k", In: "l0"}},
		Edges: []flow.Edge{{From: "l0", To: "d0", Label: "do"}, {From: "d0", To: "l0", Label: "`a.-b`", Back: true}, {From: "d0", To: "l0", Label: `"x-->y"`}},
	}
	out := string(Mermaid(g))
	for _, want := range []string{"d0 -.->|\"#96;a.-b#96;\"| l0\n", "d0 -->|\"#quot;x-->y#quot;\"| l0\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}
