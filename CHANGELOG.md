# Changelog

All notable changes to CruiseKube will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

All the unreleased changes are listed under `Unreleased` section. Add your changes here, they will be moved to the next release.

## Unreleased

## v0.3.4 (08-07-2026)

* feat: add a preflight checks to verify if the installation is done correctly by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/273
* feat: update frontend and add diagnostics script by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/274
* feat: update config for standalone prometheus by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/270
* docs: update standalone prom values by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/271
* fix: properly return database errors in HasCluster and HasWorkloadForCluster by @SuvigyaSrivastava in https://github.com/truefoundry/CruiseKube/pull/268
* build(deps): bump golang.org/x/net from 0.52.0 to 0.55.0 by @dependabot in https://github.com/truefoundry/CruiseKube/pull/272

## v0.3.3 (04-06-2026)

* feat: bundle prometheus with cruisekube by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/260
* feat: add book a call to hero section by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/265

## v0.3.2 (02-06-2026)

* feat: add source-aware Sentry fingerprinting for better error grouping by @vorflux[bot] in https://github.com/truefoundry/CruiseKube/pull/255
* update landing page by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/256
* fix: troubleshoot, screenshot, gaps by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/258
* feat: update kubectl chart by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/261
* fix: replace base kubectl docker image by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/263

## v0.3.1 (18-05-2026)

* feat: make basic authentication optional via admin.enabled flag by @vorflux[bot] in https://github.com/truefoundry/CruiseKube/pull/246
* Update logo and update frontend. by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/247
* fix: filter failed pods from savings replica count by @vorflux[bot] in https://github.com/truefoundry/CruiseKube/pull/249
* fix: normalize day-of-week 7 to 0 for Sunday in disruption window cron parsing by @vorflux[bot] in https://github.com/truefoundry/CruiseKube/pull/250
* fix sentry issues by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/240


## v0.3.0 (11-05-2026)

Same release as v0.3.0-rc.1

## v0.3.0-rc.1 (11-05-2026)

### Breaking changes

* Add authentication layer to all the APIs & expose login API by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/227


### Bug Fixes

* fix: add missing values to original requested by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/236
* Fix duplicate pod owner joins in createStats by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/216

### Improvements

* Show pod average resources in workload by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/230
* pre-fetching all pods before matching pod to workloads by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/239
* feat: add usage telemetry to cruisekube by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/238
* reusing already fetched workload object to make create stats faster by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/237
* Fetch workload overrides from database instead of API in apply recommendation task by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/226
* Add imagePullSecrets parameter for certificate generator by @koundykarthik in https://github.com/truefoundry/CruiseKube/pull/222

### Documentation

* Add [Login & authentication](https://cruisekube.com/documentation/operate/authentication/) guide (Helm admin Secret, generated password, retrieval, rotation).
* Move docs to zensical by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/215
* feat: update gh pages by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/221

### Other

* Add a issue type for adopters by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/233
* hotfix: update minor bug by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/234
* release v0.2.6-rc.2 by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/223
* release: v0.2.6-rc.1 by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/217
* Standardize Go file and package naming by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/219
* increasing timeout for timeseries prediction by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/220
* build(deps): bump github.com/jackc/pgx/v5 from 5.9.0 to 5.9.2 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/231
* build(deps): bump go.opentelemetry.io/otel/sdk from 1.40.0 to 1.43.0 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/225
* build(deps): bump go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp from 1.38.0 to 1.43.0 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/224
* build(deps): bump github.com/jackc/pgx/v5 from 5.6.0 to 5.9.0 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/229

### New Contributors

* @koundykarthik made their first contribution in https://github.com/truefoundry/CruiseKube/pull/222


## v0.2.6-rc.2 [2026-04-08]

### Improvements
* Standardize Go file and package naming by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/219
* increasing timeout for timeseries prediction by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/220
* Move docs to zensical by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/215
* feat: update gh pages by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/221
* Add imagePullSecrets parameter for certificate generator by @koundykarthik in https://github.com/truefoundry/CruiseKube/pull/222


## v0.2.6-rc.1 (2026-04-02)

### Other

* Initialize the `v0.2.6` release candidate line. No product changes are included beyond the version bump from `v0.2.5`.

## v0.2.5 (2026-03-31)

### Breaking Changes

* Allow raw namespaceSelector input for webhook chart by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/204
* Remove obsolete write/apply config gates by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/195
* Remove dry-run support by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/196

### Improvements

* simplifying node_stats_builder.go by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/197
* Remove unused create-stats max metrics by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/198
* integrate deadcode linter by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/199
* Persist workloads with incomplete stats by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/203
* feat: add excluded annotation to postgres pod by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/211
* add version tag to config api by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/208

### Other

* build(deps): bump google.golang.org/grpc from 1.75.0 to 1.79.3 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/184
* feat: update docs by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/193
* feat: release frontend by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/210
* release 0.2.5 rc.2 by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/207
* release: v0.2.5-rc.1 by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/202

## v0.2.5-rc.2 (2026-03-27)

### Improvements
* simplifying node_stats_builder.go by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/197
* Remove obsolete write/apply config gates by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/195
* integrate deadcode linter by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/199
* Persist workloads with incomplete stats by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/203
* Allow raw namespaceSelector input for webhook chart by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/204

### Other
* Remove dry-run support by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/196
* Remove unused create-stats max metrics by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/198
* build(deps): bump google.golang.org/grpc from 1.75.0 to 1.79.3 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/184
* release: v0.2.5-rc.1 by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/202

## v0.2.5-rc.1 (2026-03-25)

### Breaking Changes
* Remove obsolete write/apply config gates by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/195
* Remove dry-run support by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/196
* CruiseKube and the Helm charts removed legacy dry-run config, CLI, and chart fields. Recommendation application mutates by default now. Before upgrading, remove all legacy dry-run keys from your values files, environment variables, and CLI flags.

### Improvements
* simplifying node_stats_builder.go by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/197
* Remove unused create-stats max metrics by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/198

### Other
* Removed dry-run support from CruiseKube. Historical changelog entries may still refer to the previous dry-run flow.

## v0.2.4 (2026-03-19)
* Add Google Analytics configuration to mkdocs.yml by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/185
* Fix analytics property key in google tag by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/186
* Mi -> MB by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/187
* frontend update by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/188
* using the latest ratio for calculating original requested allocatable by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/189
* fix e2e for decimal memory unit fixtures by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/190
* fix: don't update recommendation generated for nonOptimizable workloads by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/191

## v0.2.3 (2026-03-18)

* Make Prometheus insecure TLS configurable by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/181
* refactor cleanup task for audits and snapshots by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/179
* fix: negative possible savings issue by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/175


## v0.2.2 (2026-03-18)

* fix incorrect conversion for oom memory by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/151
* feat: add workload fixes, gpu workloads, workload requested etc by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/155
* rounding memory and cpu recommendations before applying by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/157
* remove incorrectly labelled errors by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/156
* Remove continuous_optimization from codebase by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/153
* sentry integration for error reporting by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/149
* implement api to batch update overrides for workloads by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/158
* Batch update workload overrides API fix by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/160
* feat: Include HPA excluded code to summary api by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/159
* Minor fixe in webhook and fetch workloads | Update priority to critical  by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/161
* move incomplete metrics case in create stats to debug by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/163
* add sidecar containers to pod container resources by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/165
* remove recent workload filtering by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/166
* reduce log level for non critical errors by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/167
* remove taskModifyEqualCPUResources by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/170
* add retries with backoff for database initialization by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/169
* fix cpu change issue by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/173
* update values.yaml with sentry env values by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/162
* handle eviction failure due to pod not found by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/174
* build(deps): bump go.opentelemetry.io/otel/sdk from 1.38.0 to 1.40.0 by @dependabot[bot] in https://github.com/truefoundry/CruiseKube/pull/136
* feat: update frontend by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/177


## v0.2.1 (2026-03-05)

### Breaking Changes

* Rename stats table to workloads and modify corresponding functions to load workloads instead of stats by @innoavator in https://github.com/truefoundry/CruiseKube/pull/112
* feat: add snapshot for cluster by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/123
* feat: audit system by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/127
* Remove unused recommender endpoints by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/148
* Change project license to BUSL-1.1 by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/140


### What's Changed
* Update getting started docs by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/94
* feat: dry run fix by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/96
* Excluding best effort pods from optimisation by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/95
* release: v0.1.11-rc.1 by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/97
* release v0.1.11-rc.2 by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/99
* Improved development docs by @innoavator in https://github.com/truefoundry/CruiseKube/pull/103
* Removed the extra overrides API and combined it with the workloads api by @innoavator in https://github.com/truefoundry/CruiseKube/pull/105
* store recommendations to db on every run by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/106
* feat: add cost calculation by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/88
* updating frontend to latest main by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/108
* set eviction ranking to disabled if workload has a do-not-disrupt ann… by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/102
* Added disruption window support by @innoavator in https://github.com/truefoundry/CruiseKube/pull/111
* consolidating workloads summary call by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/110
* implement disruption force task by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/107
* implement workload level disruption window override by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/115
* disable dry run by default and set default mode to recommend only by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/116
* feat: add workload details api by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/117
* fix pdb annotation labels in task disruption force by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/118
* Clarify HPA limitations in CruiseKube documentation by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/122
* delete stale workloads from db by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/120
* Simplification of code for workload summary by @innoavator in https://github.com/truefoundry/CruiseKube/pull/124
* Consolidate AdmissionWebhook and controller via proxy API by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/109
* bump frontend to main by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/126
* move config to db by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/119
* implement disruption window changes in webhook by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/128
* fix selector matching for pdbs and workloads by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/129
* add disruption window state to workload metadata by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/132
* fix disruption force task stat constraint check by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/131
* storing recommended but disabled recommendations as well by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/133
* fix: replace panic() calls with proper error handling by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/134
* Tighten config validation and task config guardrails by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/135
* Update frontend submodule to main by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/138
* Refactor: extract startup assembly from main by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/137
* update only minAvailable or maxUnavailable for pdb based on original … by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/139
* Update frontend submodule to main by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/143
* Add runtime lifecycle manager by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/141
* Isolate scheduler lifecycle ownership by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/144
* feat: add api for snapshot and audit events by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/130
* Introduce first handler dependency container slice by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/145
* Remove CPU 7-day stats and workload analysis API by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/146
* return error if workloads listing fails by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/150
* Add webhook patching tests by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/147
* feat: update worklaod summary by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/152


### New Contributors
* @innoavator made their first contribution in https://github.com/truefoundry/CruiseKube/pull/103


## v0.1.10 (2026-02-12)

### What's Changed
* implement api to trigger a task manually by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/61
* Hotfix for the helm index. by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/63
* Hotfix - Update index.yaml for helm by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/64
* add kuttl e2e tests for apply recommendations by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/59
* feat: docs updates with comparison, limitations and other optimizations by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/65
* update documentation on oom handling by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/66
* cleanup older oom events in db by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/67
* move manual task trigger api to dev api group by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/71
* use dev api for apply recommendations e2e test by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/72
* implement webhook e2e test by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/68
* Enable task stats creation in values.yaml by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/76
* Revise task enabling instructions and Prometheus config by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/77
* Update platforms for Docker build to include arm64 by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/75
* implement oom handling e2e tests by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/69
* added logging for when error is being returned by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/78
* enabling apply recommendation by default by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/79
* feat: use container info from workload, instead of pod by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/73
* update frontend by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/80
* cleanup unused oom query by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/81
* feat: Show only the workloads updated in last 1 day by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/82
* fix original vs pod container info usage by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/86
* update simple timeseries prediction max value calculation by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/84
* allow memory reduction for k8s version >= 1.34 by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/87
* feat: add demarcation metadata to stats by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/85
* allow optimizing guaranteed pods by @maanas-23 in https://github.com/truefoundry/CruiseKube/pull/83
* Relaxing cpu clamp value to 20 by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/89
* Added handling for daemonset pods to not increase resources by @shubhamrai1993 in https://github.com/truefoundry/CruiseKube/pull/90
* fix: ignore if totalRestMemory is zero for a container by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/91
* fix: update dry run fix by @ramantehlan in https://github.com/truefoundry/CruiseKube/pull/92



## v0.1.9 (2026-01-16)

* feat: major oss ready changes by @ramantehlan in [#32](https://github.com/truefoundry/CruiseKube/pull/32)
* added arch section to docs by @shubhamrai1993 in [#33](https://github.com/truefoundry/CruiseKube/pull/33)
* feat: add helm readme generator by @ramantehlan in [#34](https://github.com/truefoundry/CruiseKube/pull/34)
* implement oom informer by @maanas-23 in [#31](https://github.com/truefoundry/CruiseKube/pull/31)
* feat: add get started items and config by @ramantehlan in [#35](https://github.com/truefoundry/CruiseKube/pull/35)
* feat: add workflow, and fix unhandled errors by @ramantehlan in [#36](https://github.com/truefoundry/CruiseKube/pull/36)
* added algorithm details to architecture by @shubhamrai1993 in [#37](https://github.com/truefoundry/CruiseKube/pull/37)
* some cleanup wrt arch algorithm by @shubhamrai1993 in [#39](https://github.com/truefoundry/CruiseKube/pull/39)
* some scheduler refactoring by @shubhamrai1993 in [#40](https://github.com/truefoundry/CruiseKube/pull/40)
* remove prometheus oom query from predictions by @maanas-23 in [#41](https://github.com/truefoundry/CruiseKube/pull/41)
* using non docker registry bitnami chart by @shubhamrai1993 in [#42](https://github.com/truefoundry/CruiseKube/pull/42)
* removing duplicated env variables by @shubhamrai1993 in [#43](https://github.com/truefoundry/CruiseKube/pull/43)
* update oom memory stats and apply oom recommendations by @maanas-23 in [#38](https://github.com/truefoundry/CruiseKube/pull/38)
* feat: add mutex to scheduler by @ramantehlan in [#44](https://github.com/truefoundry/CruiseKube/pull/44)
* Rt helm remove pvc by @ramantehlan in [#45](https://github.com/truefoundry/CruiseKube/pull/45)
* added sections for cpu and memory stats by @shubhamrai1993 in [#46](https://github.com/truefoundry/CruiseKube/pull/46)
* adding all changes to helm-main from main by @shubhamrai1993 in [#47](https://github.com/truefoundry/CruiseKube/pull/47)
* build and push frontend as well by @shubhamrai1993 in [#49](https://github.com/truefoundry/CruiseKube/pull/49)
* viper parses an env variable that is comma separated by @shubhamrai1993 in [#50](https://github.com/truefoundry/CruiseKube/pull/50)
* correcting in documentation for cruisekube usecase by @shubhamrai1993 in [#51](https://github.com/truefoundry/CruiseKube/pull/51)
* Adding launch blog for cruisekube by @shubhamrai1993 in [#52](https://github.com/truefoundry/CruiseKube/pull/52)
* Added cruisekube.com domain name by @shubhamrai1993 in [#54](https://github.com/truefoundry/CruiseKube/pull/54)
* add oom cooldown duration before increasing memory again by @maanas-23 in [#48](https://github.com/truefoundry/CruiseKube/pull/48)
* corrected image urls for cruisekube getting started blog by @shubhamrai1993 in [#55](https://github.com/truefoundry/CruiseKube/pull/55)
* evict pod on OOM by @maanas-23 in [#56](https://github.com/truefoundry/CruiseKube/pull/56)
* removed topology and affinity removal from webhook by @shubhamrai1993 in [#57](https://github.com/truefoundry/CruiseKube/pull/57)
* removed ref to kubeelasti with cruisekube by @shubhamrai1993 in [#58](https://github.com/truefoundry/CruiseKube/pull/58)
* Update github actions & add helm index.yaml by @ramantehlan in [#60](https://github.com/truefoundry/CruiseKube/pull/60)
