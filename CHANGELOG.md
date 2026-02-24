# Changelog

All notable changes to CruiseKube will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

All the unreleased changes are listed under `Unreleased` section. Add your changes here, they will be moved to the next release.

## Unreleased

### Breaking Changes

* Rename stats table to workloads and modify corresponding functions to load workloads instead of stats by @innoavator in https://github.com/truefoundry/CruiseKube/pull/112


### What's Changed
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

## New Contributors
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


