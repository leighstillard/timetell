// Package render turns a flow.Graph into Mermaid text.
package render

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/leighstillard/timetell/internal/flow"
)

var (
	camel = regexp.MustCompile(`([a-z])([A-Z])`)
	plain = regexp.MustCompile(`^[A-Za-z0-9 _]*$`)
	nonID = regexp.MustCompile(`[^A-Za-z0-9_]`)
	// edgeEsc escapes a quoted edge label; a backtick would start a Markdown string.
	edgeEsc = strings.NewReplacer("#", "#35;", `"`, "#quot;", "`", "#96;")
	shapes  = map[flow.Kind][2]string{
		flow.Start: {"((", "))"}, flow.End: {"((", "))"},
		flow.Activity: {"[", "]"}, flow.LocalActivity: {"[", "]"}, // ponytail: local drawn like activity until :::local styling is asked for
		flow.Decision: {"{", "}"}, flow.Unknown: {"[/", "/]"}, // Loop: drawn by Mermaid as a subgraph
		flow.Signal: {"{{", "}}"}, flow.Timer: {"(", ")"},
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

// Options are the Config fields the renderer reads.
type Options struct {
	Direction string // TD or LR
	// SplitNamesByWords splits camelCase activity and signal names into
	// words. Python splits decisions and signals; here decision labels are
	// directive text or condition source, which splitting would mangle.
	SplitNamesByWords bool
}

// Default is flowchart TD with split activity names.
var Default = Options{Direction: "TD", SplitNamesByWords: true}

func label(n *flow.Node, split bool) string {
	l := n.Label
	if split && (n.Kind == flow.Activity || n.Kind == flow.LocalActivity || n.Kind == flow.Signal) {
		l = camel.ReplaceAllString(l, "$1 $2")
	}
	if n.Kind == flow.Unknown || plain.MatchString(l) {
		return l
	}
	return `"` + strings.ReplaceAll(l, `"`, "#quot;") + `"`
}

// Loop styling ("style C", Leigh 2026-11-10): each loop body is a dashed,
// tinted subgraph titled with the loop label; nested loops alternate tints.
// doneColour marks a loop's 'done' exit: solid and thick, against the dotted
// orange back edges.
const (
	loopColour = "#c77700"
	doneColour = "#2e7d32"
	loopClass  = "classDef loop fill:#fde7c8,stroke:" + loopColour + ",stroke-width:2px,color:#333\n"
)

var tints = [2]string{"#fffaf2", "#fde6c6"}

// Mermaid renders g: flowchart in o.Direction, nodes (s first, the rest sorted by ID,
// e last, each loop's nodes in its subgraph), then edges in builder order,
// then loop styles.
func Mermaid(g *flow.Graph, o Options) []byte {
	var b strings.Builder
	b.WriteString("flowchart " + o.Direction + "\n")
	nodes := slices.Clone(g.Nodes)
	rank := func(n *flow.Node) int { return map[flow.Kind]int{flow.Start: -1, flow.End: 1}[n.Kind] }
	slices.SortStableFunc(nodes, func(a, c *flow.Node) int {
		if r := rank(a) - rank(c); r != 0 {
			return r
		}
		return strings.Compare(id(a.ID), id(c.ID))
	})
	in := map[string][]*flow.Node{}
	taken := map[string]bool{} // Mermaid shares one namespace for nodes and subgraphs
	for _, n := range nodes {
		in[n.In] = append(in[n.In], n)
		taken[id(n.ID)] = true
	}
	var styles []string
	loops := map[string]bool{}
	var emit func(list []*flow.Node, depth int)
	emit = func(list []*flow.Node, depth int) {
		pad := strings.Repeat("  ", depth)
		for _, n := range list {
			if n.Kind != flow.Loop {
				sh := shapes[n.Kind]
				b.WriteString(pad + id(n.ID) + sh[0] + label(n, o.SplitNamesByWords) + sh[1] + "\n")
				continue
			}
			loops[n.ID] = true
			sg := "L" + strings.TrimPrefix(id(n.ID), "l")
			for taken[sg] { // an activity may be called L0
				sg += "_"
			}
			taken[sg] = true
			b.WriteString(pad + "subgraph " + sg + ` ["` + strings.ReplaceAll(n.Label, `"`, "#quot;") + "\"]\n")
			b.WriteString(pad + "  " + id(n.ID) + `{{"↻"}}:::loop` + "\n")
			emit(in[n.ID], depth+1)
			b.WriteString(pad + "end\n")
			styles = append(styles, "style "+sg+" fill:"+tints[depth%2]+",stroke:"+loopColour+",stroke-dasharray:4,color:#333\n")
		}
	}
	emit(in[""], 0)
	var back, done, brk []string
	for i, e := range g.Edges {
		switch { // one style per edge; an inner loop's done or break that is also an outer back edge keeps its exit style
		case loops[e.From] && e.Label == "done":
			done = append(done, strconv.Itoa(i))
		case e.Break:
			brk = append(brk, strconv.Itoa(i))
		case e.Back:
			back = append(back, strconv.Itoa(i))
		}
		l := e.Label
		// Style C drops the walker's do/loop labels; only loop entries carry a
		// bare "do" (a case label keeps its quotes).
		if e.Back && l == "loop" || l == "do" {
			l = ""
		}
		arrow := " --> "
		switch {
		case l != "" && !plain.MatchString(l): // e.g. a case label holding "-->" or ".-"
			arrow = " -->|"
			if e.Back {
				arrow = " -.->|"
			}
			arrow += `"` + edgeEsc.Replace(l) + `"| `
		case e.Back && l != "":
			arrow = " -. " + l + " .-> "
		case e.Back:
			arrow = " -.-> "
		case l != "":
			arrow = " -- " + l + " --> "
		}
		b.WriteString(id(e.From) + arrow + id(e.To) + "\n")
	}
	if len(styles) > 0 {
		b.WriteString(loopClass)
	}
	if len(back) > 0 {
		b.WriteString("linkStyle " + strings.Join(back, ",") + " stroke:" + loopColour + ",stroke-width:2px\n")
	}
	if len(done) > 0 {
		b.WriteString("linkStyle " + strings.Join(done, ",") + " stroke:" + doneColour + ",stroke-width:3px\n")
	}
	if len(brk) > 0 { // break out of a loop: as thick as done, default colour
		b.WriteString("linkStyle " + strings.Join(brk, ",") + " stroke:#333,stroke-width:3px\n")
	}
	for _, s := range styles {
		b.WriteString(s)
	}
	return []byte(b.String())
}
