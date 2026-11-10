# timetell: analysis

Goal: draw every shape a Temporal Go workflow can take (activities, decisions, signals, timers, child workflows, coroutines, loops) as Mermaid, without running it, so an agent-written workflow can be reviewed at a glance.

Reference: `temporalio-graphs` 0.4.0 (PyPI, MIT, github.com/LucaDeLeo/temporalio-graphs), itself a port of the .NET `Temporalio.Graphs`. Read: the wheel source plus the repo's `examples/*/expected_output*.md`.

## How the Python tool actually works

It collects activities, `to_decision` calls, `wait_condition` calls and child calls from the AST, **sorts them all by source line**, then emits one path per 2^n combination of decision/signal outcomes and unions the edges into a flowchart (`max_paths=1024` hard cap). Consequences, checked against its own code and golden files:

- An activity under an unmarked `if`, `for` or `while` is drawn on every path, in line order. Plain branches and loops aren't refused; they're just flattened silently.
- Path count is 2^n whatever the structure, so sequential decisions explode even when they're independent.
- Its own `parent_child_workflow` golden files show child mode still incomplete: decisions render as `[rect]`, the child as `[Name]` not `[[Name]]`, and the four "inline" paths are identical. (Checked 2026-11-09 against `examples/parent_child_workflow/expected_output_{inline,reference,subgraph}.md`: `HighValue[High Value]`, `PaymentWorkflow[Payment Workflow]`, and inline Paths 0-3 are the same sequence.)

## Port as-is

- **Shapes:** `s((Start))`, `e((End))`, activity `Name[Name]`, decision `d0{Label}` with `yes`/`no` edges, signal wait `Name{{Label}}` with `Signaled`/`Timeout` edges, child workflow `[[Name]]`, external signal send `[/Signal 'name'\]`, signal handler `{{name}}` (styled `fill:#e6f3ff,stroke:#0066cc`), unresolved target `[/?/]`, cross-workflow edge `-.name.->`, `flowchart LR` (changed 2026-11-10, Leigh: `flowchart TD` by default, because long workflows read better top-down; LR stays possible through config, timetell-pas.7). Node IDs as Python's: `s`, `e`, `d<n>`, activity name. One change: a second call site of the same activity gets `Name_2`, `Name_3` with the same label. Python merges repeat calls into one node (`renderer.py` dedupes by name), which is harmless without cycles but would read as a loop here. Nodes are emitted before edges, start first, end last, the rest sorted.
- **Config** (`Config` struct, CLI flags of the same names): `StartLabel`/`EndLabel`, `DecisionTrue/FalseLabel`, `SignalSuccess/TimeoutLabel`, `SplitNamesByWords`, `MaxDecisionPoints` (warning), `ChildWorkflowExpansion` = `reference|inline|subgraph` (the handoff's "collapsed/inline/separate" are these three; Python's names are kept so output is comparable), `ResolveSignalTargets`, `SignalVisualizationMode` = `subgraph|unified`, `SignalMaxDiscoveryDepth` (default 10), `WarnUnresolvedSignals`, `OutputFormat` = `mermaid|paths|full`. Dropped: `is_building_graph`, `exit_after_building_graph` (runtime-mode leftovers from .NET), `signal_target_search_paths` (package patterns replace it), and `max_paths` as a hard error (see the Paths row below). Redundant with `OutputFormat`, so also dropped: `include_validation_report`, `include_path_list`, `suppress_validation`. `graph_output_file` is the CLI's `-o`. Fixed at Python's defaults in v1, with no knob: `show_external_signals` (on), `external_signal_label_style` (`name-only`), `signal_resolution_strategy` (`by_name`). Changed 2026-11-10 (timetell-pas.7, Leigh): `SplitNamesByWords` splits activity names (signal names too, with timetell-pas.9), never decision labels; Python splits only signals and decisions (`renderer.py:240,293`), but decision labels come from `//timetell:decision` text the author already wrote. Added `Direction` = `TD|LR`. Added 2026-11-10 (timetell-pas.8): `MaxPaths` (default 1024, Python's `max_paths`) caps only the listed paths; `paths` prints the path list alone and `full` prints graph, path list, then report, as Python does. Added 2026-11-10 (timetell-pas.13): `Workflow` graphs only the root of that name (the CLI's `-workflow`).
- **Reporting:** a validation report of warnings (unresolved name, dynamic activity name, too many decisions), each with file:line, appended in `full` format. Python's `unreachable` warning means "activity defined but never called"; here that is the registered-but-never-invoked lint below.

## Where Go does better: decisions

| Topic | Decision | Why |
|---|---|---|
| Loading | `go/packages` with full `types.Info` | `ExecuteActivity(ctx, a.Charge)` resolves to a `*types.Func` whatever the import alias, method value or wrapper. The SDK is matched by `types.Object`, never by import, so timetell itself doesn't import the SDK. Package funcs (`ExecuteActivity`, `Go`, `GetVersion`) match on `go.temporal.io/sdk/workflow` plus name. Methods match on the declaring package being `go.temporal.io/sdk/internal` or `go.temporal.io/sdk/internal/common/workflowcontext`. `workflow.Selector` aliases `internal.Selector`; `workflow.Context` and `ReceiveChannel` alias `internal`, which re-aliases them to `workflowcontext`, where they are declared (checked in v1.50.0: `workflow/context.go:11`, `workflow/deterministic_wrappers.go:18,25`, `internal/context.go:20`, `internal/workflow.go:189,201`, `internal/common/workflowcontext/{context.go:13,channel.go:25}`; corrected 2026-11-09 when the classifier missed `Receive`). Registration matches the `RegisterWorkflow`/`RegisterActivity` interface methods in `go.temporal.io/sdk/worker`. |
| Names | constant-fold through `types.Info.Types[e].Value` | Signal names, version change IDs and string activity names can be `const`s, not only literals. A truly dynamic value becomes `?` plus a warning, as in Python. |
| Control flow | walk the type-checked AST of each function body, statement by statement (not `go/cfg`, not SSA) | The graph is built straight from the statement structure and paths are never enumerated. Statements with no Temporal effect collapse. Every rule below is defined on a statement (`if`, `for`, a directive comment, a closure body), and `go/cfg` neither records branch conditions nor enters closures. SSA is heavier and loses the source positions we need for labels. Changed 2026-11-09 from `go/cfg`, before any builder code existed; reasons, costs and the builder design are in [control-flow.md](control-flow.md). |
| Paths | count by DP over the DAG; enumerate only for `paths` output | Removes the 2^n blow-up. `MaxPaths` caps only the printed list. Lines keep Python's `Path N: Start → A → B → End` with raw activity names; the header drops Python's `(2^n = N paths)`, which is false here. A back edge ends a loop's one iteration and continues to the loop's exits, so each loop is listed 0 or 1 times; a forever loop's paths are the ones that leave it on the first pass. |
| Loops | first-class: one `Loop` node per `for`/`range` whose body has Temporal effects, `l0([each item in items])`, with a `do` edge into the body, a `loop` back edge to the head and a `done` edge out | Mermaid flowcharts allow cycles, and each `for`/`range` is one AST statement with its header and body, so the truth costs little. Label: `each <vars> in <expr>` for range, `while <cond>` for a condition, `forever` for `for {}` (no `done` edge; its exits are the `break`/`return` branches inside). Nested loops nest; a loop with no Temporal effects collapses like any other statement. Path list: 0 or 1 iteration. Python flattens loops silently, so this removes a limitation. Core, not descopable. Drawing changed 2026-11-10 (Leigh, "style C", timetell-pas.14): each loop body is a dashed, tinted `subgraph L<n> ["<label>"]`, nested for nested loops, holding the loop node `l<n>{{"↻"}}:::loop`. Back edges are dotted (`-.->`) and coloured through `linkStyle`. The `do` and `loop` labels are not drawn; the graph model keeps them. A loop's `done` edge is solid, thick and green, so it reads as the way out (Leigh, 2026-11-10). A `break` out of a loop (`Edge.Break`, set by the walker) is drawn just as thick in the default colour; a `return` out of a loop is not a break. |
| Unmarked branches | any `if`/`switch` whose arms differ in Temporal effects becomes a decision labelled with its condition source (truncated) | The diagram never presents conditional work as unconditional. Branches with identical effects collapse. |
| Error returns | an `if err != nil` arm that only returns, with no Temporal call, is not drawn. An error arm that does work (compensation) is an `error`-labelled edge out of the preceding node, not a diamond | Go checks an error after every activity. Under the rule above each check would become an `{err != nil}` diamond and bury the business decisions. Only nil-checks on an `error`-typed value get this treatment; a validation early return is still a decision. |
| Decision labels | `//timetell:decision Label` comment directive on the `if`/`case` | Zero runtime cost and no import. A marker func (`graph.Decision`) costs a dependency in workflow code. Treating every activity-derived `if` as a decision through type info is noisy, and the rule above already shows them. The same directive labels a `for`, an `AwaitWithTimeout` or a `Select`. |
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

- Lints: activity invoked but not registered; activity registered but never invoked; signal received but never sent by any analysed workflow (and the reverse); `GetVersion` with no branch for `DefaultVersion`/`min`; `ActivityOptions` with neither `StartToCloseTimeout` nor `ScheduleToCloseTimeout` on the ctx that reaches `ExecuteActivity`. The two registration lints are skipped, with a note, when the loaded packages contain no `Register*` call at all. Code after a `return` is left to `go vet`'s `unreachable` pass.
- **Non-determinism: don't rebuild it.** Temporal's `go.temporal.io/sdk/contrib/tools/workflowcheck` already flags `time.Now`, `rand`, map range and goroutines in workflow code. Document running it next to timetell. Importing it would break the "stdlib + x/tools only" rule.
- Diff: `timetell diff A B` checks out each rev with `git worktree`, builds both graphs, and renders the union with `classDef added` (green), `removed` (red, dashed) and `changed` (amber, label or edge set differs). Nodes match on stable keys (kind + name + nth occurrence).

## Architecture

```
cmd/timetell        CLI (flag pkg): graph | lint | diff
timetell            public API: Config, Load(patterns), Build(Config) → []*Graph, Lint, Diff
internal/load       go/packages loading; SDK call classification by types.Object (pkg path + name)
internal/discover   registered workflows/activities, roots, signal handler index
internal/flow       AST walk → Graph: collapse, decisions, selectors, coroutines, loops, helper inlining (control-flow.md)
internal/render     Mermaid + JSON; deterministic ordering
internal/lint       review findings (Finding{Pos, Rule, Msg})
analyzer            go/analysis wrapper (later)
testdata/<fixture>  separate go.mod requiring go.temporal.io/sdk; workflow.go + expected.mmd
```

Key types: `Graph{Workflow, Nodes []Node, Edges []Edge, Subgraphs, Warnings}`, `Node{ID, Key, Kind, Label, Pos}` with `Kind` = Start/End/Activity/LocalActivity/Decision/Loop/Signal/Timer/Child/ExternalSignal/Handler/Select/Version/Go, and `Edge{From, To, Label, Style}`.

SDK: built and tested against `go.temporal.io/sdk` v1.50.0 (current stable, needs Go 1.26). Since matching is by name, older SDKs should work wherever the function exists. The supported minimum gets established by a task that type-checks the fixtures against older tags, and goes in the README; it isn't guessed here.

## Out of scope for v1

Non-determinism lints (use `workflowcheck`); `Future` dataflow (parallel activities joined at `Get` are drawn at their call sites); `NewFuture`/`SideEffect`/`MutableSideEffect`/Nexus operations; activities started through interface values or reflection (warn `?`); cross-module helper inlining; sessions API; rendering to SVG/PNG (Mermaid text only); a review dashboard (JSON is the hook); byte-identical output with the Python tool (shapes, labels and IDs match; ordering may not).
