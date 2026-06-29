package utils

import (
	"math"

	"github.com/truefoundry/cruisekube/pkg/types"
)

// HPA target utilization is clamped to this range when CruiseKube re-derives it
// after right-sizing a request. The lower bound avoids a degenerate 0% target;
// the upper bound keeps a safety margin so a pod is never driven to ~100% of its
// (now smaller) request before the HPA reacts.
const (
	HPAMinTargetUtilization int32 = 1
	HPAMaxTargetUtilization int32 = 90
)

// HPAMetricTypeUtilization is the autoscaling/v2 MetricTargetType "Utilization"
// (target expressed as a percentage of the resource request). Only this target
// type couples the request to horizontal scaling and needs coordination.
const HPAMetricTypeUtilization = "Utilization"

// CanonicalCPURequest returns the node-independent CPU request CruiseKube would
// recommend for a container (the same value the admission webhook applies):
// max(simple-prediction max, P75), clamped to CPUClampValue. Returns 0 when the
// required stats are missing.
func CanonicalCPURequest(cs *types.ContainerStats) float64 {
	if cs == nil {
		return 0
	}
	rec := 0.0
	if cs.CPUStats != nil {
		rec = cs.CPUStats.P75
	}
	if cs.SimplePredictionsCPU != nil {
		rec = math.Max(rec, cs.SimplePredictionsCPU.MaxValue)
	}
	if rec > CPUClampValue {
		rec = CPUClampValue
	}
	return rec
}

// CanonicalMemoryRequest returns the node-independent memory request CruiseKube
// would recommend for a container (matching the admission webhook):
// max(simple-prediction max, P75), raised to the OOM-observed memory if higher.
// Returns 0 when the required stats are missing.
func CanonicalMemoryRequest(cs *types.ContainerStats) float64 {
	if cs == nil {
		return 0
	}
	rec := 0.0
	if cs.MemoryStats != nil {
		rec = cs.MemoryStats.P75
	}
	if cs.SimplePredictionsMemory != nil {
		rec = math.Max(rec, cs.SimplePredictionsMemory.MaxValue)
	}
	if cs.MemoryStats != nil && cs.MemoryStats.OOMMemory > rec {
		rec = cs.MemoryStats.OOMMemory
	}
	return rec
}

// ComputeCoordinatedHPATarget re-derives an HPA's target averageUtilization so
// that the absolute per-pod scale-out point is preserved when the request is
// right-sized from requestOld to requestNew.
//
// An HPA on a resource drives each pod toward usage = (target/100) * request, so
// the absolute "setpoint" at which it adds/removes replicas is target * request.
// Holding that setpoint constant while the request changes keeps the horizontal
// scaling behavior identical and avoids the request/replica oscillation that
// otherwise makes VPA and HPA fight on the same resource:
//
//	setpoint   = targetOld * requestOld
//	targetNew  = clamp(round(setpoint / requestNew), MIN, MAX)
//
// ok is false when any input is non-positive (target cannot be coordinated).
func ComputeCoordinatedHPATarget(targetOld int32, requestOld, requestNew float64) (int32, bool) {
	if targetOld <= 0 || requestOld <= 0 || requestNew <= 0 {
		return 0, false
	}
	setpoint := float64(targetOld) * requestOld
	target := int32(math.Round(setpoint / requestNew))
	if target < HPAMinTargetUtilization {
		target = HPAMinTargetUtilization
	}
	if target > HPAMaxTargetUtilization {
		target = HPAMaxTargetUtilization
	}
	return target, true
}
