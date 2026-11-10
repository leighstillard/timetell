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
