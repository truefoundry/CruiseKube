# Forthcoming Plan

## Purpose

This file is the single planning document for the remaining `cruisekube` simplification and reliability work.

The earlier analysis passes and the first batch of runtime/config/handler improvements have already landed. What remains is the follow-through work needed to finish the cleanup without widening regression risk.

## Current State

The following backlog items are effectively complete:

- strict config validation and task-config guardrails
- startup extraction from `cmd/cruisekube/main.go`
- runtime lifecycle manager and signal-aware shutdown
- scheduler isolation and scheduler tests
- handler dependency injection and route wiring cleanup

The following areas are still incomplete:

- webhook cleanup is only partially finished
- controller runtime still assigns legacy global singletons during transition
- task decomposition has not happened in a meaningful way
- task boilerplate is still repetitive
- data-layer cleanup has not been started as a distinct stream

## Forthcoming Work

### 1. Finish Handler And Webhook Cleanup

Primary goal:

- complete the move away from transitional global state and finish the service split around webhook behavior

Scope:

- remove the remaining `storage.Stg` and `audit.Recorder` assignments from runtime setup
- keep all handler behavior flowing through `HandlerDependencies`
- finish separating webhook transport concerns from mutation decision logic and patch-building helpers
- reduce the size and responsibility spread of `pkg/handlers/controller_webhook_handler.go`

Exit criteria:

- controller runtime no longer needs global singleton assignment for handlers
- webhook paths are testable without implicit process-wide state
- webhook files are smaller and responsibilities are clearer

### 2. Consolidate Task Boilerplate

Primary goal:

- reduce repeated constructor and metadata-parsing patterns before deeper task splits

Scope:

- introduce shared helpers for task metadata decoding
- reduce repeated `GetName()`, `GetSchedule()`, `IsEnabled()`, and related wrapper code
- clean up task config naming consistency where it has drifted over time
- make task registration easier to scan and safer to extend

Exit criteria:

- new task implementations require materially less boilerplate
- task construction errors fail in one consistent way
- controller task registration is easier to review

### 3. Decompose Task Logic By Responsibility

Primary goal:

- split the largest task files into clearer policy, orchestration, and side-effect layers

Priority targets:

- `pkg/task/taskApplyRecommendation.go`
- `pkg/task/taskCreateStats.go`
- `pkg/task/taskFetchMetrics.go`
- `pkg/task/utils/util.go`
- `pkg/task/utils/workload_handler.go`

Scope:

- separate runtime task wrappers from domain decision logic
- isolate Kubernetes mutation helpers from recommendation policy
- isolate Prometheus query logic from stats assembly logic
- reduce the catch-all nature of `pkg/task/utils`

Exit criteria:

- the largest task files shrink substantially
- pure decision logic can be unit tested without live adapters
- policy code and side-effect code are no longer interleaved throughout the same functions

### 4. Expand Test Coverage Around Refactor Seams

Primary goal:

- add fast tests where remaining refactors will otherwise be risky

Priority additions:

- more webhook characterization tests
- controller runtime tests around startup assembly and shutdown behavior
- task-domain tests for recommendation eligibility, stats assembly, and disruption behavior
- repository contract tests beyond the current narrow adapter coverage

Exit criteria:

- remaining high-risk refactors are protected by focused tests
- new behavior checks rely less on slow end-to-end coverage

### 5. Clean Up Data-Layer Boundaries

Primary goal:

- simplify persistence boundaries after the task and handler layers are less coupled

Scope:

- break up large database adapter responsibilities
- reduce JSON mapping duplication in persistence code
- tighten interfaces between storage, ports, and domain types
- keep persistence concerns separate from transport and orchestration code

Exit criteria:

- data access code is easier to test and reason about
- domain types are less shaped by storage implementation details

## Recommended Execution Order

1. Finish handler/webhook cleanup and remove the remaining transitional globals.
2. Consolidate task boilerplate to create cleaner seams.
3. Split `taskApplyRecommendation` first, then `taskCreateStats`, then `taskFetchMetrics`.
4. Add tests alongside each extracted seam, not after the full rewrite.
5. Tackle data-layer cleanup only after task and handler boundaries are more stable.

## Working Rules

- prefer behavior-preserving extractions before logic changes
- keep each PR independently reviewable and revertible
- avoid mixing deployment/chart changes with internal refactors
- add tests at the seam being changed
- keep runtime lifecycle ownership centralized

## Definition Of Done

This plan is complete when:

- handler paths no longer depend on transitional globals
- webhook logic is clearly split and covered by tests
- task hotspots are materially smaller and separated by responsibility
- remaining refactors have direct unit or integration coverage
- persistence code has clearer boundaries and lower duplication
