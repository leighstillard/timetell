package flow

import (
	"fmt"
	"math/big"
	"testing"
)

// 30 sequential diamonds: 2^30 paths, counted without enumerating.
func TestPathsCountNoEnumerate(t *testing.T) {
	g := &Graph{Nodes: []*Node{{ID: "s"}, {ID: "e"}}}
	prev := "s"
	for i := range 30 {
		d, a := fmt.Sprintf("d%d", i), fmt.Sprintf("a%d", i)
		g.Nodes = append(g.Nodes, &Node{ID: d, Kind: Decision}, &Node{ID: a, Kind: Activity})
		next := fmt.Sprintf("j%d", i)
		g.Nodes = append(g.Nodes, &Node{ID: next, Kind: Activity})
		g.Edges = append(g.Edges, Edge{From: prev, To: d}, Edge{From: d, To: a}, Edge{From: a, To: next}, Edge{From: d, To: next})
		prev = next
	}
	g.Edges = append(g.Edges, Edge{From: prev, To: "e"})
	total, list := g.Paths(3)
	if want := new(big.Int).Lsh(big.NewInt(1), 30); total.Cmp(want) != 0 || len(list) != 3 {
		t.Errorf("total %s, listed %d; want %s, 3", total, len(list), want)
	}
}

func TestPathsLoops(t *testing.T) {
	for fixture, want := range map[string]int64{
		"loop_retry":   3, // 0 iterations, or 1 that breaks or retries
		"loop_range":   2,
		"loop_forever": 2, // no exits to iterate into: continue-as-new or break
	} {
		for name, g := range build(t, fixture) {
			total, list := g.Paths(1 << 20)
			if int64(len(list)) != total.Int64() {
				t.Errorf("%s: listed %d of %s", name, len(list), total)
			}
			if total.Int64() != want {
				t.Errorf("%s: %s paths, want %d", name, total, want)
			}
		}
	}
}

// An inner loop that ends the outer body leaves by an outer back edge:
// outer 0 times, outer 1 with inner 0, outer 1 with inner 1.
func TestPathsTailNestedLoop(t *testing.T) {
	g := &Graph{
		Nodes: []*Node{{ID: "s"}, {ID: "l0", Kind: Loop}, {ID: "l1", Kind: Loop, In: "l0"}, {ID: "A", Kind: Activity, In: "l1"}, {ID: "e"}},
		Edges: []Edge{{From: "s", To: "l0"}, {From: "l0", To: "l1"}, {From: "l1", To: "A"},
			{From: "A", To: "l1", Back: true}, {From: "l1", To: "l0", Back: true}, {From: "l0", To: "e"}},
	}
	if total, list := g.Paths(10); total.Int64() != 3 || len(list) != 3 {
		t.Errorf("total %s, listed %d; want 3", total, len(list))
	}
}
