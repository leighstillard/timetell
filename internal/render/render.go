// Package render turns a flow.Graph into Mermaid text.
package render

import (
	"regexp"
	"slices"
	"strings"

	"github.com/leighstillard/timetell/internal/flow"
)

var (
	camel  = regexp.MustCompile(`([a-z])([A-Z])`)
	plain  = regexp.MustCompile(`^[A-Za-z0-9 _]*$`)
	nonID  = regexp.MustCompile(`[^A-Za-z0-9_]`)
	shapes = map[flow.Kind][2]string{
		flow.Start: {"((", "))"}, flow.End: {"((", "))"},
		flow.Activity: {"[", "]"}, flow.LocalActivity: {"[", "]"}, // ponytail: local drawn like activity until :::local styling is asked for
		flow.Decision: {"{", "}"}, flow.Loop: {"([", "])"}, flow.Unknown: {"[/", "/]"},
	}
)

// id makes a node ID safe for Mermaid: string activity names may hold any
// character, and a bare "end" closes a subgraph.
func id(s string) string {
	s = nonID.ReplaceAllString(s, "_")
	if strings.EqualFold(s, "end") {
		s += "_"
	}
	return s
}

func label(n *flow.Node) string {
	l := n.Label
	if n.Kind == flow.Activity || n.Kind == flow.LocalActivity {
		l = camel.ReplaceAllString(l, "$1 $2")
	}
	if n.Kind == flow.Unknown || plain.MatchString(l) {
		return l
	}
	return `"` + strings.ReplaceAll(l, `"`, "#quot;") + `"`
}

// Mermaid renders g: flowchart LR, nodes (s first, the rest sorted by ID,
// e last), then edges in builder order.
func Mermaid(g *flow.Graph) []byte {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	nodes := slices.Clone(g.Nodes)
	rank := func(n *flow.Node) int { return map[flow.Kind]int{flow.Start: -1, flow.End: 1}[n.Kind] }
	slices.SortStableFunc(nodes, func(a, c *flow.Node) int {
		if r := rank(a) - rank(c); r != 0 {
			return r
		}
		return strings.Compare(id(a.ID), id(c.ID))
	})
	for _, n := range nodes {
		sh := shapes[n.Kind]
		b.WriteString(id(n.ID) + sh[0] + label(n) + sh[1] + "\n")
	}
	for _, e := range g.Edges {
		arrow := " --> "
		if e.Label != "" {
			arrow = " -- " + e.Label + " --> "
		}
		b.WriteString(id(e.From) + arrow + id(e.To) + "\n")
	}
	return []byte(b.String())
}
