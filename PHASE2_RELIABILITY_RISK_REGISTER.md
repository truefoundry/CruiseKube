# Phase 2 Reliability Risk Register

## Scope

This pass focuses on runtime reliability risks in:

- process lifecycle and shutdown behavior
- fatal exits and panic paths
- goroutine and background worker management
- initialization order and hidden dependency hazards
- startup validation correctness

This is still a static analysis pass. I did not run the code, tests, or coverage tools.

## Severity Model

- Critical: likely to crash the process, deadlock lifecycle, or fail in common misconfiguration paths
- High: likely to cause production instability, hard-to-debug outages, or unsafe behavior under normal errors
- Medium: creates partial failures, observability gaps, or brittle testing/runtime behavior

## Ranked Findings

### 1. Critical: invalid controller mode can pass validation and fail later during runtime startup

Files:

- `pkg/config/viper.go`
- `cmd/cruisekube/main.go`

Details:

- `(*Config).ValidateControllerExecutionMode()` logs an invalid controller mode and returns `nil` in the default branch.
- `runcruisekube()` treats validation success as safe to continue.
- `setupControllerMode()` later hits the `default` branch of its controller-mode switch and calls `logging.Fatalf(...)`.

Why this matters:

- the process accepts an invalid configuration as valid
- the actual failure is deferred into runtime assembly
- failure mode depends on the code path rather than one deterministic validation boundary

Expected impact:

- startup crashes on malformed `controllerMode`
- confusing operator experience because the config layer and runtime layer disagree about validity

Recommendation:

- make `ValidateControllerExecutionMode()` return an error on invalid enum values
- keep config validation side-effect free and fail before runtime assembly starts

### 2. Critical: missing task configuration can trigger nil-pointer panic during controller startup

Files:

- `pkg/config/config.go`
- `cmd/cruisekube/main.go`

Details:

- `Config.GetTaskConfig(taskName)` returns `c.Controller.Tasks[taskName]`.
- If the tasks map is missing a key, the result is `nil`.
- `setupControllerMode()` dereferences these values immediately for all task types, for example `createStatsTaskConfig.Enabled` and `createStatsTaskConfig.Schedule`.

Why this matters:

- a partially specified config can crash the process with a panic
- this is a realistic configuration risk because defaults are not set for every task key in one place

Expected impact:

- startup panic before task scheduling
- weak diagnostics compared with explicit config validation errors

Recommendation:

- validate required task keys centrally
- return defaults or explicit errors instead of allowing nil task configs through

### 3. Critical: background server failures terminate the whole process through `log.Fatal` inside goroutines

Files:

- `cmd/cruisekube/main.go`
- `pkg/logging/logger.go`

Details:

- the metrics server, API server, and webhook server are all started in goroutines
- each goroutine calls `logging.Fatalf(...)` if `engine.Run(...)` or `RunTLS(...)` returns an error
- `logging.Fatalf(...)` calls `os.Exit(1)`

Why this matters:

- any background listener failure immediately exits the process from a goroutine
- this bypasses orderly shutdown and can skip deferred cleanup paths
- graceful error propagation is impossible because the process exits deep inside a background goroutine

Expected impact:

- hard process exits on bind failures, listener shutdowns, or transient server errors
- deferred telemetry shutdown is unlikely to run on these paths

Recommendation:

- replace `Fatalf` in goroutines with error reporting over channels or `errgroup`
- let one top-level lifecycle manager decide whether to exit and how to shut down

### 4. High: there is no coherent shutdown path for database, audit, scheduler, OOM observer, or OOM processor

Files:

- `cmd/cruisekube/main.go`
- `pkg/audit/audit.go`
- `pkg/cluster/scheduler.go`
- `pkg/oom/observer.go`
- `pkg/oom/processor.go`
- `pkg/adapters/database/database.go`

Details:

- database adapters expose `Close()`, but startup code never closes the database
- `audit.Audit` has `Close()`, but the singleton is never closed
- `Scheduler` has `Stop()`, but the main runtime never calls it
- `Observer` and `Processor` each have `Stop()`, but no runtime-owned shutdown wiring exists
- webhook-only mode uses `blockForever()` (`select {}`), which cannot react to OS signals or cancellation
- controller mode blocks inside `ScheduleAllTasks()` via `scheduler.Wait(...)`, which also has no signal integration here

Why this matters:

- the process has no explicit lifecycle management
- it is difficult to perform graceful shutdown in Kubernetes
- tests and local runs can leak goroutines or open resources

Expected impact:

- leaked background goroutines during tests or embedding
- non-graceful termination on container stop
- buffered audit events may be lost on shutdown

Recommendation:

- introduce a top-level runtime context derived from signal handling
- own all long-lived resources in one runtime struct
- close/stop resources in reverse startup order

### 5. High: lifecycle blocking is split across `blockForever()` and scheduler internals, making runtime control brittle

Files:

- `cmd/cruisekube/main.go`
- `pkg/cluster/singleClusterManager.go`
- `pkg/cluster/scheduler.go`

Details:

- webhook-only mode returns from setup and then blocks forever with `select {}`
- controller mode enters `clusterManager.ScheduleAllTasks()`, which schedules tasks and then blocks in `scheduler.Wait(...)`
- these two modes block in different ways, neither integrated with a shared shutdown policy

Why this matters:

- process behavior depends on execution mode in a non-uniform way
- there is no single place that owns runtime liveness
- this makes future graceful shutdown, health management, and error coordination harder

Expected impact:

- brittle operational behavior
- more complex future refactors because lifecycle is hidden in separate layers

Recommendation:

- centralize process waiting in one runtime manager
- make scheduler only schedule work, not own process blocking

### 6. High: handlers rely on global `storage.Stg`, but most handlers do not guard against it being nil

Files:

- `pkg/repository/storage/storage.go`
- `pkg/handlers/*`

Details:

- `storage.Stg` is assigned during controller startup only
- many handlers dereference `storage.Stg` directly
- `workload_summary.go` includes one nil check, but most other handlers do not

Why this matters:

- route safety depends on startup order and mode
- handlers can panic if called before initialization or in unexpected runtime configurations
- tests must mutate global state to be valid

Expected impact:

- nil-pointer panics in handler paths
- brittle tests and hidden coupling

Recommendation:

- remove the global and inject repository dependencies explicitly into handlers
- if globals remain temporarily, add consistent guardrails and fail closed with clear HTTP errors

### 7. High: webhook request handling depends on a package-global singleton client initialized by side effect

Files:

- `pkg/handlers/webhook_admission.go`
- `cmd/cruisekube/main.go`

Details:

- `InitRecommenderServiceClient(cfg)` uses `sync.Once` to initialize `recommenderServiceClient`
- `MutateHandler()` uses that global directly
- if `MutateHandler()` runs without prior initialization, it will dereference a nil pointer
- once initialized, the client cannot be refreshed for new config in the same process

Why this matters:

- runtime safety depends on one startup side effect
- tests and embedded runs become brittle
- config changes within the same process are effectively ignored

Expected impact:

- panic risk in non-standard startup paths
- stale config behavior in tests or future hot-reload scenarios

Recommendation:

- inject the client into the webhook handler dependency set
- remove `sync.Once` from request path dependencies

### 8. High: `MustGet(...)` dependency resolution turns missing middleware state into panics

Files:

- `pkg/middleware/middleware.go`
- `pkg/config/utils.go`
- multiple files under `pkg/handlers`

Details:

- many handlers call `c.MustGet("clusterManager")`
- config lookup uses `c.MustGet("appConfig")`
- `MustGet` panics when middleware state is absent or miswired

Why this matters:

- route registration mistakes become process-level handler panics
- panic recovery may hide dependency wiring defects as generic 500s
- this makes transport behavior less explicit and harder to validate

Expected impact:

- hidden runtime coupling between route registration and handler behavior
- avoidable panics instead of controlled HTTP error responses

Recommendation:

- use explicit typed handler dependencies
- at minimum, replace `MustGet` with guarded retrieval and explicit HTTP failure responses

### 9. Medium: OOM watcher retry loop is not fully cancellation-aware during sleep

Files:

- `pkg/oom/informer.go`

Details:

- `watchEventsWithRetries()` loops until `ctx.Done()`
- after a watch attempt ends, it performs `time.Sleep(waitTime)`
- that sleep does not select on `ctx.Done()`

Why this matters:

- shutdown responsiveness is delayed by the backoff period
- not catastrophic, but it adds shutdown lag and complicates deterministic tests

Expected impact:

- slower-than-necessary stop behavior

Recommendation:

- replace `time.Sleep` with a timer/select that also listens on `ctx.Done()`

### 10. Medium: audit writes are intentionally lossy and may silently drop events under load or at shutdown

Files:

- `pkg/audit/audit.go`

Details:

- `Audit.Record()` is non-blocking and drops events when the buffer is full
- current shutdown paths do not call `Audit.Close()`

Why this matters:

- audit behavior is intentionally best-effort, but today there is little control or visibility around dropped events
- without orderly shutdown, buffered events are likely to be lost

Expected impact:

- missing operational audit trails during bursty or failing conditions

Recommendation:

- preserve non-blocking behavior if desired, but add dropped-event metrics/log counters
- wire `Close()` into controlled shutdown

## Systemic Reliability Themes

### Process exits are too deep in the stack

There are many `logging.Fatalf(...)` paths, and several are inside goroutines. That means failures bypass coordinated cleanup and make graceful recovery impossible.

### Runtime liveness is implicit, not owned

The binary stays alive via:

- a scheduler wait path in controller mode
- `select {}` in webhook-only mode

Neither is tied to a signal-aware root context.

### Initialization order is not enforced by types

Critical runtime dependencies depend on package-global assignment:

- storage
- audit
- webhook client

That creates hidden startup contracts and panic risk.

### Validation does not fully protect runtime startup

Configuration validation is not strict enough to prevent at least two realistic failure modes:

- invalid controller mode
- missing task configuration entries

## Recommended Reliability Backlog

### First tranche

1. Make config validation strict and side-effect free.
2. Validate every required task config key before controller setup.
3. Replace goroutine-local `Fatalf` exits with top-level error coordination.

### Second tranche

1. Introduce signal-driven root context and coordinated shutdown.
2. Close database, audit, scheduler, OOM observer, and OOM processor explicitly.
3. Remove package-global runtime dependencies from handlers.

### Third tranche

1. Split lifecycle management out of `cluster.Manager`.
2. Add characterization tests for startup behavior and dependency wiring.
3. Add tests for invalid config permutations and missing task definitions.

## Suggested Follow-Up Test Cases

- invalid `controllerMode` must fail validation
- missing required task configs must fail validation without panic
- webhook handler must fail safely when dependency wiring is incomplete
- server startup errors must surface to the top-level runtime manager without `os.Exit` in goroutines
- shutdown must flush audit and stop background workers
