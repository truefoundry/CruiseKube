# CruiseKube Code Quality & Reliability Improvement Plan

> **Branch:** `code-quality-plan`
> **Created:** 2026-03-01
> **Scope:** ~13,155 LoC across 75 Go files, 18 packages
> **Status:** 🟡 In Planning

---

## Executive Summary

CruiseKube is a Kubernetes resource optimization controller with a clean interface-driven architecture but significant gaps in test coverage (< 1%), several monolithic files, disabled linters, and a handful of `panic()` calls that should be proper errors. This plan organizes improvements into five priority tiers with concrete tasks, acceptance criteria, and progress tracking.

---

## Baseline Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Test lines / production lines | 111 / 13,155 (0.8%) | ≥ 25% |
| Unit-tested packages | 1 / 18 | ≥ 12 / 18 |
| `panic()` calls | 4 | 0 |
| Disabled linters | 6 | ≤ 2 (justified) |
| Files > 600 lines | 3 | 0 |
| Files 350–600 lines | 5 | ≤ 3 (split or refactored) |
| Package-level godoc | ~0% | 100% of public packages |

---

## Priorities & Phases

```
P1 → Testing (Critical)           ██████████████████  4 tasks
P2 → Refactoring (High)           ███████████████     4 tasks
P3 → Code Quality (Medium)        ████████████        4 tasks
P4 → Documentation (Medium)       ████████            3 tasks
P5 → Observability (Low)          █████               2 tasks
```

---

## Phase 1 — Testing (Critical)

**Goal:** Establish a meaningful test baseline. The near-zero test coverage is the single biggest reliability risk.

### 1.1 Task Package Unit Tests

- **Files:** `pkg/task/*.go`
- **Why:** Contains core business logic (858-line `taskApplyRecommendation.go`, stats generation, metric fetching, disruption). No tests exist.
- **Approach:**
  - Define mock interfaces for `KubeClient`, `DatabasePort`, `PrometheusClient`
  - Test `Run()` methods with table-driven test cases
  - Cover success paths, error paths, and edge cases (empty metrics, missing workloads)
- **Acceptance Criteria:**
  - [ ] `taskApplyRecommendation_test.go` — ≥ 10 test cases
  - [ ] `taskCreateStats_test.go` — ≥ 8 test cases
  - [ ] `taskFetchMetrics_test.go` — ≥ 6 test cases
  - [ ] `taskDisruptionForce_test.go` — ≥ 5 test cases
  - [ ] Coverage for `pkg/task/` ≥ 60%
- **Status:** ⬜ Pending

---

### 1.2 Task Utilities Unit Tests

- **Files:** `pkg/task/utils/*.go`
- **Why:** 2,947 lines of algorithmic utility code with zero tests. Hosts timeseries prediction, node stats builder, pod resource manipulation.
- **Approach:**
  - `util.go`: Test pod resource get/set helpers, GPU detection logic
  - `workload_handler.go`: Table-driven tests for each workload type (Deployment, StatefulSet, DaemonSet, Job)
  - `timeseries_prediction.go`: Property-based tests for prediction bounds
  - `node_stats_builder.go`: Test capacity calculations with mock node data
- **Acceptance Criteria:**
  - [ ] `util_test.go` created
  - [ ] `workload_handler_test.go` created
  - [ ] `timeseries_prediction_test.go` with edge-case inputs
  - [ ] `node_stats_builder_test.go` created
  - [ ] Coverage for `pkg/task/utils/` ≥ 65%
- **Status:** ⬜ Pending

---

### 1.3 HTTP Handler Tests

- **Files:** `pkg/handlers/*.go`
- **Why:** 13 handler files with no tests. Handlers interact with DB and Kubernetes — bugs here are user-visible.
- **Approach:**
  - Use `httptest.NewRecorder()` with Gin's test mode
  - Mock `Manager`, `Database` interfaces
  - Cover happy path, validation errors, and upstream failures for each handler
- **Acceptance Criteria:**
  - [ ] `handlers_test.go` — health, clusters, stats endpoints
  - [ ] `workload_summary_test.go` — summary aggregation
  - [ ] `workload_detail_test.go`
  - [ ] `handleRecommendation_test.go`
  - [ ] `killswitch_test.go`
  - [ ] Coverage for `pkg/handlers/` ≥ 55%
- **Status:** ⬜ Pending

---

### 1.4 Config & OOM Tests

- **Files:** `pkg/config/*.go`, `pkg/oom/*.go`
- **Why:** Config loading errors are silent; OOM observer misses can lead to untracked events.
- **Approach:**
  - Config: Test YAML load, flag override, defaults, missing required fields
  - OOM: Test observer event processing with a fake informer
- **Acceptance Criteria:**
  - [ ] `config_test.go` — load from file, env overrides, validation
  - [ ] `observer_test.go` with a fake pod informer
  - [ ] `processor_test.go` testing event deduplication
  - [ ] Coverage for `pkg/config/` ≥ 70%, `pkg/oom/` ≥ 50%
- **Status:** ⬜ Pending

---

## Phase 2 — Refactoring (High)

**Goal:** Reduce cognitive load. No file should exceed 500 lines without a strong justification.

### 2.1 Split `taskApplyRecommendation.go` (858 lines)

- **File:** `pkg/task/taskApplyRecommendation.go`
- **Why:** Single file handles recommendation fetching, filtering, dry-run mode, patch building, Kubernetes update, rollback logic, and audit recording — multiple responsibilities.
- **Proposed Split:**
  ```
  taskApplyRecommendation.go       ← orchestrator (~150 lines)
  taskApplyRecommendation_fetch.go ← recommendation fetching/filtering
  taskApplyRecommendation_patch.go ← patch building and validation
  taskApplyRecommendation_apply.go ← Kubernetes apply + rollback
  ```
- **Acceptance Criteria:**
  - [ ] No resulting file > 300 lines
  - [ ] All existing behaviour preserved (verify with new tests from 1.1)
  - [ ] No new exported symbols added
- **Status:** ⬜ Pending

---

### 2.2 Split `pkg/task/utils/util.go` (682 lines)

- **File:** `pkg/task/utils/util.go`
- **Why:** Mixed utility grab-bag — pod resource helpers, GPU detection, label manipulation, version parsing. Hard to navigate and reason about.
- **Proposed Split:**
  ```
  pod_resources.go    ← get/set CPU+memory on containers
  gpu_detection.go    ← GPU label/resource detection
  label_utils.go      ← label manipulation helpers
  version_utils.go    ← version parsing/caching
  ```
- **Acceptance Criteria:**
  - [ ] Original `util.go` reduced to ≤ 100 lines (re-exports or removed)
  - [ ] New files are single-responsibility
  - [ ] All existing callers updated via `sed`/refactor tool
- **Status:** ⬜ Pending

---

### 2.3 Split `adapters/database/database.go` (600 lines) & Interface Segregation

- **File:** `pkg/adapters/database/database.go`, `pkg/ports/database.go`
- **Why:** The `Database` interface exposes 20+ methods. Callers only use a subset — this breaks the Interface Segregation Principle and makes mocking costly.
- **Approach:**
  - Audit which methods each caller actually uses
  - Extract smaller interfaces: `WorkloadReader`, `WorkloadWriter`, `OOMWriter`, `RecommendationReader`, `SettingsStore`
  - Keep full `Database` interface as a composition (`Database interface { WorkloadReader; WorkloadWriter; ... }`)
  - Update callers to accept the narrowest interface they need
- **Acceptance Criteria:**
  - [ ] Ports file defines ≥ 4 granular sub-interfaces
  - [ ] At least 3 callers updated to narrower interfaces
  - [ ] No behaviour change (existing tests pass)
- **Status:** ⬜ Pending

---

### 2.4 Deduplicate Handler Patterns

- **Files:** `pkg/handlers/*.go`
- **Why:** Repeated patterns: JSON error response, cluster-from-context extraction, DB-not-found handling. Duplicated in ~8 handler files.
- **Approach:**
  - Extract `respondError(c *gin.Context, status int, err error)` helper
  - Extract `clusterFromContext(c *gin.Context) (string, bool)` helper
  - Consolidate repeated `if err != nil { c.JSON(...); return }` blocks
- **Acceptance Criteria:**
  - [ ] New `pkg/handlers/response_helpers.go` with ≥ 3 helper functions
  - [ ] Callers in ≥ 6 handler files updated to use helpers
  - [ ] No new logic added — strictly extract existing patterns
- **Status:** ⬜ Pending

---

## Phase 3 — Code Quality (Medium)

**Goal:** Eliminate unsafe patterns and restore linter discipline.

### 3.1 Replace `panic()` with Proper Error Handling

- **Files:** `pkg/oom/observer.go:72`, `pkg/adapters/metricsProvider/prometheus/promql.go`, `pkg/config/utils.go`
- **Why:** 4 `panic()` calls in non-init paths create unrecoverable crashes.
- **Changes:**
  - Type assertion panics → use comma-ok idiom + return error
  - Semaphore type assertion in `promql.go` → validate at construction time
  - Config type mismatch → return error from config function
- **Acceptance Criteria:**
  - [ ] Zero `panic()` calls outside `main.go` and test helpers
  - [ ] Each replaced panic is covered by a test case
- **Status:** ⬜ Pending

---

### 3.2 Re-enable Disabled Linters

- **File:** `.golangci.yaml`
- **Currently disabled:** `gosimple`, `noctx`, `promlinter`, `revive`, `gocognit`, `gocyclo`
- **Approach:**
  - Re-enable `revive` with a permissive ruleset first
  - Fix `promlinter` violations (metric naming conventions)
  - Re-enable `gocyclo` with threshold 20 (current code likely fine)
  - Leave `noctx` suppressed only for SQLite files with a targeted `//nolint` comment
  - `gosimple` — investigate if it's a version issue; update or remove
- **Acceptance Criteria:**
  - [ ] `revive` enabled and passing
  - [ ] `promlinter` enabled and passing
  - [ ] `gocyclo` enabled with threshold ≤ 20
  - [ ] `noctx` narrowed to `//nolint:noctx` on affected lines only
  - [ ] `golangci-lint run` exits 0 in CI
- **Status:** ⬜ Pending

---

### 3.3 Fix `promql.go` Complexity

- **File:** `pkg/adapters/metricsProvider/prometheus/promql.go` (755 lines)
- **Why:** Contains complex semaphore management, retry logic, and multiple query strategies inline. High cognitive complexity.
- **Approach:**
  - Extract semaphore management into a `throttler` struct
  - Extract query retry logic into a `withRetry(fn)` helper
  - Extract per-metric-type query builders into separate methods
- **Acceptance Criteria:**
  - [ ] `promql.go` reduced to ≤ 400 lines
  - [ ] New `throttler.go` file in same package
  - [ ] Cyclomatic complexity of all functions ≤ 15
- **Status:** ⬜ Pending

---

### 3.4 Auth Middleware Stub Remediation

- **File:** `pkg/middleware/*.go`
- **Why:** Auth middleware is an empty stub. Leaving it in creates false security expectations and is a reliability risk if a future caller assumes it validates tokens.
- **Approach:**
  - Either implement basic token validation (configurable shared secret or OIDC)
  - Or remove the stub and document that auth is handled at infrastructure layer (ingress/service mesh)
  - Add a clear comment/doc to whichever path is chosen
- **Acceptance Criteria:**
  - [ ] Either: working auth middleware with tests
  - [ ] Or: stub removed + `SECURITY.md` updated to explain external auth boundary
  - [ ] Decision documented in `docs/` ADR
- **Status:** ⬜ Pending

---

## Phase 4 — Documentation (Medium)

**Goal:** Make the codebase navigable for new contributors and operators.

### 4.1 Package-Level Godoc Comments

- **Scope:** All 18 packages under `pkg/`
- **Why:** Zero packages have a `doc.go` or package comment. `go doc ./...` output is empty.
- **Approach:**
  - Add `// Package <name> <one-line description>.` to each package's primary file or a new `doc.go`
  - Focus on *why* the package exists, not *what* the code does
- **Acceptance Criteria:**
  - [ ] 18 / 18 packages have a package-level comment
  - [ ] `go doc ./pkg/...` produces non-empty output for each
- **Status:** ⬜ Pending

---

### 4.2 Algorithm Documentation

- **Files:** `pkg/task/utils/timeseries_prediction.go`, `pkg/task/utils/node_stats_builder.go`, `pkg/task/applystrategies/`
- **Why:** These contain non-obvious mathematical/algorithmic logic with no comments explaining the approach.
- **Approach:**
  - Add a block comment above each algorithm explaining: inputs, outputs, assumptions, and any known limitations
  - Reference external papers/docs if applicable
- **Acceptance Criteria:**
  - [ ] `timeseries_prediction.go` — algorithm comment + parameter explanation
  - [ ] `node_stats_builder.go` — capacity calculation formula documented
  - [ ] `applystrategies/` — each strategy has a doc comment explaining its heuristic
- **Status:** ⬜ Pending

---

### 4.3 Architecture Decision Records (ADRs)

- **Directory:** `docs/adr/`
- **Proposed ADRs:**
  1. `adr-001-task-scheduler-design.md` — Why cron-based vs event-driven
  2. `adr-002-database-choice.md` — SQLite vs PostgreSQL tradeoffs
  3. `adr-003-auth-boundary.md` — Why auth is (or isn't) in-process
  4. `adr-004-prometheus-semaphore.md` — Why bounded concurrency in Prometheus queries
- **Acceptance Criteria:**
  - [ ] `docs/adr/` directory created
  - [ ] ≥ 3 ADRs written in standard format (Context / Decision / Consequences)
- **Status:** ⬜ Pending

---

## Phase 5 — Observability (Low)

**Goal:** Make the task execution pipeline easier to debug in production.

### 5.1 OpenTelemetry Tracing for Task Execution

- **Files:** `pkg/task/*.go`, `pkg/cluster/scheduler.go`
- **Why:** Telemetry package exists but task execution has no spans. Root-cause analysis for slow or failing tasks is blind.
- **Approach:**
  - Wrap each `Task.Run()` in a span: `tracer.Start(ctx, "task.<name>")`
  - Add span attributes: workload name, namespace, cluster
  - Record errors on spans
- **Acceptance Criteria:**
  - [ ] Each task implementation creates a root span
  - [ ] Span attributes include workload identity
  - [ ] Errors are recorded on spans, not just logged
- **Status:** ⬜ Pending

---

### 5.2 Latency Histograms for Key Operations

- **Files:** `pkg/metrics/`, `pkg/adapters/database/`, `pkg/adapters/metricsProvider/`
- **Why:** No latency metrics for DB queries or Prometheus queries. Hard to detect degradation.
- **Approach:**
  - Add `prometheus.HistogramVec` for DB operation duration
  - Add histogram for Prometheus query duration
  - Instrument task total execution time
- **Acceptance Criteria:**
  - [ ] `db_operation_duration_seconds` histogram exposed
  - [ ] `prometheus_query_duration_seconds` histogram exposed
  - [ ] `task_run_duration_seconds` histogram exposed with `task_type` label
- **Status:** ⬜ Pending

---

## Progress Tracker

### By Phase

| Phase | Tasks | Done | In Progress | Pending |
|-------|-------|------|-------------|---------|
| P1 Testing | 4 | 0 | 0 | 4 |
| P2 Refactoring | 4 | 0 | 0 | 4 |
| P3 Code Quality | 4 | 0 | 0 | 4 |
| P4 Documentation | 3 | 0 | 0 | 3 |
| P5 Observability | 2 | 0 | 0 | 2 |
| **Total** | **17** | **0** | **0** | **17** |

### By File / Area

| File / Area | Lines | Phase | Status |
|-------------|-------|-------|--------|
| `pkg/task/taskApplyRecommendation.go` | 858 | P1 + P2 | ⬜ |
| `pkg/task/utils/util.go` | 682 | P2 | ⬜ |
| `pkg/adapters/metricsProvider/prometheus/promql.go` | 755 | P3 | ⬜ |
| `pkg/adapters/database/database.go` | 600 | P2 | ⬜ |
| `pkg/task/taskCreateStats.go` | 460 | P1 | ⬜ |
| `pkg/handlers/controller_webhook_handler.go` | 427 | P1 | ⬜ |
| `pkg/task/utils/workload_handler.go` | 676 | P1 + P2 | ⬜ |
| `pkg/oom/observer.go` | 227 | P3 | ⬜ |
| `pkg/config/utils.go` | — | P3 | ⬜ |
| `pkg/middleware/` | 111 | P3 | ⬜ |

---

## Implementation Notes

### Recommended Execution Order

1. **Start with P3.1** (panic removal) — small, high-impact, verifiable immediately
2. **Then P1.2** (task utils tests) — builds mock/test infrastructure others reuse
3. **Then P1.1** (task tests) — uses mocks from 1.2
4. **Then P2.1** (split taskApplyRecommendation) — safe once 1.1 tests exist
5. **Continue P1.3, P1.4** in parallel with P2.2–2.4
6. **P3.2** (linters) after refactoring settles
7. **P4, P5** can be worked on any time independently

### Mock Strategy

Prefer interface-based mocks using `github.com/stretchr/testify/mock` or hand-written fakes. Do **not** use `gomock` (adds a code generation dependency). Key interfaces to mock:

- `ports.Database` → `pkg/adapters/database/mock_database.go`
- `cluster.Manager` → `pkg/cluster/mock_manager.go`
- Kubernetes `client.Interface` → use `k8s.io/client-go/kubernetes/fake`
- Prometheus client → wrap behind an interface in `pkg/adapters/metricsProvider/`

### Definition of Done (Per Task)

- [ ] Code change merged to branch
- [ ] `go build ./...` passes
- [ ] `golangci-lint run` passes (with current enabled linters)
- [ ] New tests pass: `go test ./... -race`
- [ ] Progress table above updated
- [ ] PR linked in this document

---

## Linked PRs / Commits

<!-- Add PR links here as work progresses -->

| Task | PR / Commit | Date | Notes |
|------|-------------|------|-------|
| — | — | — | — |

---

## Open Questions

1. **Auth middleware:** Implement in-process or document external boundary? Needs owner decision before P3.4.
2. **Test framework:** `testify` only, or add `gomock`? Recommend testify + hand-written fakes.
3. **ADR ownership:** Who approves architectural decisions (P4.3)?
4. **Coverage threshold:** Is 60% a realistic short-term target given the domain complexity?

---

*Last updated: 2026-03-01 — initial plan creation*
