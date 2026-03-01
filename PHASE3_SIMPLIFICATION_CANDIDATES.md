# Phase 3 Simplification Candidate Review

## Scope

This pass identifies the parts of the codebase that will yield the biggest quality and maintainability gains from structural simplification.

Focus areas:

- oversized files and packages
- mixed-responsibility modules
- repeated construction and control-flow patterns
- weak package boundaries
- utility buckets that hide domain logic

This is still a static review. I did not change product code or run tests.

## Primary Simplification Targets

### 1. `cmd/cruisekube/main.go` should become a thin composition root

Current issues:

- owns CLI setup, config binding, config load, telemetry, metrics listener, webhook listener, controller setup, DB creation, repo creation, audit setup, OOM startup, task registration, and process blocking
- contains repeated configuration extraction and task registration code
- mixes lifecycle policy with dependency assembly

Simplification direction:

- extract `buildRuntimeConfig()`
- extract `buildInfrastructure()`
- extract `startControllerRuntime()`
- extract `startWebhookRuntime()`
- extract `registerControllerTasks()`

Expected benefit:

- smaller blast radius for startup changes
- better lifecycle testing
- clearer ownership of runtime dependencies

### 2. `pkg/task` should be split by domain logic vs orchestration vs infra actions

`pkg/task` is the largest complexity center in the repo (`6077` lines total across task files).

The highest-cost files are:

- `pkg/task/taskApplyRecommendation.go` (`858` lines)
- `pkg/task/utils/util.go` (`682` lines)
- `pkg/task/utils/workload_handler.go` (`676` lines)
- `pkg/task/taskCreateStats.go` (`460` lines)
- `pkg/task/taskFetchMetrics.go` (`417` lines)
- `pkg/task/taskDisruptionForce.go` (`397` lines)

Current issues:

- task files mix scheduling interface methods, config decoding, cluster reads, Prometheus queries, policy decisions, resource mutation, metrics emission, and audit writes
- `utils` contains both generic helpers and core domain behavior
- task constructors repeat the same metadata decoding and wrapper boilerplate

Simplification direction:

- split task packages into:
  - runtime task wrappers
  - domain services / policy evaluators
  - Kubernetes mutation helpers
  - Prometheus query adapters
  - workload/stat transformation helpers
- reduce `utils` to genuinely reusable helpers only
- introduce shared task base helpers for metadata parsing and trivial task interface methods

Expected benefit:

- easier unit testing of pure decision logic
- clearer separation between “compute recommendation” and “apply recommendation”
- less duplication across task implementations

### 3. `pkg/handlers` should be split into transport adapters plus explicit service layer

The handler package is the second major hotspot (`2106` lines in top-level handler files).

Key large files:

- `pkg/handlers/controller_webhook_handler.go` (`432` lines)
- `pkg/handlers/workload_summary.go` (`342` lines)
- `pkg/handlers/handlers.go` (`248` lines)
- `pkg/handlers/killswitch.go` (`220` lines)
- `pkg/handlers/workload_detail.go` (`201` lines)

Current issues:

- handlers parse requests, fetch dependencies, perform business decisions, call storage, query Prometheus, shape responses, and sometimes emit audit records
- several handlers depend on package globals and Gin context simultaneously
- control flow is repetitive, especially “log and return empty patch list”

Simplification direction:

- create explicit handler/service dependency structs
- move non-HTTP logic into service methods
- keep handlers limited to:
  - request parsing
  - auth/context extraction
  - service invocation
  - response mapping
- centralize common webhook empty-response handling

Expected benefit:

- lower cognitive load in HTTP endpoints
- better testability without full Gin setup
- fewer hidden dependencies

## Mixed-Responsibility File Findings

### `pkg/task/taskApplyRecommendation.go`

This file currently combines:

- task metadata decoding
- runtime entrypoint logic
- recommender client construction
- override fetching
- optimization workflow orchestration
- strategy execution
- pod mutation / eviction side effects
- snapshot persistence
- audit emission
- metrics updates

Best split:

- task wrapper
- recommendation planner
- recommendation executor
- snapshot writer
- audit/metrics reporting adapter

### `pkg/task/taskCreateStats.go`

This file currently combines:

- task metadata decoding
- workload discovery
- stale workload cleanup
- recent-stat filtering
- Prometheus batch querying
- prediction model orchestration
- HPA / PDB checks
- stat construction
- storage writes

Best split:

- workload inventory loader
- metrics collection orchestrator
- prediction pipeline
- stats assembler
- persistence adapter

### `pkg/handlers/controller_webhook_handler.go`

This file currently combines:

- admission request parsing
- config and cluster dependency lookup
- workload lookup
- apply eligibility evaluation
- JSON patch generation
- resource patch mutation logic
- disruption annotation logic
- audit logging

Best split:

- admission transport handler
- mutation decision service
- patch builder
- audit notifier

### `pkg/adapters/database/database.go`

This file currently combines:

- database factory bridging
- migrations
- persistence implementation
- JSON marshaling/unmarshaling of domain types
- query composition for multiple aggregates

Best split:

- factory/bootstrap
- migration logic
- workload repository persistence
- OOM repository persistence
- recommendation repository persistence
- settings/audit persistence

## Repeated Patterns Worth Consolidating

### 1. Task constructor boilerplate

Repeated across multiple task files:

- metadata struct creation
- `taskConfig.ConvertMetadataToStruct(...)`
- logging on decode failure
- basic wrapper initialization

Simplification:

- add a generic helper for metadata decoding
- add an embedded base config or helper for `GetName()`, `GetSchedule()`, `IsEnabled()`

### 2. Repeated trivial task methods

Nearly every task repeats:

- `GetCoreTask()`
- `GetName()`
- `GetSchedule()`
- `IsEnabled()`

Simplification:

- use a shared base task type or narrow the interface
- reconsider whether `GetCoreTask() any` is necessary at all

### 3. Repeated webhook “safe no-op” responses

`pkg/handlers/controller_webhook_handler.go` contains many branches that log and return:

- `c.JSON(http.StatusOK, []client.JSONPatchOp{})`

Simplification:

- centralize a `respondNoPatch(...)` helper
- optionally centralize structured reason logging to keep branch logic concise

### 4. Repeated workload wrapper logic in `pkg/task/utils/workload_handler.go`

Deployment, StatefulSet, and DaemonSet wrappers repeat nearly identical implementations for:

- loading pod containers
- loading init containers
- selector conversion
- fallback behavior

Simplification:

- introduce a smaller shared helper around:
  - selector resolution
  - fallback-to-template logic
  - dynamic pod container merge

### 5. Repeated JSON persistence mapping in database adapter

The database layer repeatedly:

- loads rows
- unmarshals JSON blobs into domain structs
- patches timestamps
- rewraps data into API/domain containers

Simplification:

- extract row-to-domain mappers
- separate persistence DTOs from domain models more clearly

## Package Boundary Problems

### `pkg/task/utils` is too broad

Current contents include:

- Kubernetes mutations
- workload discovery
- GPU detection
- Prometheus helpers
- stats reading
- version cache
- recommendation application helpers
- node-stat building
- prediction logic

This is not a coherent package boundary.

Recommended split:

- `pkg/domain/workloads` or similar for workload inspection and mapping
- `pkg/domain/recommendations` for recommendation rules and transforms
- `pkg/infra/kubeops` for pod update / eviction helpers
- `pkg/infra/promquery` or keep adapter-local Prometheus query helpers
- `pkg/domain/stats` for stat assembly and transformations

### `pkg/types` likely carries both storage and transport concerns

The repo uses shared types broadly across:

- storage
- handlers
- tasks
- audit

That usually makes boundaries leaky over time.

Recommended split:

- transport DTOs
- persisted models / repository payloads
- core domain objects

This should be done carefully and incrementally, but it is an important long-term cleanup target.

### `pkg/repository/storage` is both abstraction and service locator

`storage.Storage` is a legitimate boundary, but `storage.Stg` turns the package into a global service locator.

Recommended split:

- keep repository type
- remove package-global singleton usage
- pass storage explicitly where needed

## Concrete Candidate Refactor Batches

### Batch 1: low-risk structural cleanup

- extract common task config decoding helper
- add shared webhook no-op response helper
- reduce repetitive logging/response branches in webhook handlers
- separate `main.go` task registration into a helper without changing behavior

Why first:

- high readability gain
- low behavior risk
- good setup for deeper refactors

### Batch 2: explicit dependency injection

- introduce handler dependency structs
- route registration passes typed dependencies into handlers
- stop reading `storage.Stg` and webhook singleton client directly inside handlers

Why second:

- removes hidden coupling
- improves testability ahead of behavior refactors

### Batch 3: split overgrown task files

- extract pure planning logic from `taskApplyRecommendation.go`
- extract stats assembly pipeline from `taskCreateStats.go`
- leave task wrappers thin and schedule-facing

Why third:

- this is where most complexity lives
- should happen after dependency seams are clearer

### Batch 4: database adapter decomposition

- isolate migrations
- isolate row mappers
- split repository responsibilities by aggregate

Why fourth:

- useful, but data-layer changes can affect many flows
- safer after tests improve

## Suggested First Three Refactor PRs

### PR 1

Extract controller runtime assembly and task registration out of `cmd/cruisekube/main.go` without changing runtime behavior.

### PR 2

Introduce explicit dependencies for webhook and API handlers, and remove direct use of `storage.Stg` from one handler slice first (start with webhook handlers).

### PR 3

Refactor `pkg/task/taskApplyRecommendation.go` into:

- task wrapper
- planner
- executor

Keep function signatures stable where possible and add characterization tests around current behavior before moving logic.

## Areas To Avoid Simplifying Too Early

- Helm chart behavior and deployment defaults
- broad type renames across `pkg/types`
- combined logic+behavior rewrites in recommendation math
- database schema changes mixed with code reorganization

These changes carry a larger regression surface and should follow better test coverage.
