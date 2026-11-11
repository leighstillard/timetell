// Package flow builds a workflow's Graph by walking the type-checked AST of
// its function body, statement by statement (docs/control-flow.md).
package flow

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"time"

	"github.com/leighstillard/timetell/internal/discover"
	"github.com/leighstillard/timetell/internal/load"
)

// Kind is what a Node draws.
type Kind int

const (
	Start Kind = iota
	End
	Activity
	LocalActivity
	Decision
	Loop
	Unknown // unresolved name or unsupported construct: [/?/]
	Signal  // a signal wait: Receive, Await, AwaitWithTimeout
	Timer   // Sleep, NewTimer
)

// Node is one vertex. Label is raw: the renderer splits and quotes it. In is
// the ID of the innermost loop around the node ("" at top level); a Loop
// node's In is the loop around it, not itself.
type Node struct {
	ID    string
	Kind  Kind
	Label string
	Pos   token.Position
	In    string
}

// Edge is a directed edge. Back marks a loop's back edge, which path
// counting skips. Break marks an edge that leaves a loop through break.
type Edge struct {
	From, To, Label string
	Back, Break     bool
}

// Graph is one workflow. Nodes and Edges are in creation order.
type Graph struct {
	Workflow string
	Nodes    []*Node
	Edges    []Edge
	Warnings []load.Warning
}

// exit is a dangling edge: from a node, with the label its next edge carries;
// brk if it left a loop through break.
type exit struct {
	from, label string
	brk         bool
}

// frame is a breakable statement (loop, switch) on the walk stack.
type frame struct {
	label     string
	loop      bool
	brk, cont []exit
}

// Labels are the words the walker writes into the graph: the Start and End
// node labels, the edges out of an if decision and out of an
// AwaitWithTimeout whose ok an if tests.
type Labels struct{ Start, End, Yes, No, Signaled, Timeout string }

// DefaultLabels are Python's defaults.
var DefaultLabels = Labels{Start: "Start", End: "End", Yes: "yes", No: "no", Signaled: "Signaled", Timeout: "Timeout"}

type walker struct {
	g      *Graph
	words  Labels
	info   *types.Info
	fset   *token.FileSet
	dirs   map[int]string // line -> //timetell:decision label
	ids    map[string]int
	seen   map[Edge]bool
	fr     []exit // the frontier
	frames []*frame
	loops  []string                // IDs of the drawn loops around the current statement
	label  string                  // label of the enclosing LabeledStmt, for the next frame
	chans  map[types.Object]string // signal channel variable -> signal name
	waits  map[types.Object]string // AwaitWithTimeout ok variable -> wait node ID
}

const directive = "//timetell:decision "

// Build walks fn's body into a Graph.
func Build(fn *discover.Func, l Labels) *Graph {
	w := &walker{
		g:     &Graph{Workflow: fn.Obj.Name()},
		words: l,
		info:  fn.Pkg.TypesInfo,
		fset:  fn.Pkg.Fset,
		dirs:  map[int]string{},
		ids:   map[string]int{},
		seen:  map[Edge]bool{},
		chans: map[types.Object]string{},
		waits: map[types.Object]string{},
	}
	for _, f := range fn.Pkg.Syntax {
		if f.Pos() <= fn.Decl.Pos() && fn.Decl.Pos() < f.End() {
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					if s, ok := strings.CutPrefix(c.Text, directive); ok {
						w.dirs[w.fset.Position(c.Pos()).Line] = strings.TrimSpace(s)
					}
				}
			}
		}
	}
	w.g.Nodes = append(w.g.Nodes, &Node{ID: "s", Kind: Start, Label: w.words.Start})
	w.fr = []exit{{from: "s"}}
	w.stmts(fn.Decl.Body.List)
	w.connect(w.fr, "e", "", false)
	w.g.Nodes = append(w.g.Nodes, &Node{ID: "e", Kind: End, Label: w.words.End})
	return w.g
}

func (w *walker) warn(p token.Pos, msg string) {
	w.g.Warnings = append(w.g.Warnings, load.Warning{Pos: w.fset.Position(p), Msg: msg})
}

func (w *walker) connect(fr []exit, to, label string, back bool) {
	for _, x := range fr {
		e := Edge{From: x.from, To: to, Label: x.label, Back: back, Break: x.brk}
		if label != "" {
			e.Label = label
		}
		if !w.seen[e] {
			w.seen[e] = true
			w.g.Edges = append(w.g.Edges, e)
		}
	}
}

// node creates a node, connects the frontier to it and makes it the
// frontier. Activities are numbered per name (Name, Name_2); the rest per
// kind prefix in source order (d0, l0, q0, w0, t0).
func (w *walker) node(k Kind, label string, p token.Pos) string {
	var id string
	switch k {
	case Activity, LocalActivity:
		w.ids[label]++
		id = label
		if n := w.ids[label]; n > 1 {
			id += "_" + strconv.Itoa(n)
		}
	default:
		pre := map[Kind]string{Decision: "d", Loop: "l", Unknown: "q", Signal: "w", Timer: "t"}[k]
		id = pre + strconv.Itoa(w.ids[pre])
		w.ids[pre]++
	}
	n := &Node{ID: id, Kind: k, Label: label, Pos: w.fset.Position(p)}
	if len(w.loops) > 0 {
		n.In = w.loops[len(w.loops)-1]
	}
	w.g.Nodes = append(w.g.Nodes, n)
	w.connect(w.fr, id, "", false)
	w.fr = []exit{{from: id}}
	return id
}

// labelFor is the //timetell:decision label on the line above p, else def.
func (w *walker) labelFor(p token.Pos, def string) string {
	if s, ok := w.dirs[w.fset.Position(p).Line-1]; ok {
		return s
	}
	// ponytail: fixed 40-rune cut; make it a Config field if labels need it.
	if r := []rune(def); len(r) > 40 {
		return string(r[:39]) + "…"
	}
	return def
}

func (w *walker) stmts(list []ast.Stmt) {
	for _, s := range list {
		if w.fr == nil { // unreachable; go vet reports it
			return
		}
		w.stmt(s)
	}
}

func (w *walker) stmt(s ast.Stmt) {
	lbl := w.label
	w.label = ""
	switch s := s.(type) {
	case *ast.BlockStmt:
		w.stmts(s.List)
	case *ast.LabeledStmt:
		w.label = s.Label.Name
		w.stmt(s.Stmt)
	case *ast.ReturnStmt:
		label := ""
		for _, r := range s.Results {
			if c, ok := ast.Unparen(r).(*ast.CallExpr); ok && load.Classify(w.info, c) == load.NewContinueAsNewError {
				label = "continue-as-new"
			}
		}
		w.scan(s)
		w.connect(w.fr, "e", label, false)
		w.fr = nil
	case *ast.IfStmt:
		w.ifStmt(s)
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		w.switchStmt(s, lbl)
	case *ast.ForStmt, *ast.RangeStmt:
		w.loop(s, lbl)
	case *ast.BranchStmt:
		w.branch(s)
	case *ast.DeferStmt:
		if w.temporal(s, true) {
			w.warn(s.Pos(), "defer with Temporal calls is not drawn (timetell-pas.25)")
		}
	case *ast.GoStmt:
		if w.temporal(s, true) {
			w.warn(s.Pos(), "native go statement with Temporal calls is not drawn (timetell-pas.27)")
		}
	case *ast.SelectStmt:
		if w.temporal(s, true) {
			w.warn(s.Pos(), "native select statement with Temporal calls is not drawn (timetell-pas.27)")
		}
	case *ast.AssignStmt:
		w.assign(s)
	default: // ExprStmt, DeclStmt, IncDecStmt, SendStmt, EmptyStmt
		w.scan(s)
	}
}

// assign scans s and records the signal channels and AwaitWithTimeout
// results it assigns to variables.
// ponytail: var declarations are not tracked; add ValueSpec if fixtures use them.
func (w *walker) assign(s *ast.AssignStmt) {
	w.scan(s)
	for _, l := range s.Lhs { // a reassigned variable no longer names its old channel or wait
		if id, ok := l.(*ast.Ident); ok {
			delete(w.chans, w.info.ObjectOf(id))
			delete(w.waits, w.info.ObjectOf(id))
		}
	}
	for i, r := range s.Rhs {
		c, ok := ast.Unparen(r).(*ast.CallExpr)
		id, _ := s.Lhs[min(i, len(s.Lhs)-1)].(*ast.Ident)
		if !ok || id == nil || w.info.ObjectOf(id) == nil {
			continue
		}
		switch load.Classify(w.info, c) {
		case load.GetSignalChannel:
			if name, ok := load.ConstString(w.info, c.Args[1]); ok {
				w.chans[w.info.ObjectOf(id)] = name
			}
		case load.AwaitWithTimeout: // ok, err := ...: the last node is the wait
			w.waits[w.info.ObjectOf(id)] = w.g.Nodes[len(w.g.Nodes)-1].ID
		}
	}
}

func (w *walker) branch(s *ast.BranchStmt) {
	switch s.Tok {
	case token.GOTO:
		w.warn(s.Pos(), "goto is not supported; the path ends here")
		w.node(Unknown, "?", s.Pos())
		w.fr = nil
	case token.BREAK, token.CONTINUE:
		for i := len(w.frames) - 1; i >= 0; i-- {
			f := w.frames[i]
			if s.Label != nil && f.label != s.Label.Name || s.Label == nil && s.Tok == token.CONTINUE && !f.loop {
				continue
			}
			if s.Tok == token.BREAK {
				for _, x := range w.fr {
					x.brk = x.brk || f.loop
					f.brk = append(f.brk, x)
				}
			} else {
				f.cont = append(f.cont, w.fr...)
			}
			break
		}
		w.fr = nil
	}
	// fallthrough is handled by switchStmt.
}

func (w *walker) push(f *frame) func() {
	w.frames = append(w.frames, f)
	return func() { w.frames = w.frames[:len(w.frames)-1] }
}

// walkFlat walks stmts that draw nothing (only for their warnings) and
// leaves the frontier as it was.
func (w *walker) walkFlat(list []ast.Stmt) {
	start := w.fr
	w.stmts(list)
	w.fr = start
}

func (w *walker) ifStmt(s *ast.IfStmt) {
	if s.Init != nil {
		w.stmt(s.Init)
	}
	w.scan(s.Cond)
	if w.isErrCheck(s.Cond) {
		hasCalls := w.temporal(s.Body, false)
		if !hasCalls && len(s.Body.List) > 0 {
			if _, ok := s.Body.List[len(s.Body.List)-1].(*ast.ReturnStmt); ok {
				if s.Else != nil {
					w.stmt(s.Else)
				}
				return // plain error return: not drawn
			}
		}
		if hasCalls { // compensation: an 'error' edge, no diamond
			start := w.fr
			w.fr = nil
			for _, x := range start {
				w.fr = append(w.fr, exit{x.from, "error", x.brk})
			}
			w.stmts(s.Body.List)
			errFr := w.fr
			w.fr = start
			if s.Else != nil {
				w.stmt(s.Else)
			}
			w.fr = append(errFr, w.fr...)
			return
		}
	}
	if !w.effect(s.Body, nil) && (s.Else == nil || !w.effect(s.Else, nil)) {
		w.walkFlat(s.Body.List)
		if s.Else != nil {
			w.walkFlat([]ast.Stmt{s.Else})
		}
		return
	}
	if s.Else != nil && w.sig(s.Body.List) == w.sig([]ast.Stmt{s.Else}) {
		w.stmts(s.Body.List) // identical arms: no decision
		return
	}
	d, yes, no := w.waitCond(s.Cond)
	if d == "" {
		d, yes, no = w.node(Decision, w.labelFor(s.Pos(), types.ExprString(s.Cond)), s.Pos()), w.words.Yes, w.words.No
	}
	w.fr = []exit{{d, yes, false}}
	w.stmts(s.Body.List)
	yesFr := w.fr
	w.fr = []exit{{d, no, false}}
	if s.Else != nil {
		w.stmt(s.Else)
	}
	w.fr = append(yesFr, w.fr...)
}

// waitCond returns the wait node whose ok result cond (ok or !ok) tests and
// the labels of its two arms, when the frontier is that wait alone; else "".
func (w *walker) waitCond(cond ast.Expr) (string, string, string) {
	cond = ast.Unparen(cond)
	yes, no := w.words.Signaled, w.words.Timeout
	if u, ok := cond.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		cond, yes, no = ast.Unparen(u.X), no, yes
	}
	id, _ := cond.(*ast.Ident)
	if id == nil {
		return "", "", ""
	}
	if d := w.waits[w.info.ObjectOf(id)]; d != "" && len(w.fr) == 1 && w.fr[0] == (exit{from: d}) {
		return d, yes, no
	}
	return "", "", ""
}

func (w *walker) switchStmt(s ast.Stmt, lbl string) {
	var body *ast.BlockStmt
	tag := "switch"
	switch s := s.(type) {
	case *ast.SwitchStmt:
		if s.Init != nil {
			w.stmt(s.Init)
		}
		if s.Tag != nil {
			w.scan(s.Tag)
			tag = types.ExprString(s.Tag)
		}
		body = s.Body
	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			w.stmt(s.Init)
		}
		w.stmt(s.Assign)
		ast.Inspect(s.Assign, func(n ast.Node) bool {
			if ta, ok := n.(*ast.TypeAssertExpr); ok {
				tag = types.ExprString(ta)
			}
			return true
		})
		body = s.Body
	}
	f := &frame{label: lbl}
	defer w.push(f)()
	if !w.effect(body, s) {
		for _, c := range body.List {
			w.walkFlat(c.(*ast.CaseClause).Body)
		}
		return
	}
	if same := w.sameClauses(body); same != nil {
		w.stmts(same) // identical arms: no decision
		w.fr = append(w.fr, f.brk...)
		return
	}
	d := w.node(Decision, w.labelFor(s.Pos(), tag), s.Pos())
	var out, carry []exit
	other := []exit{{d, "other", false}}
	for _, c := range body.List {
		cc := c.(*ast.CaseClause)
		label := "default"
		if cc.List == nil {
			other = nil
		} else {
			var cases []string
			for _, e := range cc.List {
				cases = append(cases, types.ExprString(e))
			}
			label = strings.Join(cases, ", ")
		}
		w.fr = append([]exit{{d, label, false}}, carry...)
		w.stmts(cc.Body)
		carry = nil
		if n := len(cc.Body); n > 0 {
			if b, ok := cc.Body[n-1].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
				carry = w.fr
				continue
			}
		}
		out = append(out, w.fr...)
	}
	w.fr = append(append(out, f.brk...), other...)
}

// sameClauses returns the shared body when every clause of a switch with a
// default draws the same thing, else nil.
// ponytail: a switch with fallthrough never collapses.
func (w *walker) sameClauses(body *ast.BlockStmt) []ast.Stmt {
	var first []ast.Stmt
	dflt := false
	for i, c := range body.List {
		cc := c.(*ast.CaseClause)
		if n := len(cc.Body); n > 0 {
			if b, ok := cc.Body[n-1].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
				return nil
			}
		}
		dflt = dflt || cc.List == nil
		if i == 0 {
			first = cc.Body
		} else if w.sig(cc.Body) != w.sig(first) {
			return nil
		}
	}
	if !dflt {
		return nil
	}
	return first
}

// sig fingerprints what list draws: it walks list in a scratch walker from a
// fresh exit, so equal arms give equal strings whatever their positions.
// The scratch walk's warnings are discarded; the real walk repeats them.
// ponytail: re-walks arms, so nested if/else costs 3^depth walks; memoise
// per statement if deep nesting shows up.
func (w *walker) sig(list []ast.Stmt) string {
	sub := &walker{g: &Graph{}, words: w.words, info: w.info, fset: w.fset, dirs: w.dirs,
		ids: map[string]int{}, seen: map[Edge]bool{}, fr: []exit{{from: "x"}},
		chans: w.chans, waits: map[types.Object]string{}}
	for _, f := range w.frames {
		sub.frames = append(sub.frames, &frame{label: f.label, loop: f.loop})
	}
	sub.stmts(list)
	var b strings.Builder
	for _, n := range sub.g.Nodes {
		fmt.Fprintf(&b, "%d %q;", n.Kind, n.Label)
	}
	fmt.Fprint(&b, sub.g.Edges, sub.fr)
	for _, f := range sub.frames {
		fmt.Fprint(&b, f.brk, f.cont)
	}
	return b.String()
}

func (w *walker) loop(s ast.Stmt, lbl string) {
	var body *ast.BlockStmt
	var post ast.Stmt
	label, forever := "forever", false
	switch s := s.(type) {
	case *ast.ForStmt:
		if s.Init != nil {
			w.stmt(s.Init)
		}
		if s.Cond != nil {
			w.scan(s.Cond)
			label = "while " + types.ExprString(s.Cond)
		} else {
			forever = true
		}
		body, post = s.Body, s.Post
	case *ast.RangeStmt:
		w.scan(s.X)
		var vars []string
		for _, v := range []ast.Expr{s.Key, s.Value} {
			if id, ok := v.(*ast.Ident); ok && id.Name != "_" {
				vars = append(vars, id.Name)
			}
		}
		label = "each " + strings.Join(vars, ", ") + " in " + types.ExprString(s.X)
		body = s.Body
	}
	f := &frame{label: lbl, loop: true}
	defer w.push(f)()
	if !w.effect(body, s) {
		w.walkFlat(body.List)
		return
	}
	l := w.node(Loop, w.labelFor(s.Pos(), label), s.Pos())
	w.fr = []exit{{l, "do", false}}
	w.loops = append(w.loops, l)
	w.stmts(body.List)
	w.fr = append(w.fr, f.cont...)
	if post != nil {
		w.stmt(post)
	}
	w.loops = w.loops[:len(w.loops)-1]
	for _, x := range w.fr {
		if x.label == "" {
			x.label = "loop"
		}
		w.connect([]exit{x}, l, "", true)
	}
	w.fr = nil
	if !forever {
		w.fr = []exit{{l, "done", false}}
	}
	w.fr = append(w.fr, f.brk...)
}

// scan emits nodes for the Temporal calls in n, in source order. It does not
// enter closures: the construct that consumes a closure walks it.
func (w *walker) scan(n ast.Node) {
	ast.Inspect(n, func(x ast.Node) bool {
		switch x := x.(type) {
		case *ast.FuncLit:
			if w.temporal(x.Body, false) {
				w.warn(x.Pos(), "closure with Temporal calls is not drawn (timetell-pas.26)")
			}
			return false
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				w.scan(x.X)
				if w.temporal(x.Y, false) {
					w.warn(x.Y.Pos(), "Temporal call in the right operand of "+x.Op.String()+" runs conditionally but is drawn as unconditional")
				}
				w.scan(x.Y)
				return false
			}
		case *ast.CallExpr:
			w.call(x)
		}
		return true
	})
}

func (w *walker) call(c *ast.CallExpr) {
	k := load.Classify(w.info, c)
	switch k {
	case load.Receive:
		name := ""
		if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
			name = w.signal(sel.X)
		}
		if name == "" {
			w.warn(c.Pos(), "dynamic signal name")
			w.node(Unknown, "?", c.Pos())
			return
		}
		w.node(Signal, name, c.Pos())
		return
	case load.Await, load.AwaitWithTimeout: // label: directive, else the signal the condition reads
		def := cmp.Or(w.signal(c.Args[len(c.Args)-1]), "wait")
		w.node(Signal, w.labelFor(c.Pos(), def), c.Pos())
		return
	case load.Sleep, load.NewTimer:
		label := "Timer"
		if tv := w.info.Types[c.Args[1]]; tv.Value != nil {
			if ns, ok := constant.Int64Val(tv.Value); ok {
				label += " " + duration(time.Duration(ns))
			}
		}
		w.node(Timer, label, c.Pos())
		return
	}
	if k != load.ExecuteActivity && k != load.ExecuteLocalActivity || len(c.Args) < 2 {
		return
	}
	t, ok := load.ResolveTarget(w.info, c.Args[1])
	if !ok {
		w.warn(c.Args[1].Pos(), "dynamic activity name")
		w.node(Unknown, "?", c.Pos())
		return
	}
	kind := Activity
	if k == load.ExecuteLocalActivity {
		kind = LocalActivity
	}
	w.node(kind, t.Name, c.Pos())
}

// signal is the constant name of the first signal channel n reads, else "".
func (w *walker) signal(n ast.Node) (name string) {
	ast.Inspect(n, func(x ast.Node) bool {
		if name != "" { // Inspect still visits siblings; don't let them clear it
			return false
		}
		switch x := x.(type) {
		case *ast.Ident:
			name = w.chans[w.info.ObjectOf(x)]
		case *ast.CallExpr:
			if load.Classify(w.info, x) == load.GetSignalChannel && len(x.Args) > 1 {
				name, _ = load.ConstString(w.info, x.Args[1])
			}
		}
		return name == ""
	})
	return name
}

// duration is d without zero trailing units: 24h, 1h30m, 1m30s.
func duration(d time.Duration) string {
	s := d.String()
	if t, ok := strings.CutSuffix(s, "m0s"); ok {
		s = t + "m"
	}
	if t, ok := strings.CutSuffix(s, "h0m"); ok {
		s = t + "h"
	}
	return s
}

// silent are SDK calls that never draw anything by themselves.
var silent = map[load.Kind]bool{load.None: true, load.WithActivityOptions: true, load.GetSignalChannel: true,
	load.NewSelector: true, load.RegisterWorkflow: true, load.RegisterActivity: true}

// temporal reports whether n contains an SDK call that draws something;
// deep also looks inside closures.
func (w *walker) temporal(n ast.Node, deep bool) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if c, ok := x.(*ast.CallExpr); ok && !silent[load.Classify(w.info, c)] {
			found = true
		}
		_, lit := x.(*ast.FuncLit)
		return !found && (deep || !lit)
	})
	return found
}

// effect reports whether n draws a node or leaves the normal flow: a Temporal
// call, a return, a goto, or a break/continue whose target is outside n.
// self is the statement n belongs to (its own breaks don't count).
func (w *walker) effect(n ast.Node, self ast.Node) bool {
	var stack []ast.Node
	if self != nil {
		stack = append(stack, self)
	}
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if found { // Inspect still visits siblings; don't let them clear it
			return false
		}
		switch x := x.(type) {
		case nil:
			stack = stack[:len(stack)-1]
			return false
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = true
		case *ast.CallExpr:
			found = !silent[load.Classify(w.info, x)]
		case *ast.BranchStmt:
			found = escapes(x, stack)
		}
		if found {
			return false
		}
		stack = append(stack, x)
		return true
	})
	return found
}

func escapes(b *ast.BranchStmt, stack []ast.Node) bool {
	switch b.Tok {
	case token.GOTO:
		return true
	case token.FALLTHROUGH:
		return false
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if b.Label != nil {
			if l, ok := stack[i].(*ast.LabeledStmt); ok && l.Label.Name == b.Label.Name {
				return false
			}
			continue
		}
		switch stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return false
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if b.Tok == token.BREAK {
				return false
			}
		}
	}
	return true
}

var errorType = types.Universe.Lookup("error").Type()

// isErrCheck reports whether cond is `x != nil` (either way round) with x
// of type error.
// ponytail: only != nil with the error arm as Body; `err == nil` with an
// else arm stays a decision.
func (w *walker) isErrCheck(cond ast.Expr) bool {
	b, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	isNil := func(e ast.Expr) bool { return w.info.Types[e].IsNil() }
	isErr := func(e ast.Expr) bool { return types.Identical(w.info.TypeOf(e), errorType) }
	return isNil(b.Y) && isErr(b.X) || isNil(b.X) && isErr(b.Y)
}
