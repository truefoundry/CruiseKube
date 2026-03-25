package task

import (
	"context"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type fakeWorkloadObject struct {
	creationTime time.Time
	replicas     int32
}

func (f fakeWorkloadObject) GetNamespace() string { return "ns" }
func (f fakeWorkloadObject) GetName() string      { return "app" }
func (f fakeWorkloadObject) GetContainerSpecs(context.Context, *kubernetes.Clientset) []corev1.Container {
	return nil
}
func (f fakeWorkloadObject) GetInitContainerSpecs(context.Context, *kubernetes.Clientset) []corev1.Container {
	return nil
}
func (f fakeWorkloadObject) GetSelector() (labels.Selector, error) { return labels.Everything(), nil }
func (f fakeWorkloadObject) GetCreationTime() time.Time            { return f.creationTime }
func (f fakeWorkloadObject) GetReplicas() int32                    { return f.replicas }

func TestBuildBaseWorkloadStatUsesWorkloadReplicaFallback(t *testing.T) {
	task := &CreateStatsTask{}
	workloadInfo := utils.WorkloadInfo{Kind: "Deployment", Namespace: "ns", Name: "app"}
	workloadObj := fakeWorkloadObject{creationTime: time.Unix(100, 0).UTC(), replicas: 3}
	containerResources := []utils.OriginalContainerResources{{Name: "main", Type: types.AppContainer, CPURequest: 1, MemoryRequest: 256}}

	stat := task.buildBaseWorkloadStat(workloadInfo, workloadObj, containerResources, nil)

	if stat.Replicas != 3 {
		t.Fatalf("expected workload replicas fallback to be used, got %d", stat.Replicas)
	}
	if len(stat.ContainerStats) != 1 || stat.ContainerStats[0].ContainerName != "main" {
		t.Fatalf("expected placeholder container stats to be created, got %#v", stat.ContainerStats)
	}
}

func TestBuildBaseWorkloadStatPrefersWorkloadMetricsReplicaCount(t *testing.T) {
	task := &CreateStatsTask{}
	workloadInfo := utils.WorkloadInfo{Kind: "Deployment", Namespace: "ns", Name: "app"}
	workloadObj := fakeWorkloadObject{creationTime: time.Unix(100, 0).UTC(), replicas: 1}
	containerResources := []utils.OriginalContainerResources{{Name: "main", Type: types.AppContainer}}
	workloadKey := utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)
	workloadMetrics := utils.NamespaceVsWorkloadMetrics{
		"ns": {
			workloadKey: {MedianReplicas: 5},
		},
	}

	stat := task.buildBaseWorkloadStat(workloadInfo, workloadObj, containerResources, workloadMetrics)

	if stat.Replicas != 5 {
		t.Fatalf("expected workload metrics replicas to override workload replicas, got %d", stat.Replicas)
	}
}

func TestBuildBaseWorkloadStatDoesNotOverwriteReplicaFallbackWithZeroMetric(t *testing.T) {
	task := &CreateStatsTask{}
	workloadInfo := utils.WorkloadInfo{Kind: "Deployment", Namespace: "ns", Name: "app"}
	workloadObj := fakeWorkloadObject{creationTime: time.Unix(100, 0).UTC(), replicas: 3}
	containerResources := []utils.OriginalContainerResources{{Name: "main", Type: types.AppContainer}}
	workloadKey := utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)
	workloadMetrics := utils.NamespaceVsWorkloadMetrics{
		"ns": {
			workloadKey: {MedianReplicas: 0},
		},
	}

	stat := task.buildBaseWorkloadStat(workloadInfo, workloadObj, containerResources, workloadMetrics)

	if stat.Replicas != 3 {
		t.Fatalf("expected workload replica fallback to be preserved when metric replicas are zero, got %d", stat.Replicas)
	}
}

func TestMarkWorkloadStatIncompletePreservesExistingExcludedCodes(t *testing.T) {
	task := &CreateStatsTask{}
	stat := &utils.WorkloadStat{
		OriginalContainerResources: []utils.OriginalContainerResources{
			{Name: "main", Type: types.AppContainer},
			{Name: "init", Type: types.InitContainer},
		},
		Metadata: &types.WorkloadStatMetadata{
			Excluded:      true,
			ExcludedCodes: []types.ExcludedCode{types.ExcludedCodeCPUHPA},
		},
	}

	task.markWorkloadStatIncomplete(stat)

	if !stat.IsIncomplete() {
		t.Fatal("expected workload stat to be marked incomplete")
	}
	if !stat.HasExcludedCode(types.ExcludedCodeCPUHPA) {
		t.Fatal("expected existing excluded code to be preserved")
	}
	if !stat.HasExcludedCode(types.ExcludedCodeIncomplete) {
		t.Fatal("expected incomplete excluded code to be added")
	}
	if len(stat.ContainerStats) != 2 {
		t.Fatalf("expected placeholder container stats for all original containers, got %d", len(stat.ContainerStats))
	}
}
