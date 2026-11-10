# Fixtures

A separate module (`go.temporal.io/sdk` v1.50.0), so the root module never imports the SDK. Each directory holds `workflow.go` (workflows, activities and a `Register(worker.Worker)`) and `expected.mmd`, the intended Mermaid output with default config. A fixture with several workflows has one graph per workflow, sorted by name, each headed by a `%% Name` comment. `go test ./... -update` rewrites the goldens.

| Fixture | Exercises | Task |
|---|---|---|
| trivial | Start → End, no Temporal calls | .1 |
| simple_linear | linear activities, `if err := …; err != nil { return err }` not drawn | .5, .6 |
| money_transfer | two `//timetell:decision` decisions, reconvergence, error check after every activity (port). Also `expected_labels.mmd` (custom labels, LR), `expected_nosplit.mmd` (`SplitNamesByWords` false), `expected_full.txt`, `expected_paths.txt` and `expected_paths_capped.txt` (`MaxPaths` 2) | .5, .6, .7, .8 |
| approval | `AwaitWithTimeout` whose `ok` feeds an `if`: Signaled/Timeout (port of signal_workflow). `expected_paths.txt`: the wait counts as a decision point | .9 |
| waits | single-edge waits: `Receive`, `Await` labelled with the signal it reads, `Sleep`/`NewTimer` with constant and variable durations; `if !ok` swaps Signaled/Timeout | .9 |
| parent_child | `ExecuteChildWorkflow`, reference mode; two roots (port) | .10 |
| signal_flow | `SignalExternalWorkflow` and `GetSignalChannel().Receive` across three workflows (port) | .9, .12 |
| saga_compensation | error arm that runs an activity: `error` edge, no diamond | .5, .6 |
| repeated_activity | one activity at two call sites: `SendEmail`, `SendEmail_2` | .5, .6 |
| loop_retry | `for` with a condition, `break` on success | .5 (terminates), .14 |
| loop_range | `for _, item := range items` | .14 |
| loop_forever | `for {}` with `break` and a continue-as-new return | .14 |
| loop_nested | retry loop (directive-labelled) inside a `range`: nested style C subgraphs. `expected_paths.txt`: 0 or 1 iteration per loop | .14, .8 |
| selector_signal_timer | `Selector` with `AddReceive` + `AddFuture(timer)` | .15 |
| selector_loop | `Selector` inside `for {}` | .14, .15 |
| helper_split | activities in unexported helpers taking `workflow.Context` | .18 |
| coroutine | `workflow.Go` | .17 |
| unregistered_activity | lint: activity invoked, never registered | .20 |
| fallback | no worker: roots are exported funcs taking workflow.Context | .4 |
| classify | aliased import, method value, const names, dynamic activity name. `expected_full.txt` is `OutputFormat` full with its validation report | .3, .7 |
| walker | one workflow per case in docs/control-flow.md's test table, plus defer/closure-var/native go+select warnings; fallback roots. `go vet` reports its `AfterReturn` unreachable code on purpose | .5 |
| unused_activity | lint: activity registered, never invoked | .20 |
| missing_timeout | lint: `ActivityOptions` without StartToClose/ScheduleToClose | .21 |
| getversion_no_default | `GetVersion` switch with no `DefaultVersion` case; lint | .16, .21 |
