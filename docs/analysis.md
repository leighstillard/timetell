# timetell: analysis

Goal: draw every shape a Temporal Go workflow can take (activities, decisions, signals, timers, child workflows, coroutines, loops) as Mermaid, without running it, so an agent-written workflow can be reviewed at a glance.

Reference: `temporalio-graphs` 0.4.0 (PyPI, MIT, github.com/LucaDeLeo/temporalio-graphs), itself a port of the .NET `Temporalio.Graphs`. Read: the wheel source plus the repo's `examples/*/expected_output*.md`.

## How the Python tool actually works

It collects activities, `to_decision` calls, `wait_condition` calls and child calls from the AST, **sorts them all by source line**, then emits one path per 2^n combination of decision/signal outcomes and unions the edges into a flowchart (`max_paths=1024` hard cap). Consequences, checked against its own code and golden files:

- An activity under an unmarked `if`, `for` or `while` is drawn on every path, in line order. Plain branches and loops aren't refused; they're just flattened silently.
- Path count is 2^n whatever the structure, so sequential decisions explode even when they're independent.
- Its own `parent_child_workflow` golden files show child mode still incomplete: decisions render as `[rect]`, the child as `[Name]` not `[[Name]]`, and the four "inline" paths are identical.

## Port as-is

- **Shapes:** `s((Start))`, `e((End))`, activity `Name[Name]`, decision `d0{Label}` with `yes`/`no` edges, signal wait `Name{{Label}}` with `Signaled`/`Timeout` edges, child workflow `[[Name]]`, external signal send `[/Signal 'name'\]`, signal handler `{{name}}` (styled `fill:#e6f3ff,stroke:#0066cc`), unresolved target `[/?/]`, cross-workflow edge `-.name.->`, `flowchart LR`. Node IDs as Python's: `s`, `e`, `d<n>`, activity name. Nodes are emitted before edges, start first, end last, the rest sorted.
- **Config** (`Config` struct, CLI flags of the same names): `StartLabel`/`EndLabel`, `DecisionTrue/FalseLabel`, `SignalSuccess/TimeoutLabel`, `SplitNamesByWords`, `MaxDecisionPoints` (warning), `ChildWorkflowExpansion` = `reference|inline|subgraph` (the handoff's "collapsed/inline/separate" are these three; Python's names are kept so output is comparable), `ResolveSignalTargets`, `SignalVisualizationMode` = `subgraph|unified`, `SignalMaxDiscoveryDepth` (default 10), `WarnUnresolvedSignals`, `OutputFormat` = `mermaid|paths|full`. Dropped: `is_building_graph`, `exit_after_building_graph` (runtime-mode leftovers from .NET), `signal_target_search_paths` (package patterns replace it), and `max_paths` as a hard error (see the CFG section).
- **Reporting:** a validation report of warnings (unresolved name, dynamic activity name, too many decisions, unreachable activity), each with file:line, appended in `full` format.

## Where Go does better: decisions

| Topic | Decision | Why |
|---|---|---|
| Loading | `go/packages` with full `types.Info` | `ExecuteActivity(ctx, a.Charge)` resolves to a `*types.Func` whatever the import alias, method value or wrapper. The SDK is matched by package path `go.temporal.io/sdk/workflow` plus func name, so timetell itself never imports the SDK. |
| Names | constant-fold through `types.Info.Types[e].Value` | Signal names, version change IDs and string activity names can be `const`s, not only literals. A truly dynamic value becomes `?` plus a warning, as in Python. |
| Control flow | `golang.org/x/tools/go/cfg` per function body (not SSA) | A CFG gives real branch structure, so we build the graph straight from blocks and never enumerate paths. Blocks with no Temporal effect collapse. SSA is heavier and loses source positions we need for labels; `go/cfg` is enough. |
| Paths | count by DP over the DAG; enumerate only for `paths` output | Removes the 2^n blow-up. `MaxPaths` caps only the printed list. |
| Loops | draw the back edge, labelled `loop` | Mermaid flowcharts allow cycles, so the truth costs nothing. Path list: 0 or 1 iteration. Removes a documented Python limitation. |
| Unmarked branches | any `if`/`switch` whose arms differ in Temporal effects becomes a decision labelled with its condition source (truncated) | The diagram never presents conditional work as unconditional. Branches with identical effects collapse. |
| Decision labels | `//timetell:decision Label` comment directive on the `if`/`case` | Zero runtime cost and no import. A marker func (`graph.Decision`) costs a dependency in workflow code. Treating every activity-derived `if` as a decision through type info is noisy, and the rule above already shows them. Directive also accepted on `AwaitWithTimeout` and `Select` for labels. |
| Discovery | roots = funcs passed to `worker.RegisterWorkflow[WithOptions]` in the loaded `./...` patterns; fallback = exported funcs whose first param is `workflow.Context` | Uses the module graph, not search paths. Activities registered through `RegisterActivity` (funcs, or every exported method of a struct) feed the lints. |
| Helpers | inline calls to same-module funcs that take a `workflow.Context`, depth-bounded, recursion guarded | Agents split workflows into helpers, and Python can't follow them. |

## Go-SDK constructs

| Construct | Rendering |
|---|---|
| `ExecuteActivity` / `ExecuteLocalActivity` | `[Name]`; local gets `:::local` (dashed). Placed at the call site, not at `Future.Get`. |
| `ExecuteChildWorkflow` | `[[Name]]` and the three expansion modes |
| `GetSignalChannel(ctx, n).Receive` | `{{n}}`, single out-edge |
| `AwaitWithTimeout` whose `ok` feeds an `if` | signal node with `Signaled`/`Timeout` edges (the `wait_condition` analogue); `Await` = one edge |
| `NewSelector` + `AddReceive`/`AddFuture`/`AddDefault` + `Select` | the selector variable is tracked. `Select` fans out to one edge per `Add*` in registration order, labelled with the signal name, `timer`, the activity name or `default`. Each closure body is inlined as that branch. Signal + timer is the commonest shape and comes out as Signaled/Timeout-like edges. |
| `NewTimer` / `Sleep` | `(Timer 24h)` rounded, with the duration if it's constant |
| `workflow.Go` | a dotted `-. go .->` edge into the coroutine body, drawn as its own subgraph. The main path continues; there's no false join. |
| `GetVersion(ctx, id, min, max)` + comparison | decision `{Version id}`, edges labelled with the compared versions |
| `SetQueryHandler` / `SetUpdateHandler` | a `Handlers` subgraph. Update handler bodies are graphed like workflow bodies, because they run activities. |
| `SignalExternalWorkflow`, `ChildWorkflowFuture.SignalChildWorkflow` | trapezoid send, resolved by name to `GetSignalChannel` receivers in other workflows |
| `NewContinueAsNewError` return | edge to `e` labelled `continue-as-new` |
| `NewFuture`, `SideEffect`, Nexus | v1: not drawn |

## Distribution and output

CLI first: `timetell graph|lint|diff [-flags] ./...` is a single static binary, CGO off, exiting non-zero on findings, so it already runs in CI. The library (`timetell` package: `Load`, `Build`, `Render`) is what the CLI calls. A `go/analysis` analyzer is a thin later layer, exposing per-package lints to `go vet -vettool` and gopls. Cross-package lints (registration) stay CLI-only, so we don't need analysis facts. Output: Mermaid (default) and `-format json`, a serialised `Graph` with stable node keys, which is what diff mode compares.

## Review features (proposed, not built)

- Lints: activity invoked but not registered; signal received but never sent by any analysed workflow (and the reverse); `GetVersion` with no branch for `DefaultVersion`/`min`; unreachable activity (CFG-dead block); `ActivityOptions` with neither `StartToCloseTimeout` nor `ScheduleToCloseTimeout` on the ctx that reaches `ExecuteActivity`.
- **Non-determinism: don't rebuild it.** Temporal's `go.temporal.io/sdk/contrib/tools/workflowcheck` already flags `time.Now`, `rand`, map range and goroutines in workflow code. Document running it next to timetell. Importing it would break the "stdlib + x/tools only" rule.
- Diff: `timetell diff A B` checks out each rev with `git worktree`, builds both graphs, and renders the union with `classDef added` (green), `removed` (red, dashed) and `changed` (amber, label or edge set differs). Nodes match on stable keys (kind + name + nth occurrence).

## Architecture

```
cmd/timetell        CLI (flag pkg): graph | lint | diff
timetell            public API: Config, Load(patterns), Build(Config) → []*Graph, Lint, Diff
internal/load       go/packages loading; SDK call classification by types.Object (pkg path + name)
internal/discover   registered workflows/activities, roots, signal handler index
internal/flow       go/cfg → Graph: collapse, decisions, selectors, coroutines, loops, helper inlining
internal/render     Mermaid + JSON; deterministic ordering
internal/lint       review findings (Finding{Pos, Rule, Msg})
analyzer            go/analysis wrapper (later)
testdata/<fixture>  separate go.mod requiring go.temporal.io/sdk; workflow.go + expected.mmd
```

Key types: `Graph{Workflow, Nodes []Node, Edges []Edge, Subgraphs, Warnings}`, `Node{ID, Key, Kind, Label, Pos}` with `Kind` = Start/End/Activity/LocalActivity/Decision/Signal/Timer/Child/ExternalSignal/Handler/Select/Version/Go, and `Edge{From, To, Label, Style}`.

SDK: built and tested against `go.temporal.io/sdk` v1.50.0 (current stable, needs Go 1.26). Since matching is by name, older SDKs should work wherever the function exists. The supported minimum gets established by a task that type-checks the fixtures against older tags, and goes in the README; it isn't guessed here.

## Out of scope for v1

Non-determinism lints (use `workflowcheck`); `Future` dataflow (parallel activities joined at `Get` are drawn at their call sites); `NewFuture`/`SideEffect`/`MutableSideEffect`/Nexus operations; activities started through interface values or reflection (warn `?`); cross-module helper inlining; sessions API; rendering to SVG/PNG (Mermaid text only); a review dashboard (JSON is the hook); byte-identical output with the Python tool (shapes, labels and IDs match; ordering may not).
