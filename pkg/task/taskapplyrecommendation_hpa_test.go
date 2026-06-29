package task

import (
	"context"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func i32(i int32) *int32 { return &i }

var hpaGVR = schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}

func cpuHPAObject() *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta:   metav1.TypeMeta{APIVersion: "autoscaling/v2", Kind: "HorizontalPodAutoscaler"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "web", APIVersion: "apps/v1"},
			MinReplicas:    i32(2),
			MaxReplicas:    10,
			Metrics: []autoscalingv2.MetricSpec{{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name:   corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType, AverageUtilization: i32(70)},
				},
			}},
		},
	}
}

func fakeDynamic(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	_ = autoscalingv2.AddToScheme(scheme)
	return dynamicfake.NewSimpleDynamicClient(scheme, objs...)
}

func liveTargetUtilization(t *testing.T, client dynamic.Interface) int32 {
	t.Helper()
	obj, err := client.Resource(hpaGVR).Namespace("default").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get HPA error: %v", err)
	}
	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &hpa); err != nil {
		t.Fatalf("convert error: %v", err)
	}
	return *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization
}

func hpaWorkload() *types.WorkloadInCluster {
	return &types.WorkloadInCluster{
		WorkloadID: utils.GetWorkloadKey("Deployment", "default", "web"),
		Stat: &types.WorkloadStat{
			HPA: &types.HPAInfo{
				Name:      "web",
				Namespace: "default",
				CPU: &types.HPAResourceTarget{
					MetricType:             utils.HPAMetricTypeUtilization,
					AverageUtilization:     70,
					RecommendedUtilization: 35,
				},
			},
		},
	}
}

func newApplyTask(client dynamic.Interface, flag bool) *ApplyRecommendationTask {
	return &ApplyRecommendationTask{
		dynamicClient: client,
		config: &ApplyRecommendationTaskConfig{
			RecommendationSettings: config.RecommendationSettings{HPAResourceAwareOptimization: flag},
		},
	}
}

func enabledOverride(workloadID string, enabled bool) map[string]*types.WorkloadOverrideInfo {
	return map[string]*types.WorkloadOverrideInfo{
		workloadID: {Overrides: &types.WorkloadOverridesEffective{Enabled: enabled}},
	}
}

func TestReconcileHPATargetsCruiseEnabledPatchesHPA(t *testing.T) {
	client := fakeDynamic(cpuHPAObject())
	a := newApplyTask(client, true)
	w := hpaWorkload()

	a.reconcileHPATargets(context.Background(), []*types.WorkloadInCluster{w}, enabledOverride(w.WorkloadID, true))

	if got := liveTargetUtilization(t, client); got != 35 {
		t.Fatalf("HPA target = %d, want 35 (coordinated)", got)
	}
}

func TestReconcileHPATargetsRecommendOnlyDoesNotPatch(t *testing.T) {
	client := fakeDynamic(cpuHPAObject())
	a := newApplyTask(client, true)
	w := hpaWorkload()

	// Override disabled => recommend-only => HPA must be left untouched.
	a.reconcileHPATargets(context.Background(), []*types.WorkloadInCluster{w}, enabledOverride(w.WorkloadID, false))

	if got := liveTargetUtilization(t, client); got != 70 {
		t.Fatalf("HPA target = %d, want 70 (unchanged)", got)
	}
}

func TestReconcileHPATargetsFlagOffDoesNotPatch(t *testing.T) {
	client := fakeDynamic(cpuHPAObject())
	a := newApplyTask(client, false)
	w := hpaWorkload()

	a.reconcileHPATargets(context.Background(), []*types.WorkloadInCluster{w}, enabledOverride(w.WorkloadID, true))

	if got := liveTargetUtilization(t, client); got != 70 {
		t.Fatalf("HPA target = %d, want 70 (feature disabled)", got)
	}
}
