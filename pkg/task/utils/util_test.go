package utils

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/adapters/kube"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// mockPodPatcher implements PodPatcher by recording the last Patch call and optionally returning a configured error.
type mockPodPatcher struct {
	lastNamespace string
	lastName      string
	lastData      []byte
	patchErr      error
}

var _ kube.PodPatcher = (*mockPodPatcher)(nil)

func (m *mockPodPatcher) Patch(ctx context.Context, namespace, name string, pt k8stypes.PatchType, data []byte, opts metav1.PatchOptions) (*corev1.Pod, error) {
	m.lastNamespace = namespace
	m.lastName = name
	m.lastData = data
	if m.patchErr != nil {
		return nil, m.patchErr
	}
	return nil, nil
}

// lastPatchPod decodes the last Patch call's data as a Pod for assertions.
func (m *mockPodPatcher) lastPatchPod() (*corev1.Pod, error) {
	if len(m.lastData) == 0 {
		return nil, nil
	}
	var p corev1.Pod
	if err := json.Unmarshal(m.lastData, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// minimalPod builds a pod with optional labels and optional PodAffinity preferred terms for test setup.
func minimalPod(namespace, name string, labels map[string]string, preferred []corev1.WeightedPodAffinityTerm) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
		Spec:      corev1.PodSpec{},
	}
	if len(preferred) > 0 {
		p.Spec.Affinity = &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: preferred,
			},
		}
	}
	return p
}

// EmptyCategories: both cpu and memory categories are empty; function should no-op and not call Patch.
func TestPatchPodHeadroomLabelsAndAffinity_EmptyCategories(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil, nil)
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "", "")
	if patched != false || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (false, \"\")", patched, errStr)
	}
	if patcher.lastNamespace != "" || patcher.lastName != "" || len(patcher.lastData) != 0 {
		t.Errorf("Patch should not have been called; got namespace=%q name=%q dataLen=%d", patcher.lastNamespace, patcher.lastName, len(patcher.lastData))
	}
}

// AdditionCPUOnly: pod has no headroom labels; adding CPU category only. Patch called once, body has CPU label and one preferred term.
func TestPatchPodHeadroomLabelsAndAffinity_AdditionCPUOnly(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil, nil)
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "high", "")
	if !patched || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (true, \"\")", patched, errStr)
	}
	if patcher.lastNamespace != "ns" || patcher.lastName != "pod1" || len(patcher.lastData) == 0 {
		t.Errorf("Patch should have been called once; got namespace=%q name=%q dataLen=%d", patcher.lastNamespace, patcher.lastName, len(patcher.lastData))
	}
	decoded, err := patcher.lastPatchPod()
	if err != nil {
		t.Fatalf("decode patch body: %v", err)
	}
	if decoded.Labels[HeadroomGroupCPULabel] != "high" {
		t.Errorf("patch body labels: got %q for %s, want \"high\"", decoded.Labels[HeadroomGroupCPULabel], HeadroomGroupCPULabel)
	}
	pref := decoded.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	if len(pref) != 1 {
		t.Errorf("patch body preferred terms: got %d, want 1", len(pref))
	}
	if len(pref) > 0 && (pref[0].PodAffinityTerm.LabelSelector == nil || len(pref[0].PodAffinityTerm.LabelSelector.MatchExpressions) == 0 || pref[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key != HeadroomGroupCPULabel) {
		t.Errorf("patch body preferred term[0] should be headroom CPU term")
	}
}

// AdditionBothCategories: pod has no headroom; add both CPU and memory. Patch body must have both labels and two preferred terms.
func TestPatchPodHeadroomLabelsAndAffinity_AdditionBothCategories(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil, nil)
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "high", "low")
	if !patched || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (true, \"\")", patched, errStr)
	}
	decoded, err := patcher.lastPatchPod()
	if err != nil {
		t.Fatalf("decode patch body: %v", err)
	}
	if decoded.Labels[HeadroomGroupCPULabel] != "high" || decoded.Labels[HeadroomGroupMemoryLabel] != "low" {
		t.Errorf("patch body labels: got cpu=%q mem=%q, want high, low", decoded.Labels[HeadroomGroupCPULabel], decoded.Labels[HeadroomGroupMemoryLabel])
	}
	pref := decoded.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	if len(pref) != 2 {
		t.Errorf("patch body preferred terms: got %d, want 2", len(pref))
	}
}

// AdditionDedupe: affinity already has the headroom term for the given category; result must not duplicate that term.
func TestPatchPodHeadroomLabelsAndAffinity_AdditionDedupe(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	existingTerms := HeadroomPreferredAffinityTerms("high", "")
	pod := minimalPod("ns", "pod1", nil, existingTerms)
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "high", "")
	if !patched || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (true, \"\")", patched, errStr)
	}
	decoded, err := patcher.lastPatchPod()
	if err != nil {
		t.Fatalf("decode patch body: %v", err)
	}
	pref := decoded.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	cpuTerms := 0
	for _, t := range pref {
		if t.PodAffinityTerm.LabelSelector != nil && len(t.PodAffinityTerm.LabelSelector.MatchExpressions) == 1 && t.PodAffinityTerm.LabelSelector.MatchExpressions[0].Key == HeadroomGroupCPULabel {
			cpuTerms++
		}
	}
	if cpuTerms != 1 {
		t.Errorf("preferred list should have exactly one headroom CPU term (no duplicate), got %d", cpuTerms)
	}
}

// UpdateLabelPresent: pod already has headroom label and affinity. Patch must update label/headroom term and preserve non-headroom terms.
func TestPatchPodHeadroomLabelsAndAffinity_UpdateLabelPresent(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	oldCPUTerm := HeadroomPreferredAffinityTerms("old", "")[0]
	nonHeadroomTerm := corev1.WeightedPodAffinityTerm{
		Weight: 50,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "other/key", Operator: metav1.LabelSelectorOpIn, Values: []string{"x"}}},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	}
	pod := minimalPod("ns", "pod1", map[string]string{HeadroomGroupCPULabel: "old"}, []corev1.WeightedPodAffinityTerm{nonHeadroomTerm, oldCPUTerm})
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "new", "")
	if !patched || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (true, \"\")", patched, errStr)
	}
	decoded, err := patcher.lastPatchPod()
	if err != nil {
		t.Fatalf("decode patch body: %v", err)
	}
	if decoded.Labels[HeadroomGroupCPULabel] != "new" {
		t.Errorf("patch body label: got %q, want \"new\"", decoded.Labels[HeadroomGroupCPULabel])
	}
	pref := decoded.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	hasNonHeadroom := false
	hasNewCPU := false
	for _, t := range pref {
		if t.PodAffinityTerm.LabelSelector != nil && len(t.PodAffinityTerm.LabelSelector.MatchExpressions) == 1 {
			key := t.PodAffinityTerm.LabelSelector.MatchExpressions[0].Key
			if key == "other/key" {
				hasNonHeadroom = true
			}
			if key == HeadroomGroupCPULabel && len(t.PodAffinityTerm.LabelSelector.MatchExpressions[0].Values) > 0 && t.PodAffinityTerm.LabelSelector.MatchExpressions[0].Values[0] == "new" {
				hasNewCPU = true
			}
		}
	}
	if !hasNonHeadroom {
		t.Errorf("patch should keep non-headroom preferred term")
	}
	if !hasNewCPU {
		t.Errorf("patch should have headroom term for new")
	}
}

// UpdateNoAffinity: pod has headroom label but Spec.Affinity or PodAffinity is nil. Must return error and not call Patch
func TestPatchPodHeadroomLabelsAndAffinity_UpdateNoAffinity(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", map[string]string{HeadroomGroupCPULabel: "x"}, nil)
	pod.Spec.Affinity = nil
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "new", "")
	if patched != false {
		t.Errorf("got patched=%v, want false", patched)
	}
	if errStr != "headroom update requires existing pod affinity" {
		t.Errorf("got errStr=%q, want \"headroom update requires existing pod affinity\"", errStr)
	}
	if len(patcher.lastData) != 0 {
		t.Errorf("Patch should not have been called")
	}
	pod2 := minimalPod("ns", "pod2", map[string]string{HeadroomGroupCPULabel: "x"}, nil)
	pod2.Spec.Affinity = &corev1.Affinity{}
	patcher2 := &mockPodPatcher{}
	patched2, errStr2 := PatchPodHeadroomLabelsAndAffinity(ctx, patcher2, pod2, "new", "")
	if patched2 != false || errStr2 != "headroom update requires existing pod affinity" || len(patcher2.lastData) != 0 {
		t.Errorf("PodAffinity nil: got (patched=%v, errStr=%q), Patch called=%v", patched2, errStr2, len(patcher2.lastData) != 0)
	}
}

// Noop: labels and affinity already match desired categories; function should return (false, "") and not call Patch.
func TestPatchPodHeadroomLabelsAndAffinity_Noop(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	terms := HeadroomPreferredAffinityTerms("high", "low")
	pod := minimalPod("ns", "pod1", map[string]string{HeadroomGroupCPULabel: "high", HeadroomGroupMemoryLabel: "low"}, terms)
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "high", "low")
	if patched != false || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (false, \"\")", patched, errStr)
	}
	if len(patcher.lastData) != 0 {
		t.Errorf("Patch should not have been called")
	}
}

// PatchError: mock Patch returns an error; function must return (false, error string) with that error.
func TestPatchPodHeadroomLabelsAndAffinity_PatchError(t *testing.T) {
	injectedErr := errors.New("injected")
	patcher := &mockPodPatcher{patchErr: injectedErr}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil, nil)
	patched, errStr := PatchPodHeadroomLabelsAndAffinity(ctx, patcher, pod, "high", "")
	if patched != false {
		t.Errorf("got patched=%v, want false", patched)
	}
	wantErr := "failed to patch pod headroom: " + injectedErr.Error()
	if errStr != wantErr {
		t.Errorf("got errStr=%q, want %q", errStr, wantErr)
	}
}
