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

func minimalPod(namespace, name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
		Spec:       corev1.PodSpec{},
	}
}

// EmptyCategories: both cpu and memory categories are empty; function should no-op and not call Patch.
func TestPatchPodHeadroomLabels_EmptyCategories(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil)
	patched, errStr := PatchPodHeadroomLabels(ctx, patcher, pod, "", "")
	if patched != false || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (false, \"\")", patched, errStr)
	}
	if patcher.lastNamespace != "" || patcher.lastName != "" || len(patcher.lastData) != 0 {
		t.Errorf("Patch should not have been called; got namespace=%q name=%q dataLen=%d", patcher.lastNamespace, patcher.lastName, len(patcher.lastData))
	}
}

// AdditionCPUOnly: pod has no headroom labels; adding CPU category only. Patch called once.
func TestPatchPodHeadroomLabels_AdditionCPUOnly(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil)
	patched, errStr := PatchPodHeadroomLabels(ctx, patcher, pod, "high", "")
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
}

// AdditionBothCategories: pod has no headroom; add both CPU and memory.
func TestPatchPodHeadroomLabels_AdditionBothCategories(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil)
	patched, errStr := PatchPodHeadroomLabels(ctx, patcher, pod, "high", "low")
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
}

// UpdateLabelPresent: pod already has headroom label; patch updates only labels.
func TestPatchPodHeadroomLabels_UpdateLabelPresent(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", map[string]string{HeadroomGroupCPULabel: "old"})
	patched, errStr := PatchPodHeadroomLabels(ctx, patcher, pod, "new", "")
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
}

// Noop: labels already match desired categories; function should return (false, "") and not call Patch.
func TestPatchPodHeadroomLabels_Noop(t *testing.T) {
	patcher := &mockPodPatcher{}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", map[string]string{HeadroomGroupCPULabel: "high", HeadroomGroupMemoryLabel: "low"})
	patched, errStr := PatchPodHeadroomLabels(ctx, patcher, pod, "high", "low")
	if patched != false || errStr != "" {
		t.Errorf("got (patched=%v, errStr=%q), want (false, \"\")", patched, errStr)
	}
	if len(patcher.lastData) != 0 {
		t.Errorf("Patch should not have been called")
	}
}

// PatchError: mock Patch returns an error; function must return (false, error string) with that error.
func TestPatchPodHeadroomLabels_PatchError(t *testing.T) {
	injectedErr := errors.New("injected")
	patcher := &mockPodPatcher{patchErr: injectedErr}
	ctx := context.Background()
	pod := minimalPod("ns", "pod1", nil)
	patched, errStr := PatchPodHeadroomLabels(ctx, patcher, pod, "high", "")
	if patched != false {
		t.Errorf("got patched=%v, want false", patched)
	}
	wantErr := "failed to patch pod headroom: " + injectedErr.Error()
	if errStr != wantErr {
		t.Errorf("got errStr=%q, want %q", errStr, wantErr)
	}
}
