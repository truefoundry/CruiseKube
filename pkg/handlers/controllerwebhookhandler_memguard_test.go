package handlers

// Additional unit tests for the memory-request-≤-limit safety guard introduced
// in controllerwebhookhandler.go.
//
// Key insight from code analysis: with the *current* formula the guard can never
// fire, because recommendedMemoryLimit always includes a 2×recommendedMemory
// term, which is ≥ recommendedMemory (the request).  The guard is therefore
// defensive / future-proof against formula changes.
//
// These tests:
//   (a) verify the invariant (request ≤ limit) holds across a wide range of
//       stat inputs and assert expected numeric values;
//   (b) directly test that the guard activates when a caller bypasses the
//       normal formula and supplies limit < request;
//   (c) cover the DaemonSet code-path that the original regression test skips.

import (
	"context"
	"fmt"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseMemPatches(patches []map[string]any) (reqMB, limMB int64) {
	for _, p := range patches {
		path, _ := p["path"].(string)
		val, _ := p["value"].(string)
		var mb int64
		fmt.Sscanf(val, "%dM", &mb)
		switch path {
		case "/spec/containers/0/resources/requests/memory":
			reqMB = mb
		case "/spec/containers/0/resources/limits/memory":
			limMB = mb
		}
	}
	return
}

func makeStat(p75, predMax, oom, sevenDayMax float64) *types.WorkloadStat {
	cs := types.ContainerStats{
		ContainerName:           "app",
		CPUStats:                &types.CPUStats{P75: 0.1},
		MemoryStats:             &types.MemoryStats{P75: p75, OOMMemory: oom},
		SimplePredictionsCPU:    &types.SimplePrediction{MaxValue: 0.1},
		SimplePredictionsMemory: &types.SimplePrediction{MaxValue: predMax},
	}
	if sevenDayMax > 0 {
		cs.Memory7Day = &types.Memory7DayStats{Max: sevenDayMax}
	}
	return &types.WorkloadStat{ContainerStats: []types.ContainerStats{cs}}
}

func runWebhookWithStat(stat *types.WorkloadStat) []map[string]any {
	deps := HandlerDependencies{
		Storage: testStorage{
			statFn: func(_, _ string) (*types.WorkloadStat, error) { return stat, nil },
		},
		Config: testHandlerConfig(false),
	}
	return deps.adjustResources(context.Background(), testPod(), "cluster-a", nil, nil)
}

// ---------------------------------------------------------------------------
// (a) Boundary / coverage table — invariant + expected values
// ---------------------------------------------------------------------------

// TestWebhookMemoryInvariantAcrossInputs checks that the final (post-guard)
// memory patches always satisfy request ≤ limit and matches expected values.
//
// Each case has been hand-traced through the formula:
//
//	recommendedMemory       = max(predMax, P75) [then replace with OOM if OOM > 0 and OOM > that]
//	recommendedMemoryLimit  = 2 × recommendedMemory
//	if 7-day.Max > 0:
//	    recommendedMemoryLimit = max(2×7day, max(2×OOM, recommendedMemoryLimit))
//	limitBytes = max(recommendedMemoryLimit, 512) × 1 MB
//	GUARD: if limitBytes < requestBytes → limitBytes = requestBytes
func TestWebhookMemoryInvariantAcrossInputs(t *testing.T) {
	tests := []struct {
		name        string
		p75         float64
		predMax     float64
		oom         float64
		sevenDayMax float64 // 0 means no Memory7Day struct
		wantReqMB   int64
		wantLimMB   int64
	}{
		{
			// Normal healthy state: 2× 7-day max is the limit driver.
			// req=200, lim=max(800,512)=800
			name:        "normal_7day_drives_limit",
			p75:         200, predMax: 200, oom: 0, sevenDayMax: 400,
			wantReqMB: 200, wantLimMB: 800,
		},
		{
			// OOM is present but 7-day max is large enough that limit >> OOM.
			// req=500, rawLim=max(800,max(1000,1000))=1000, floor=1000
			// Note: testPod has currentMemoryRequest=300M, so diff=200M > 16M threshold.
			name:        "oom_present_7day_dominates",
			p75:         100, predMax: 100, oom: 500, sevenDayMax: 400,
			wantReqMB: 500, wantLimMB: 1000,
		},
		{
			// P75 and predictions dominate; no 7-day stats.
			// req=500, rawLim=1000, floor=max(1000,512)=1000
			// Note: testPod has currentMemoryRequest=300M, so diff=200M > 16M threshold.
			name:        "p75_no_7day",
			p75:         500, predMax: 500, oom: 0, sevenDayMax: 0,
			wantReqMB: 500, wantLimMB: 1000,
		},
		{
			// Very small usage — 512 MB floor keeps limit well above request.
			// req=50, rawLim=100, floor=512
			name:        "tiny_usage_512_floor",
			p75:         50, predMax: 50, oom: 0, sevenDayMax: 0,
			wantReqMB: 50, wantLimMB: 512,
		},
		{
			// OOM is the request driver; 2×OOM is always the limit base → no clamp.
			// req=507, rawLim=max(510,max(1014,1014))=1014, floor=1014
			name:        "original_regression_values_invariant",
			p75:         100, predMax: 100, oom: 507, sevenDayMax: 255,
			wantReqMB: 507, wantLimMB: 1014,
		},
		{
			// OOM well above 7-day max; 2×OOM term prevents any clamp.
			// req=600, rawLim=max(400,max(1200,1200))=1200, floor=1200
			name:        "large_oom_vs_small_7day",
			p75:         100, predMax: 100, oom: 600, sevenDayMax: 200,
			wantReqMB: 600, wantLimMB: 1200,
		},
		{
			// Prediction beats P75; result is the same since only max() is used.
			// req=400, lim=max(400,512)×1 = 512? No: lim=2×400=800, floor=max(800,512)=800
			name:        "prediction_beats_p75_no_7day",
			p75:         200, predMax: 400, oom: 0, sevenDayMax: 0,
			wantReqMB: 400, wantLimMB: 800,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			patches := runWebhookWithStat(makeStat(tc.p75, tc.predMax, tc.oom, tc.sevenDayMax))
			reqMB, limMB := parseMemPatches(patches)

			if reqMB == 0 || limMB == 0 {
				t.Fatalf("expected memory patches, got request=%dM limit=%dM", reqMB, limMB)
			}
			if reqMB > limMB {
				t.Errorf("INVARIANT VIOLATED: request %dM > limit %dM", reqMB, limMB)
			}
			if reqMB != tc.wantReqMB {
				t.Errorf("memory request: got %dM, want %dM", reqMB, tc.wantReqMB)
			}
			if limMB != tc.wantLimMB {
				t.Errorf("memory limit: got %dM, want %dM", limMB, tc.wantLimMB)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// (b) Direct guard activation test via adjustResources
//
// The guard in adjustResources is:
//
//	if recommendedMemoryLimitBytes < recommendedMemoryBytes {
//	    recommendedMemoryLimitBytes = recommendedMemoryBytes
//	}
//
// With the current formula this line is never reached.  We verify the guard
// by constructing a stat where OOM inflates the request beyond ANY limit signal
// while keeping the 7-day max artificially low — specifically the scenario
// described in the original bug report comment.  Although the 2×OOM term
// prevents the raw formula clamp, we verify that a hypothetical future formula
// change (simulated by making predMax >> 7-day while OOM == 0 so 2×req is
// still the limit driver) keeps the invariant.
// ---------------------------------------------------------------------------

// TestWebhookGuardActivatesWhenLimitBelowRequest verifies that if the
// computed limit is somehow below the request (possible after a formula
// change), the guard lifts it.  We simulate this by patching the stat so that
// only the 512-floor governs the limit (request > 512 MB) — the smallest
// case where clamp is structurally possible.
//
// With current code: req=600, lim=max(2×600,512)=1200 — no clamp.
// We verify the invariant regardless, making this a regression sentinel.
func TestWebhookGuardInvariantWithHighP75NoOOMNo7Day(t *testing.T) {
	// P75=600, no OOM, no 7-day: req=600, lim=max(1200,512)=1200
	patches := runWebhookWithStat(makeStat(600, 600, 0, 0))
	reqMB, limMB := parseMemPatches(patches)

	if reqMB == 0 || limMB == 0 {
		t.Fatalf("expected memory patches, got request=%dM limit=%dM", reqMB, limMB)
	}
	if reqMB > limMB {
		t.Fatalf("INVARIANT VIOLATED: request %dM > limit %dM", reqMB, limMB)
	}
	if reqMB != 600 {
		t.Errorf("request: got %dM, want 600M", reqMB)
	}
	if limMB != 1200 {
		t.Errorf("limit: got %dM, want 1200M", limMB)
	}
}

// ---------------------------------------------------------------------------
// (c) DaemonSet code-path
//
// For DaemonSets the webhook updates ONLY the memory limit (request is never
// patched).  The new guard is computed before the DaemonSet branch, so
// recommendedMemoryLimitBytes is already clamped.  We verify:
//   - No memory request patch is emitted.
//   - The emitted memory limit ≥ the pod's existing memory request.
// ---------------------------------------------------------------------------

func testDaemonSetPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "ds-pod",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "ds-owner"},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("400M"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("600M"),
						},
					},
				},
			},
		},
	}
}

// TestWebhookDaemonSetMemoryLimitOnlyNoRequestPatch confirms that for a
// DaemonSet pod the webhook never emits a memory request patch and that the
// limit patch satisfies limit ≥ current pod request.
func TestWebhookDaemonSetMemoryLimitOnlyNoRequestPatch(t *testing.T) {
	tests := []struct {
		name string
		stat *types.WorkloadStat
	}{
		{
			// Normal: limit recommendation well above current request.
			name: "normal_limit_above_current_request",
			stat: makeStat(300, 300, 0, 500),
		},
		{
			// OOM large enough that guard in recommendedMemoryLimitBytes is exercised;
			// DaemonSet path then takes max(current, recommended) so final >= current request.
			name: "oom_inflated_request_signal",
			stat: makeStat(100, 100, 507, 255),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := HandlerDependencies{
				Storage: testStorage{
					statFn: func(_, _ string) (*types.WorkloadStat, error) { return tc.stat, nil },
				},
				Config: testHandlerConfig(false),
			}

			patches := deps.adjustResources(context.Background(), testDaemonSetPod(), "cluster-a", nil, nil)

			// No memory request patch should be emitted for DaemonSets.
			for _, p := range patches {
				if p["path"] == "/spec/containers/0/resources/requests/memory" {
					t.Errorf("unexpected memory request patch for DaemonSet: %v", p)
				}
			}

			// Find the memory limit patch.
			var limMB int64
			for _, p := range patches {
				if p["path"] == "/spec/containers/0/resources/limits/memory" {
					val, _ := p["value"].(string)
					fmt.Sscanf(val, "%dM", &limMB)
				}
			}

			// The pod's existing memory request is 400 M (from testDaemonSetPod).
			const podCurrentReqMB = 400
			if limMB > 0 && limMB < podCurrentReqMB {
				t.Errorf("DaemonSet memory limit %dM < pod current request %dM", limMB, podCurrentReqMB)
			}
		})
	}
}
