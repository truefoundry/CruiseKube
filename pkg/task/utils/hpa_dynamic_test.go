package utils

import (
	"context"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func int32ptr(i int32) *int32 { return &i }

var hpaGVRForTest = schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}

func newCPUHPA() *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta:   metav1.TypeMeta{APIVersion: "autoscaling/v2", Kind: "HorizontalPodAutoscaler"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "web", APIVersion: "apps/v1"},
			MinReplicas:    int32ptr(2),
			MaxReplicas:    10,
			Metrics: []autoscalingv2.MetricSpec{{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name:   corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType, AverageUtilization: int32ptr(70)},
				},
			}},
		},
	}
}

func newFakeDynamicClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	_ = autoscalingv2.AddToScheme(scheme)
	return dynamicfake.NewSimpleDynamicClient(scheme, objs...)
}

func TestCollectHPAInfo(t *testing.T) {
	client := newFakeDynamicClient(newCPUHPA())

	info, err := CollectHPAInfo(context.Background(), client, "")
	if err != nil {
		t.Fatalf("CollectHPAInfo error: %v", err)
	}

	key := GetWorkloadKey("Deployment", "default", "web")
	got, ok := info[key]
	if !ok {
		t.Fatalf("expected HPA info for %s, got keys %v", key, info)
	}
	if got.Name != "web" || got.Namespace != "default" {
		t.Fatalf("unexpected HPA identity: %+v", got)
	}
	if got.CPU == nil || got.CPU.MetricType != HPAMetricTypeUtilization || got.CPU.AverageUtilization != 70 {
		t.Fatalf("unexpected CPU target: %+v", got.CPU)
	}
	if got.Memory != nil {
		t.Fatalf("expected no memory target, got %+v", got.Memory)
	}
	if got.MinReplicas != 2 || got.MaxReplicas != 10 {
		t.Fatalf("unexpected replica bounds: min=%d max=%d", got.MinReplicas, got.MaxReplicas)
	}
}

func TestUpdateHPATargetUtilizations(t *testing.T) {
	client := newFakeDynamicClient(newCPUHPA())
	ctx := context.Background()

	changed, err := UpdateHPATargetUtilizations(ctx, client, "default", "web", map[corev1.ResourceName]int32{corev1.ResourceCPU: 35})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	obj, err := client.Resource(hpaGVRForTest).Namespace("default").Get(ctx, "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &hpa); err != nil {
		t.Fatalf("convert error: %v", err)
	}
	if got := *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization; got != 35 {
		t.Fatalf("target = %d, want 35", got)
	}

	// Idempotent: applying the same value again is a no-op.
	changed, err = UpdateHPATargetUtilizations(ctx, client, "default", "web", map[corev1.ResourceName]int32{corev1.ResourceCPU: 35})
	if err != nil {
		t.Fatalf("second update error: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on idempotent update")
	}
}
