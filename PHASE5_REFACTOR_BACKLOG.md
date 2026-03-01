# Phase 5 Refactor Backlog And Execution Order

## Goal

Turn the analysis into a practical, low-regression refactor sequence that:

- reduces startup and runtime fragility first
- adds test seams before deep structural rewrites
- keeps each batch small enough for review and rollback

This backlog is intentionally ordered by dependency and risk, not just by code ownership.

## Execution Principles

- Prefer behavior-preserving extractions before behavioral fixes.
- Add tests at the seam you are about to refactor, not after the entire rewrite.
- Keep deployment/chart changes separate from internal code cleanup.
- Do not combine package-boundary changes and business-rule changes in one PR unless unavoidable.
- Keep each PR independently revertible.

## Refactor Streams

### Stream A: Startup And Runtime Control

Primary targets:

- `cmd/cruisekube/main.go`
- `pkg/config`
- `pkg/cluster`
- `pkg/logging`

Purpose:

- fix the highest-risk startup and lifecycle issues
- establish explicit runtime ownership

### Stream B: Dependency Injection And Handler Cleanup

Primary targets:

- `pkg/handlers`
- `pkg/server`
- `pkg/middleware`
- `pkg/repository/storage`
- `pkg/audit`

Purpose:

- remove hidden dependencies
- make route logic testable without package globals

### Stream C: Task Decomposition

Primary targets:

- `pkg/task`
- `pkg/task/utils`
- `pkg/oom`

Purpose:

- break up the highest-complexity business logic
- separate policy from side effects

### Stream D: Data Layer Cleanup

Primary targets:

- `pkg/adapters/database`
- `pkg/ports`
- `pkg/types`

Purpose:

- simplify persistence boundaries
- reduce domain leakage across transport/storage layers

## Recommended PR Sequence

### PR 1: Strict Config Validation And Task Config Guardrails

Scope:

- make `ValidateControllerExecutionMode()` return an error for invalid values
- validate required task config keys before controller startup
- normalize validation to return errors instead of printing/logging directly

Why first:

- closes two critical startup crash paths immediately
- low behavioral ambiguity
- improves diagnostics before deeper changes

Suggested tests:

- invalid `controllerMode`
- invalid `executionMode`
- missing task config entries
- missing webhook-required values

Acceptance criteria:

- invalid config always fails before runtime startup
- no startup panics due to missing task config

### PR 2: Extract Startup Assembly From `main.go`

Scope:

- extract config/bootstrap helpers
- extract controller runtime assembly
- extract webhook runtime assembly
- extract task registration helper

Why second:

- reduces the largest orchestration hotspot early
- creates clearer seams for later lifecycle fixes

Suggested tests:

- execution-mode branching
- startup helper behavior with fake dependencies where practical

Acceptance criteria:

- no functional behavior change intended
- `main.go` becomes a thin composition entrypoint

### PR 3: Introduce Runtime Lifecycle Manager

Scope:

- replace `blockForever()` and scheduler-owned blocking with one top-level runtime wait path
- introduce signal-aware root context
- stop using `Fatalf` inside server goroutines
- coordinate listener errors through one top-level error path

Why third:

- addresses the most serious lifecycle reliability issues
- easier after startup code is already decomposed

Suggested tests:

- server startup errors propagate to top-level manager
- shutdown path invokes cleanup hooks

Acceptance criteria:

- process liveness is owned in one place
- graceful shutdown path exists for long-lived resources

### PR 4: Scheduler Isolation And Tests

Scope:

- keep scheduler focused on scheduling only
- remove process-blocking responsibility from `ScheduleAllTasks()`
- add scheduler tests

Why fourth:

- isolates a core runtime primitive
- lowers coupling before task refactors

Suggested tests:

- duplicate task registration
- invalid duration parsing
- overlapping execution suppression
- stop behavior

Acceptance criteria:

- scheduler no longer owns process liveness
- scheduler semantics are covered by unit tests

### PR 5: Handler Dependency Container

Scope:

- introduce explicit handler dependencies (storage, audit, cluster manager, config, webhook client)
- stop reading package globals directly in one handler slice first
- start with webhook and core API handlers

Why fifth:

- removes a major source of hidden coupling
- unlocks isolated handler tests

Suggested tests:

- webhook malformed payload
- non-Pod admission request
- no-op response path
- config/context dependency failure path

Acceptance criteria:

- selected handlers do not depend on `storage.Stg` or singleton webhook client
- route registration passes dependencies explicitly

### PR 6: Webhook Logic Split

Scope:

- split `controller_webhook_handler.go` into:
  - transport handler
  - mutation decision service
  - patch builder helpers
  - response helper

Why sixth:

- webhook behavior is operationally critical and currently overloaded
- dependency injection from PR 5 provides the seam

Suggested tests:

- representative patch generation
- skip conditions
- memory-disabled path

Acceptance criteria:

- webhook transport code is thin
- patch logic is testable outside Gin

### PR 7: Task Boilerplate Consolidation

Scope:

- introduce shared metadata decoding helper
- reduce repeated trivial task methods
- reassess `GetCoreTask() any`

Why seventh:

- low-risk cleanup that reduces repetition before larger task file splits

Suggested tests:

- metadata decoding helper behavior
- existing tasks still satisfy the task interface

Acceptance criteria:

- repeated constructor/method boilerplate is materially reduced

### PR 8: Split `taskApplyRecommendation`

Scope:

- extract:
  - task wrapper
  - recommendation planner
  - recommendation executor
  - snapshot writer
  - reporting/audit adapter

Why eighth:

- this is the single highest-complexity backend file
- prior PRs create the safety net and seams needed

Suggested tests:

- dry-run behavior
- write-unauthorized cluster skip
- version-gated behavior
- representative recommendation application flow with fakes

Acceptance criteria:

- top-level task file becomes substantially smaller
- planning logic is testable without live Kubernetes calls

### PR 9: Split `taskCreateStats`

Scope:

- extract:
  - workload inventory loader
  - stats collection orchestrator
  - prediction pipeline
  - stats assembler
  - persistence boundary

Why ninth:

- second major task hotspot
- benefits from the same patterns proven in PR 8

Suggested tests:

- stale workload deletion behavior
- recent-stat filtering
- skip-memory behavior
- prediction fallback behavior

Acceptance criteria:

- task file size and responsibility count are materially reduced

### PR 10: `pkg/task/utils` Package Boundary Cleanup

Scope:

- split broad utility buckets into coherent subdomains
- separate generic helpers from workload/recommendation/stats logic

Why tenth:

- this should happen after the major task extractions reveal stable boundaries

Suggested tests:

- move existing and new tests with the extracted logic

Acceptance criteria:

- fewer “miscellaneous” utilities
- clearer domain ownership per package

### PR 11: Database Adapter Decomposition

Scope:

- separate migrations from CRUD logic
- split repository logic by aggregate
- extract row/domain mapping helpers

Why eleventh:

- valuable, but touches many flows and should follow better coverage

Suggested tests:

- expanded repository contract tests
- serializer/deserializer mapping tests

Acceptance criteria:

- `database.go` becomes a coordinator instead of a monolith

### PR 12: Type Boundary Hardening

Scope:

- separate transport DTOs from storage/domain models where practical
- reduce overuse of shared cross-layer structs

Why twelfth:

- useful long term, but cross-cutting and easy to over-expand
- safest after handlers, tasks, and repositories are cleaner

Suggested tests:

- serialization compatibility tests
- route contract tests for affected responses

Acceptance criteria:

- domain and transport boundaries are clearer without broad contract regressions

## Immediate Backlog (Next 2-4 Weeks)

Recommended near-term execution:

1. PR 1: strict config validation and task config guardrails
2. PR 2: startup assembly extraction
3. PR 3: runtime lifecycle manager
4. PR 4: scheduler isolation and tests
5. PR 5: handler dependency container

This sequence gives the best ratio of risk reduction to code churn.

## Deferrable Backlog (After Basic Safety Net Improves)

- full task decomposition beyond the first two hotspot files
- database adapter decomposition
- broad type-boundary cleanup
- frontend repo-boundary/tooling changes

These are still important, but they should follow the first reliability and testability upgrades.

## Suggested Ownership Model

- Startup/runtime changes: one focused backend maintainer
- Handler dependency cleanup: one backend maintainer with API familiarity
- Task decomposition: one maintainer at a time per task hotspot to avoid merge conflicts
- Data layer cleanup: one maintainer after repository test expansion

This repo has several large files that are conflict-prone. Parallel deep refactors across the same areas will create avoidable churn.

## Stop Conditions

Pause and re-scope if any of the following occur:

- e2e failures increase without clear unit-test coverage to localize regressions
- a refactor PR needs Helm/chart changes to ship
- a “mechanical” extraction starts changing recommendation behavior
- package moves begin forcing broad `pkg/types` churn across unrelated areas

## Deliverable After This Backlog

The next practical step is not another analysis doc. It is to begin execution with:

1. a small test-first PR for config validation
2. then a behavior-preserving extraction of startup assembly

That order reduces real production risk before deeper structural work begins.
