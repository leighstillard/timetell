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

// An inner loop's done edge that returns to the outer loop is a back edge
// too: it gets the done style only.
func TestDoneBackEdge(t *testing.T) {
	g := &flow.Graph{
		Nodes: []*flow.Node{{ID: "l0", Kind: flow.Loop, Label: "outer"}, {ID: "l1", Kind: flow.Loop, Label: "inner", In: "l0"}},
		Edges: []flow.Edge{{From: "l0", To: "l1", Label: "do"}, {From: "l1", To: "l0", Label: "done", Back: true}},
	}
	out := string(Mermaid(g))
	if !strings.Contains(out, "l1 -. done .-> l0\n") || !strings.Contains(out, "linkStyle 1 stroke:#2e7d32") || strings.Contains(out, "linkStyle 1 stroke:#c77700") {
		t.Errorf("done back edge styling:\n%s", out)
	}
}
