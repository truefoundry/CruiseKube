# Phase 1 Architecture Review

## Scope

This pass focuses on:

- backend entrypoints and startup wiring
- package responsibilities and dependency boundaries
- global state and lifecycle management
- immediate structural risks that will affect later simplification work

This is a static code review pass only. I did not run tests, builds, or coverage commands yet.

## Repository Shape

### Backend

- The Go backend lives under `cmd/` and `pkg/`.
- There is one primary CLI/runtime entrypoint: `cmd/cruisekube/main.go`.
- The backend currently contains `76` Go files under `cmd/` and `pkg/`.
- The largest concentration of code is in:
  - `pkg/task`
  - `pkg/handlers`
  - `pkg/adapters`

### Frontend

- `cruiseKube-frontend/` is a git submodule, not just a nested folder.
- `.gitmodules` points it at `https://github.com/truefoundry/cruiseKube-frontend.git`.
- Its local `.git` file resolves to `../.git/modules/cruiseKube-frontend`.
- Any simplification plan should treat frontend structural changes as a cross-repo concern unless the submodule boundary is intentionally being removed.

### Supporting Surface

- Helm charts live in `charts/`.
- e2e manifests live in `test/e2e/`.
- docs and release artifacts also live in the repo, so internal cleanup needs to preserve deployment and API contracts.

## High-Level Architecture Map

### 1. Process Bootstrap

`cmd/cruisekube/main.go` currently owns:

- CLI definition through Cobra
- flag-to-Viper binding
- config loading and validation
- telemetry initialization
- metrics server startup
- webhook server startup
- controller startup
- database initialization
- storage repository initialization
- audit singleton initialization
- Kubernetes and Prometheus client creation
- HTTP server startup
- OOM observer/processor startup
- task construction and task scheduling

This makes `main.go` the composition root, runtime orchestrator, and process lifecycle controller all at once.

### 2. Configuration Layer

`pkg/config` provides:

- the main config schema in `config.go`
- Viper-backed loading and defaults in `viper.go`
- config retrieval from Gin context in `utils.go`

Config precedence is:

1. defaults via `v.SetDefault(...)`
2. config file via `ReadInConfig()`
3. environment variables via `AutomaticEnv()`
4. CLI flags bound onto the shared Viper instance in `main.go`

The current loading path is workable, but the boundary is split between `main.go` and `pkg/config`, so configuration ownership is not fully centralized.

### 3. Integration Adapters

Adapters are concentrated in:

- `pkg/adapters/database`
- `pkg/adapters/kube`
- `pkg/adapters/metricsProvider/prometheus`

These packages provide the infrastructure-facing clients. The rest of the system depends heavily on them, especially through the `ports.Database` interface and the Prometheus provider.

### 4. Domain/Orchestration Layer

Core orchestration packages are:

- `pkg/cluster`
- `pkg/task`
- `pkg/oom`
- `pkg/repository/storage`

`pkg/cluster` exposes a `Manager` abstraction, but the active implementation is `SingleClusterManager`, which currently manages a single in-memory cluster entry.

`pkg/task` contains most of the backend’s operational behavior and several of the largest files in the codebase. This is the current center of backend complexity.

### 5. Transport Layer

The runtime exposes three Gin servers:

- API server from `server.SetupServerEngine(...)`
- webhook server from `server.SetupWebhookServerEngine(...)`
- metrics server from `server.SetupMetricsServerEngine()`

HTTP routes are declared centrally in `pkg/server/server.go`, but business logic is implemented in `pkg/handlers`.

## Observed Dependency Flow

The runtime dependency chain is broadly:

`main` -> `config` -> `database`/`storage`/`audit` -> `kube`/`prometheus` -> `cluster.Manager` -> `server` + `handlers` + `oom` + `task`

More concretely:

1. `main.go` builds config.
2. `main.go` constructs database and wraps it with `storage.NewStorageRepo(...)`.
3. `main.go` stores that repository in the package-global `storage.Stg`.
4. `main.go` constructs the package-global `audit.Recorder`.
5. `main.go` creates Kubernetes and Prometheus clients.
6. `main.go` creates a `cluster.Manager`.
7. `main.go` passes that manager into middleware and server setup.
8. Handlers resolve dependencies partly from Gin context and partly from package globals.
9. Tasks and OOM processing also reach shared package globals for storage/audit behavior.

This is a mixed dependency model: some dependencies are injected explicitly, while others are accessed through singleton state.

## Package Responsibilities

### `cmd/cruisekube`

- Composition root and runtime bootstrap.
- Currently too large for a thin entrypoint.

### `pkg/config`

- Configuration schema, defaults, loading, validation.
- Validation behavior is inconsistent: some branches return errors, some print directly, and one invalid controller-mode branch logs an error but returns `nil`.

### `pkg/server`

- Route registration and Gin engine assembly.
- Thin, centralized, and mostly appropriate.

### `pkg/middleware`

- Dependency injection into Gin context.
- request-scoped context decoration
- placeholder auth
- cluster existence guard

This package is small, but its use of `c.MustGet(...)` in downstream code means dependency failures become panics instead of explicit errors.

### `pkg/handlers`

- HTTP and webhook transport handlers.
- Broad surface area with many route handlers and some heavy business logic.
- Depends on both Gin-context dependencies and package-level globals.

This is a major simplification target.

### `pkg/task`

- Scheduled background operations.
- Contains several very large files and most of the core controller behavior.
- Mixes scheduling inputs, orchestration, Kubernetes interactions, Prometheus interactions, storage, and recommendation logic.

This is the highest-complexity package in the repo.

### `pkg/repository/storage`

- Thin repository wrapper around `ports.Database`.
- Central data access seam.
- Also exposes package-global `Stg`, which makes the package both a dependency boundary and a global service locator.

### `pkg/audit`

- Async buffered audit writer around `ports.Database`.
- Behavior is reasonable in isolation, but exposure through package-global `Recorder` creates hidden dependencies.

### `pkg/cluster`

- Cluster abstraction plus task scheduler.
- `SingleClusterManager` is effectively a task registry plus one cluster entry.
- `ScheduleAllTasks()` blocks forever by calling `scheduler.Wait(...)`, which makes the manager both scheduler and process blocker.

## Global State And Hidden Dependencies

The following package-level mutable globals are part of the active runtime path:

- `cmd/cruisekube/main.go`: shared Viper instance `v`
- `pkg/logging/logger.go`: `defaultLogger`
- `pkg/repository/storage/storage.go`: `storage.Stg`
- `pkg/audit/audit.go`: `audit.Recorder`
- `pkg/handlers/webhook_admission.go`: `recommenderServiceClient`

Impact:

- handler behavior is not self-contained from function signatures
- tests cannot isolate dependencies easily without mutating package globals
- initialization order matters in ways the type system does not enforce
- concurrent tests will be harder to make reliable
- future multi-process or multi-tenant evolution becomes harder

## File Size Hotspots

The largest files in the backend include:

- `pkg/task/taskApplyRecommendation.go` (`858` lines)
- `pkg/adapters/metricsProvider/prometheus/promql.go` (`755` lines)
- `pkg/task/utils/util.go` (`682` lines)
- `pkg/task/utils/workload_handler.go` (`676` lines)
- `pkg/adapters/database/database.go` (`600` lines)
- `pkg/task/taskCreateStats.go` (`460` lines)
- `pkg/handlers/controller_webhook_handler.go` (`427` lines)
- `pkg/task/taskFetchMetrics.go` (`417` lines)
- `pkg/task/taskDisruptionForce.go` (`397` lines)

These files should be treated as likely mixed-responsibility modules until proven otherwise.

## Immediate Structural Findings

### 1. `main.go` is overloaded

`cmd/cruisekube/main.go` is doing too much. The highest-value early simplification is to extract:

- config/flag bootstrap
- infrastructure assembly
- controller runtime assembly
- webhook runtime assembly
- task registration

This should become a small composition root that delegates to explicit builders.

### 2. Dependency injection is inconsistent

The codebase currently mixes:

- explicit constructor injection
- Gin context injection
- package-global singletons

That inconsistency increases cognitive load and makes behavior harder to test.

### 3. Controller lifecycle is tightly coupled to scheduling

`SingleClusterManager.ScheduleAllTasks()` schedules enabled tasks and then blocks forever. That means:

- there is no clear separation between task registration and process lifecycle
- shutdown coordination is not explicit
- the scheduler’s `Stop()` path exists, but is not wired into process lifecycle here

### 4. Config validation needs normalization

The current config validation flow has at least one correctness issue:

- `ValidateControllerExecutionMode()` logs an invalid controller mode and returns `nil`

That means an invalid controller mode can pass validation and fail later in runtime setup.

There is also mixed reporting behavior:

- `fmt.Println(...)`
- returned errors
- logging side effects

This should be unified into pure validation that returns structured errors only.

### 5. Handlers are tightly bound to global repositories

Multiple handlers directly reach `storage.Stg`, and some also use `audit.Recorder`. This makes route behavior depend on startup side effects rather than explicit dependencies.

### 6. Webhook handler uses a singleton client with `sync.Once`

`pkg/handlers/webhook_admission.go` initializes `recommenderServiceClient` once and then uses it globally. That creates:

- hidden cross-request shared state
- hard-to-test behavior
- possible stale config if webhook settings are ever expected to change across tests or embedded runtimes

## Test Surface Snapshot

From the current repo tree, backend unit tests appear very limited:

- `pkg/adapters/database/database_test.go`

There is also a mirrored test file under a tooling path:

- `.claude/worktrees/code-quality-plan/pkg/adapters/database/database_test.go`

That second file is not part of the main backend package tree and should not be counted as real coverage for production code.

This means the current Go unit-test safety net is extremely thin relative to the size and coupling of the codebase.

## Suggested First Refactor Targets

These are the best early targets because they reduce coupling without immediately changing core business logic.

### Target 1: Extract runtime builders from `main.go`

Create explicit assembly helpers such as:

- controller runtime builder
- webhook runtime builder
- shared infrastructure builder

Goal:

- make startup testable
- reduce one-file orchestration complexity
- make runtime dependencies visible

### Target 2: Remove package-global storage and audit access from handlers

Introduce explicit handler dependencies, likely through:

- a handler struct
- grouped dependency container passed into route registration

Goal:

- stop relying on `storage.Stg` and `audit.Recorder`
- improve testability and clarity

### Target 3: Separate scheduler control from cluster management

Keep `cluster.Manager` focused on cluster access and task registration, and move lifecycle blocking/shutdown handling into a dedicated runtime layer.

Goal:

- cleaner process lifecycle
- easier testing of scheduled behavior
- clearer shutdown path

### Target 4: Normalize config validation

Make config validation:

- deterministic
- side-effect free
- strict on invalid enum values
- centrally tested

Goal:

- fail early and predictably
- reduce runtime-only configuration failures

## Open Questions For Later Phases

- Should the API server and webhook server remain in the same binary and process by default?
- Is the single-cluster manager a temporary simplification or the intended long-term model?
- Should the `pkg/task` package be split into domain logic plus integration/orchestration layers?
- Should the frontend submodule remain a submodule, or should release/build tooling treat it as an external artifact?
- Which API routes and task flows are operationally critical and require characterization tests before refactor work?

## Proposed Next Deliverable

Phase 2 should produce a reliability risk register with a focus on:

- fatal exits and panic paths
- goroutine lifecycle and shutdown behavior
- task concurrency semantics
- nil/global initialization hazards
- config validation correctness issues
