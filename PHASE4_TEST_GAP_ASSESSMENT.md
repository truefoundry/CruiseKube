# Phase 4 Test Gap Assessment

## Scope

This pass evaluates the current validation and test safety net across:

- backend unit and package tests
- backend CI coverage
- e2e coverage
- frontend validation hooks
- practical gaps that will matter before refactoring

This assessment is based on repository structure and workflow definitions. I did not run tests.

## Current Test Surface

### Backend unit tests

In the main repository tree, there is only one Go test file:

- `pkg/adapters/database/database_test.go`

That test covers a narrow slice of the SQLite-backed database adapter:

- basic stat upsert/read/delete
- recommendation row save validation

It does not cover:

- controller startup
- config validation
- handlers
- webhook behavior
- task behavior
- scheduler behavior
- OOM processing
- middleware behavior
- Prometheus adapter behavior

There is also a second test file under:

- `.claude/worktrees/code-quality-plan/pkg/adapters/database/database_test.go`

That appears to be an auxiliary tooling/worktree artifact, not meaningful production coverage.

### Backend CI

`[.github/workflows/lint-and-test.yaml](/Users/shubham.rai/workspace/truefoundry/cruiseKube/.github/workflows/lint-and-test.yaml)` provides three backend checks:

- `golangci-lint`
- `go build -v ./...`
- `make test` (which resolves to `go test ./...`)

This is useful as a baseline, but because the repo has almost no unit tests, the current `test` job mostly verifies compilation and that the database adapter test passes.

### E2E coverage

`[.github/workflows/e2e-kuttl-test.yaml](/Users/shubham.rai/workspace/truefoundry/cruiseKube/.github/workflows/e2e-kuttl-test.yaml)` runs KUTTL-based e2e tests in Kind.

The current e2e suites cover three broad flows:

- `01-apply-recommendations`
- `02-webhook`
- `03-oom-handling`

This is valuable because it exercises integration behavior that unit tests currently miss.

However:

- e2e tests are expensive and slow
- they are not a substitute for fast refactor safety tests
- they are too coarse to isolate regressions in config, task orchestration, or handler logic

### Frontend validation

The frontend submodule has local scripts for:

- `npm run lint`
- `npm run build`

But in the main repo workflows scanned here, frontend checks are not clearly part of the regular lint/test gating path. The visible workflow references to `cruiseKube-frontend` are image/release related, not source validation.

That means frontend breakage may be caught late unless the frontend submodule has its own independent CI.

## Coverage Quality Assessment

### What is covered reasonably today

- basic Go compilation of backend code
- basic linting
- one database adapter path
- high-level integration behavior for a few end-to-end cluster scenarios

### What is weak or missing

- startup and config behavior
- failure-mode testing
- handler behavior in isolation
- webhook patch logic in isolation
- task policy logic in isolation
- scheduler semantics
- OOM processing logic in isolation
- repository contract coverage beyond one adapter test
- frontend source validation in the main repo’s normal backend-oriented CI path

## High-Risk Untested Areas

### 1. Config loading and validation

Why it matters:

- current analysis already found correctness gaps in config validation
- startup failure paths are config-driven

What is missing:

- tests for invalid `executionMode`
- tests for invalid `controllerMode`
- tests for missing task config entries
- tests for env/file/default precedence

### 2. Runtime bootstrap in `cmd/cruisekube/main.go`

Why it matters:

- this file is the composition root and a major risk hotspot

What is missing:

- tests for execution-mode branching
- tests for server startup error propagation
- tests for controller vs webhook lifecycle behavior
- tests for dependency initialization ordering

### 3. Webhook mutation logic

Why it matters:

- it affects live pod admission behavior
- it mixes transport, decision logic, and patch generation

What is missing:

- tests for no-op mutation cases
- tests for malformed admission payloads
- tests for patch generation with/without stats
- tests for memory-disabled and annotation-based skip paths

### 4. Task policy logic

Why it matters:

- most of the business logic lives in tasks
- the biggest files are in `pkg/task`

What is missing:

- characterization tests for recommendation eligibility
- tests for dry-run vs apply behavior
- tests for stale-workload cleanup behavior
- tests for prediction fallback paths
- tests for disruption-force decision logic

### 5. Scheduler and background execution behavior

Why it matters:

- current lifecycle is tightly coupled to scheduling and goroutines

What is missing:

- tests for duplicate task registration handling
- tests for invalid schedule parsing
- tests that overlapping runs are skipped
- tests for stop/shutdown behavior

### 6. OOM observer and processor logic

Why it matters:

- it directly affects eviction behavior after OOM events

What is missing:

- tests for cooldown behavior
- tests for disabled memory-application behavior
- tests for override-based eviction skip behavior
- tests for event parsing and watcher retry behavior

## Test Gap Matrix

### Fast unit-test candidates (highest ROI)

- `pkg/config`
  - validate config permutations
  - validate task-map completeness
- `pkg/cluster`
  - scheduler behavior
  - task registration semantics
- extracted handler/service logic
  - admission decisions
  - no-op vs mutate conditions
- extracted task domain logic
  - recommendation selection
  - stats assembly rules
  - disruption window evaluation

### Integration-test candidates (medium cost, high confidence)

- repository contract tests across sqlite and postgres-backed adapters
- API route tests using Gin test server with fake dependencies
- webhook route tests with representative AdmissionReview payloads

### E2E candidates (keep small and strategic)

- one happy-path recommendation apply flow
- one webhook mutation flow
- one OOM flow

The current e2e structure already maps to these. The goal should be to preserve them while reducing the amount of logic that only e2e tests can verify.

## Recommended Minimum Safety Net Before Refactoring

### Step 1: Add startup/config characterization tests

Add fast tests for:

- invalid controller mode
- invalid execution mode
- missing task config keys
- webhook-required config checks

Reason:

- these directly cover critical startup failure risks already identified

### Step 2: Add scheduler tests

Add tests for:

- schedule parse failure
- duplicate task name handling
- overlapping run suppression

Reason:

- scheduler behavior is central and relatively isolated

### Step 3: Add webhook handler/service tests

Start with:

- malformed payload
- non-Pod request
- missing stats
- no-op recommendation skip
- successful patch generation for a small representative case

Reason:

- webhook behavior is business-critical and easier to isolate than the full task stack

### Step 4: Add task-level characterization around the first refactor target

Before splitting `taskApplyRecommendation.go`, add tests around:

- dry-run mode
- write-unauthorized cluster skip
- version-gated skip paths

Reason:

- this gives a safe seam for the largest refactor target

## CI Improvement Recommendations

### Backend

- keep lint/build/test as separate jobs
- add coverage reporting once a meaningful unit-test base exists
- add focused package tests for changed files where possible

### Frontend

- if the frontend is owned here operationally, add a dedicated workflow or ensure the submodule has enforced CI
- at minimum, document whether frontend validation is expected in this repo or only in the submodule repo

### E2E

- keep e2e as a slower integration gate
- avoid relying on e2e for first detection of simple logic regressions

## Practical Refactor Testing Order

1. Add tests to `pkg/config` and `pkg/cluster/scheduler` first.
2. Add isolated tests for webhook decision and patch logic before handler decomposition.
3. Add characterization tests around `taskApplyRecommendation` before file splitting.
4. Add repository contract expansion after database adapter decomposition starts.

## Residual Risks

- Without immediate new unit tests, even “mechanical” refactors in `main.go`, `pkg/task`, and `pkg/handlers` will still carry meaningful regression risk.
- Existing e2e coverage is too slow and too coarse to protect incremental structural cleanup by itself.
