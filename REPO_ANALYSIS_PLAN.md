# Repository Analysis And Simplification Plan

## Goal

Reduce code complexity and improve reliability in `cruisekube` by:

- mapping the current architecture and ownership boundaries
- identifying high-risk coupling, dead code, and duplication
- tightening tests around critical behavior
- simplifying interfaces, entrypoints, and configuration flow
- defining a sequenced refactor backlog with low regression risk

## Current Signals From Initial Repo Scan

- The main application entrypoint in `cmd/cruisekube/main.go` wires many concerns directly: config, database, telemetry, cluster clients, task orchestration, webhook, and HTTP server.
- The backend is primarily Go under `pkg/`, with broad packages such as `handlers`, `task`, `cluster`, `config`, `adapters`, and `types`.
- The frontend lives in `cruiseKube-frontend/` as a separate React/Vite app with its own lockfiles, `node_modules`, build output, and an embedded `.git` directory.
- The top-level `Makefile` provides only a basic `go test ./...` target for backend validation.
- Test coverage appears sparse from the initial scan: only `pkg/adapters/database/database_test.go` was visible in the backend tree.
- The repo also carries operational surface area in Helm charts, docs, and e2e manifests that should be protected from accidental behavioral regressions while simplifying code.

## Working Assumptions

- This plan is for analysis and backlog creation first, not immediate broad refactors.
- Existing untracked files in the main worktree are user-owned and should remain untouched.
- The frontend may be intentionally managed as a nested repository or imported subtree, so that relationship should be confirmed before structural changes.

## Phase 1: Baseline The Codebase

### Objectives

- Build a clear inventory of modules, entrypoints, and cross-package dependencies.
- Establish what is critical-path code versus peripheral tooling.

### Actions

- Generate a package map for `cmd/` and `pkg/`, grouped by responsibility.
- Trace startup flow from `cmd/cruisekube/main.go` into:
  - config loading
  - controller setup
  - webhook setup
  - server startup
  - background task scheduling
- Identify global state and singleton-style patterns, especially in logging, storage, audit, metrics, and task execution.
- Catalog all externally visible interfaces:
  - HTTP handlers
  - webhook handlers
  - database ports
  - cluster adapters
  - Prometheus adapters
- Document configuration sources and precedence:
  - config files
  - environment variables
  - CLI flags
  - defaults

### Deliverables

- Architecture map
- Dependency hot-spot list
- Config-flow summary

## Phase 2: Reliability Risk Assessment

### Objectives

- Find the areas most likely to cause outages, silent corruption, or hard-to-debug behavior.

### Actions

- Review error handling patterns across `pkg/handlers`, `pkg/task`, and `pkg/adapters`.
- Identify `log.Fatal` or process-exit paths that make behavior hard to test or recover from.
- Inspect concurrency paths:
  - scheduled/background tasks
  - shared caches
  - global repositories or clients
  - webhook/controller running in the same process
- Check context propagation and cancellation handling in long-running routines.
- Review database interaction boundaries for transaction safety, nil handling, and partial-write failure behavior.
- Audit assumptions around Kubernetes and Prometheus client failures, retries, and timeouts.
- Compare production behavior against existing e2e coverage to find untested critical flows.

### Deliverables

- Ranked reliability risk register
- List of panic/fatal/implicit-exit paths
- Gaps between critical paths and test coverage

## Phase 3: Complexity And Simplification Analysis

### Objectives

- Reduce unnecessary coupling and make the code easier to reason about and change safely.

### Actions

- Measure package size and file size hotspots, prioritizing:
  - `pkg/handlers`
  - `pkg/task`
  - `cmd/cruisekube/main.go`
- Identify mixed-responsibility files that combine orchestration, validation, data transformation, and side effects.
- Find duplicate logic in:
  - config parsing and validation
  - recommendation processing
  - workload and node metrics handling
  - HTTP response shaping
- Review `pkg/types` for overly broad or leaky shared models.
- Evaluate whether some packages should be split into:
  - domain logic
  - adapters/integrations
  - transport layer
  - orchestration layer
- Decide whether the frontend should remain colocated, be treated as an external deliverable, or be isolated more explicitly in tooling.

### Deliverables

- Simplification candidates by package
- Proposed package boundary revisions
- Dead-code and duplication shortlist

## Phase 4: Testing Strategy Upgrade

### Objectives

- Raise confidence before refactoring by improving coverage where failures would be expensive.

### Actions

- Establish a baseline test inventory:
  - unit tests
  - integration tests
  - e2e tests
  - frontend lint/build checks
- Add focused characterization tests before changing behavior in high-risk packages.
- Prioritize tests for:
  - config parsing and validation
  - startup mode selection
  - recommendation application logic
  - webhook mutation/validation behavior
  - storage/repository behavior beyond the current sqlite test
- Define fast local verification commands that should pass before each refactor batch.
- Separate pure logic from side effects so unit tests can avoid requiring Kubernetes, Prometheus, or real databases.

### Deliverables

- Test-gap matrix
- Minimum refactor safety net
- Recommended CI gating order

## Phase 5: Refactor Backlog And Execution Order

### Objectives

- Convert analysis into a safe, incremental sequence of changes.

### Proposed Refactor Order

1. Extract startup wiring from `cmd/cruisekube/main.go` into smaller composition functions or packages.
2. Remove or reduce package-level global state where it hides dependencies.
3. Tighten configuration parsing and validation into one explicit boundary.
4. Split large handler and task files by responsibility.
5. Introduce clearer interfaces around cluster, metrics, and storage integrations.
6. Expand tests around each extracted seam before deeper internal rewrites.
7. Reconcile backend/frontend repository boundaries and tooling expectations.

### Refactor Rules

- Prefer behavior-preserving changes first.
- Land mechanical moves separately from logical changes.
- Add tests before modifying unstable or heavily coupled flows.
- Keep deployment and Helm contract changes isolated from internal code cleanup.

## Recommended Analysis Checklist

- [ ] Map package responsibilities and ownership boundaries
- [ ] Trace startup path and identify dependency injection gaps
- [ ] Inventory all global variables and singleton-style state
- [ ] Rank files by size, churn, and dependency fan-in/fan-out
- [ ] Review error handling and timeout behavior in integrations
- [ ] Measure current backend test coverage
- [ ] Review frontend build/lint/test posture
- [ ] Confirm nested frontend repository intent
- [ ] Propose package boundary changes
- [ ] Define first three low-risk refactor PRs

## Suggested Commands For The Analysis Pass

```bash
go test ./...
go test ./... -cover
rg -n "var .*=" cmd pkg
rg -n "log\\.Fatal|Fatalf|panic\\(" cmd pkg
rg -n "go func|cron|ticker|timer" cmd pkg
rg -n --glob '*_test.go' '.' cmd pkg cruiseKube-frontend
```

For the frontend:

```bash
cd cruiseKube-frontend
npm run lint
npm run build
```

## Tracking Log

### Status

- [x] Initial repository scan completed
- [x] Detailed architecture mapping
- [x] Reliability risk review
- [x] Simplification candidate review
- [x] Test-gap assessment
- [x] Refactor backlog definition

### Notes

- Create follow-up docs in this worktree if the analysis is split into multiple passes.
- Keep findings tied to specific files and packages so the eventual refactor plan can be executed in small PRs.
