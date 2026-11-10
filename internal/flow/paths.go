package flow

import "math/big"

// Paths counts the Start→End paths of g by DP, skipping back edges, so a
// loop is taken 0 or 1 times. It lists at most max of them, each as its
// nodes in order, in edge creation order; the count never enumerates.
func (g *Graph) Paths(max int) (*big.Int, [][]*Node) {
	byID := map[string]*Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	out := map[string][]string{}
	for _, e := range g.Edges {
		if !e.Back {
			out[e.From] = append(out[e.From], e.To)
		}
	}
	// A back edge ends the one iteration: it continues to the loop's exits,
	// the head's out-edges that leave the body. A forever loop has none. An
	// inner loop that ends an outer body exits by the outer back edge, so
	// its exits are the outer loop's.
	var exits func(l string) []string
	exits = func(l string) (x []string) {
		for _, e := range g.Edges { // ponytail: O(E) per back edge and nesting level; fine at workflow size
			switch {
			case e.From != l:
			case !e.Back && byID[e.To].In != l:
				x = append(x, e.To)
			case e.Back && e.To != l:
				x = append(x, exits(e.To)...)
			}
		}
		return x
	}
	for _, e := range g.Edges {
		if e.Back {
			out[e.From] = append(out[e.From], exits(e.To)...)
		}
	}
	memo := map[string]*big.Int{}
	var count func(string) *big.Int
	count = func(id string) *big.Int {
		if c, ok := memo[id]; ok {
			return c
		}
		c := new(big.Int)
		if id == "e" {
			c.SetInt64(1)
		}
		memo[id] = c // ponytail: back edges excluded, so the rest is a DAG
		for _, to := range out[id] {
			c.Add(c, count(to))
		}
		return c
	}
	total := count("s")

	var list [][]*Node
	var walk func(id string, path []*Node)
	walk = func(id string, path []*Node) {
		path = append(path, byID[id])
		if id == "e" {
			list = append(list, path)
			return
		}
		for _, to := range out[id] {
			if len(list) == max {
				return
			}
			if count(to).Sign() > 0 { // skip branches that never reach End
				walk(to, path[:len(path):len(path)])
			}
		}
	}
	if max > 0 && total.Sign() > 0 {
		walk("s", nil)
	}
	return total, list
}
