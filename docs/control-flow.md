# Control flow: how the graph is built

**Decision (Leigh, 2026-11-09):** `internal/flow` builds the graph by walking the type-checked AST of each workflow function, statement by statement. It does not use `golang.org/x/tools/go/cfg` or SSA.

This replaces the earlier decision to build from `go/cfg` blocks. The rendering rules in [analysis.md](analysis.md) are unchanged. Only the way the builder gets from source to graph changed.

## History

1. Leigh's original brief leaned to the CFG: "Go gives us a real CFG for free. Decide whether to enumerate paths from the CFG rather than hand-walking `if`/`switch`/`select`/`for` AST nodes."
2. The first analysis chose `go/cfg`. The reasons it recorded were against SSA (heavier, loses source positions) and against enumerating 2^n paths. Neither is a reason against a structured AST walk, which also draws from structure and never enumerates.
3. An audit the same day listed `go/cfg` as settled. It also added a note to task `timetell-pas.5` saying go/cfg splits `a && b` into separate blocks. That note was wrong (see reason 2).
4. The building agent asked to walk the AST instead, before starting `timetell-pas.5`. The claims below were checked and Leigh agreed.

No builder code had been written against `go/cfg` when this changed.

## Why the AST walk

**1. Every rendering rule is defined on a statement.** The CFG is made of blocks, so each rule would need mapping from blocks back to statements.

| Rule (analysis.md) | What the builder needs | In the AST |
|---|---|---|
| One decision per `if`/`switch` | the statement and its arms | `IfStmt.Body`/`.Else`, `SwitchStmt` clauses |
| `yes`/`no` edge labels | which arm is which | `Body` is yes, `Else` is no |
| `//timetell:decision Label` | the comment attached to the statement | comment position against `stmt.Pos()` |
| Error returns | the `if err != nil` statement, its arm, and the node before it | `IfStmt.Cond` plus `types.Info`; the node before it is the current frontier |
| Loop node and label | the `for`/`range` header source | `ForStmt.Cond`, `RangeStmt.Key/Value/X` |
| Selector branches, `workflow.Go`, update handlers | the closure body | `FuncLit.Body` |

**2. `go/cfg` does not give what the first analysis assumed.** Checked in x/tools v0.52.0 (the version in `go.mod`):

- `go/cfg/cfg.go:38-39`: "The CFG does not record conditions associated with conditional branch edges, nor the short-circuit semantics of the && and || operators". So it does not split `a && b`, and it does not say which edge out of a condition is the true one. Both would be recovered from `Block.Kind` and `Block.Stmt`, which is the AST again.
- `grep -c FuncLit go/cfg/builder.go` gives 0. Closures are opaque to it. Selector callbacks, `workflow.Go` bodies and handlers would each need a separate `cfg.New` and hand stitching.

**3. Loops are simpler.** The walk is recursion over a finite tree, so it terminates without a cycle guard. The builder creates the loop's back edge itself, so it can mark it when it is created. Path counting skips marked edges and needs no cycle detection.

**4. The fixtures are structured code.** Checked in `testdata/` on 2026-11-09 with grep: no `goto`, labelled `break`/`continue`, `fallthrough` or `defer` in any of the 21 fixtures; 12 put the activity call in the `if` header (`if err := workflow.ExecuteActivity(...).Get(...); err != nil`); 5 use closures.

## What it costs

- We own control-flow handling that `go/cfg`'s tested builder would have given us: `break` and `continue` (labelled and not), `fallthrough`, and unreachable code after `return`. Each gets a test (below).
- `goto` is not supported. It produces a warning, not a wrong diagram.
- Calls that never return (`panic`, `os.Exit`) are not treated as exits, so code after them is drawn as reachable. `go/cfg` could have modelled this through its `mayReturn` callback.

## Design

One recursive function walks a statement list and carries a **frontier**: the set of dangling exits, each a node plus the label its next edge will carry.

```
stmts(list, frontier) -> frontier
```

- **Emit a node:** add an edge from every frontier entry to the new node. The frontier becomes that node.
- **Empty frontier:** stop walking the list. The rest is unreachable, and `go vet` reports that.
- **`return`:** add an edge from every frontier entry to the frame's return target, then empty the frontier. The return target is `End` for the workflow function. For an inlined helper or a closure it is the frontier after the call site.
- **`if`:** walk `Init`, then `Cond`, for Temporal calls first. Then apply the error-return rule or create a decision. For a decision, walk `Body` from `{decision, yes}` and `Else` from `{decision, no}`, and join the two results. If neither arm added a node or left the normal flow, drop the decision (collapse). If both arms draw the same thing (same nodes, edges and exits when each is walked from a fresh entry), walk one arm and draw no decision: analysis.md's "branches with identical effects collapse".
- **Error returns:** for a nil-check on an `error`-typed value, an arm that only returns is skipped and the frontier is unchanged. An arm that does work is walked from the current frontier with its entries labelled `error`. No decision node.
- **`switch`:** one decision, one arm per clause. `fallthrough` adds the clause's frontier to the next clause's entry. A switch with a `default` whose clauses all draw the same thing collapses like `if`; one with `fallthrough`, or with no `default` (the implicit arm draws nothing), keeps its decision.
- **`for`/`range`:** push a frame holding the loop's label, its `break` exits and its `continue` target. Walk the body from `{loop, do}`. The body's frontier and every `continue` get a `loop` edge back to the loop node. The result is `{loop, done}` plus the `break` exits. `for {}` has no `done`.
- **Closures and helpers:** the same function walks `FuncLit.Body` and inlined helper bodies, each with its own frame.

Expressions are scanned in source order for Temporal calls, and the scan does not descend into a `FuncLit`. The construct that consumes the closure (`AddReceive`, `AddFuture`, `workflow.Go`, a handler registration) walks it.

This section is a sketch for the implementer. The contract is the rendering rules in analysis.md and the fixtures' `expected.mmd` files.

## Cases the walker must get right, one test each

| Case | Behaviour |
|---|---|
| Temporal call in `if`/`switch`/`for` `Init` or `Cond` | node emitted before the decision or loop node |
| `if a && b` | one decision |
| Temporal call in the right operand of `&&`/`||` | node drawn, plus a warning: the call is conditional and the diagram shows it as unconditional |
| `fallthrough` | supported |
| `if`/`else` or `switch` (with `default`) whose arms draw the same thing | no decision; the arm is drawn once. Same calls in a different structure keep the decision |
| `break`/`continue` with a label | supported; resolved against the frame stack |
| `break` with no label inside a `switch` or `select` inside a loop | leaves the switch, not the loop |
| `goto` | warning with position; the path ends at a `?` node. Never draw a fall-through edge past it |
| statements after `return` in the same list | not walked |

## Open, not decided

These have no rule yet and no task. `go/cfg` would not have solved them either.

- `defer` that contains Temporal calls. Compensation in a deferred closure is a known saga idiom (from memory of Temporal's Go samples; not checked on 2026-11-09). No fixture uses `defer`. Until there is a rule, it must at least warn, so work is never dropped silently.
- A closure assigned to a variable and called later (`compensate := func() {...}`).
- Native `go` and `select` statements in workflow code. `workflowcheck` flags them as non-deterministic.

## Revisit when

- `goto` or `defer` warnings show up in real workflows often enough to matter.
- A control-flow bug in the walker repeats after its test was added.
- A feature needs dataflow, such as joining parallel activities at `Future.Get`. That points to SSA, not `go/cfg`.
