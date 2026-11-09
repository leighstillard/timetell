package flow

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/leighstillard/timetell/internal/discover"
	"github.com/leighstillard/timetell/internal/load"
)

func build(t *testing.T, fixture string) map[string]*Graph {
	t.Helper()
	pkgs, err := load.Load("../../testdata", "./"+fixture)
	if err != nil {
		t.Fatal(err)
	}
	gs := map[string]*Graph{}
	for _, r := range discover.Discover(pkgs).Roots {
		gs[r.Obj.Name()] = Build(r)
	}
	return gs
}

func edges(g *Graph) []string {
	var out []string
	for _, e := range g.Edges {
		out = append(out, e.From+" -"+e.Label+"-> "+e.To)
	}
	return out
}

func count(g *Graph, k Kind) int {
	n := 0
	for _, x := range g.Nodes {
		if x.Kind == k {
			n++
		}
	}
	return n
}

var mmdEdge = regexp.MustCompile(`^(\S+) (?:-- (.+) )?--> (\S+)$`)

// goldenEdges reads the edges of expected.mmd in the same form as edges().
func goldenEdges(t *testing.T, fixture string) []string {
	b, err := os.ReadFile("../../testdata/" + fixture + "/expected.mmd")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if m := mmdEdge.FindStringSubmatch(l); m != nil {
			out = append(out, m[1]+" -"+m[2]+"-> "+m[3])
		}
	}
	return out
}

// Single-workflow fixtures whose shape only needs this builder: edges must
// match expected.mmd exactly, in order.
func TestFixtureEdges(t *testing.T) {
	for _, fx := range []string{"simple_linear", "money_transfer", "saga_compensation", "repeated_activity",
		"loop_retry", "loop_range", "loop_forever"} {
		t.Run(fx, func(t *testing.T) {
			gs := build(t, fx)
			if len(gs) != 1 {
				t.Fatalf("%d roots", len(gs))
			}
			for _, g := range gs {
				if got, want := edges(g), goldenEdges(t, fx); !slices.Equal(got, want) {
					t.Errorf("edges\n got %q\nwant %q", got, want)
				}
				if len(g.Warnings) > 0 {
					t.Errorf("warnings: %v", g.Warnings)
				}
			}
		})
	}
}

func TestAcceptance(t *testing.T) {
	mt := build(t, "money_transfer")["MoneyTransfer"]
	if a, d := count(mt, Activity), count(mt, Decision); a != 5 || d != 2 {
		t.Errorf("money_transfer: %d activities, %d decisions", a, d)
	}
	in := 0
	for _, e := range mt.Edges {
		if e.To == "DepositFunds" {
			in++
		}
	}
	if in != 2 {
		t.Errorf("money_transfer: DepositFunds has %d in-edges, want 2 (reconverging)", in)
	}
	for _, n := range mt.Nodes {
		if strings.Contains(n.Label, "err") {
			t.Errorf("money_transfer: err decision %q", n.Label)
		}
	}
	saga := build(t, "saga_compensation")["SagaCompensation"]
	if !slices.Contains(edges(saga), "ChargePayment -error-> ReleaseInventory") || count(saga, Decision) != 0 {
		t.Errorf("saga: %q", edges(saga))
	}
	ra := build(t, "repeated_activity")["RepeatedActivity"]
	var ids []string
	for _, n := range ra.Nodes {
		if n.Label == "SendEmail" {
			ids = append(ids, n.ID)
		}
	}
	if !slices.Equal(ids, []string{"SendEmail", "SendEmail_2"}) {
		t.Errorf("repeated_activity: %v", ids)
	}
	lr := build(t, "loop_retry")["LoopRetry"]
	back := 0
	for _, e := range lr.Edges {
		if e.Back {
			back++
		}
	}
	if back != 1 {
		t.Errorf("loop_retry: %d back edges, want 1", back)
	}
}

func TestWalker(t *testing.T) {
	gs := build(t, "walker")
	cases := []struct {
		wf       string
		edges    []string
		warnings int
	}{
		{"NoEffectIf", []string{"s --> A", "A --> e"}, 0},
		{"InitAndCond", []string{"s --> A", "A --> d0", "d0 -yes-> B", "B --> e", "d0 -no-> e"}, 0},
		{"CallInCond", []string{"s --> A", "A --> d0", "d0 -yes-> B", "B --> e", "d0 -no-> e"}, 0},
		{"RightOperand", []string{"s --> A", "A --> d0", "d0 -yes-> B", "B --> e", "d0 -no-> e"}, 1},
		{"Validate", []string{"s --> d0", "d0 -yes-> e", "d0 -no-> A", "A --> e"}, 0},
		{"Fallthrough", []string{"s --> d0", "d0 -1-> A", "d0 -2-> B", "A --> B", "B --> e", "d0 -other-> e"}, 0},
		{"Goto", []string{"s --> A", "A --> q0"}, 1},
		{"AfterReturn", []string{"s --> A", "A --> e"}, 0},
		{"Labelled", []string{"s --> l0", "l0 -do-> l1", "l1 -do-> d0", "d0 -no-> d1", "d1 -no-> A", "A -loop-> l1",
			"l1 -done-> l0", "d0 -yes-> l0", "l0 -done-> B", "d1 -yes-> B", "B --> e"}, 0},
		{"BreakInSwitch", []string{"s --> l0", "l0 -do-> d0", "d0 -1-> d1", "d1 -no-> B", "B --> A", "d1 -yes-> A",
			"d0 -other-> A", "A -loop-> l0", "l0 -done-> e"}, 0},
		{"Unsupported", []string{"s --> e"}, 4},
		{"SameArmsBreak", []string{"s --> l0", "l0 -do-> A", "l0 -done-> B", "A --> B", "B --> e"}, 0},
		{"SameArms", []string{"s --> A", "A --> B", "B --> e"}, 0},
		{"DiffArms", []string{"s --> d0", "d0 -yes-> d1", "d1 -yes-> A", "d0 -no-> A_2", "A --> d2", "d1 -no-> d2",
			"A_2 --> d2", "d2 -1-> B", "d2 -2-> B_2", "B --> e", "B_2 --> e", "d2 -other-> e"}, 0},
	}
	for _, c := range cases {
		g := gs[c.wf]
		if g == nil {
			t.Errorf("%s: not a root", c.wf)
			continue
		}
		if got := edges(g); !slices.Equal(got, c.edges) {
			t.Errorf("%s edges\n got %q\nwant %q", c.wf, got, c.edges)
		}
		if len(g.Warnings) != c.warnings {
			t.Errorf("%s: warnings %v, want %d", c.wf, g.Warnings, c.warnings)
		}
	}
	if w := gs["Goto"].Warnings; len(w) == 1 && w[0].Pos.Line == 0 {
		t.Errorf("goto warning has no position")
	}
}
