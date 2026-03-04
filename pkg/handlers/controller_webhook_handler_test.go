package handlers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type testStorage struct {
	statFn      func(clusterID, workloadID string) (*types.WorkloadStat, error)
	overridesFn func(clusterID, workloadID string) (*types.Overrides, error)
}

func (s testStorage) ClusterStatsExists(clusterID string) (bool, error) {
	return false, nil
}

func (s testStorage) ReadClusterStats(clusterID string, target *types.StatsResponse) error {
	return nil
}

func (s testStorage) GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error) {
	if s.statFn != nil {
		return s.statFn(clusterID, workloadID)
	}
	return nil, nil
}

func (s testStorage) GetWorkloadOverrides(clusterID, workloadID string) (*types.Overrides, error) {
	if s.overridesFn != nil {
		return s.overridesFn(clusterID, workloadID)
	}
	return nil, nil
}

func (s testStorage) GetWorkloadsInCluster(clusterID string) ([]*types.WorkloadInCluster, error) {
	return nil, nil
}

func (s testStorage) GetPodRecommendationsForWorkload(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error) {
	return nil, nil
}

func (s testStorage) GetAllStatsForCluster(clusterID string) ([]types.WorkloadStat, error) {
	return nil, nil
}

func (s testStorage) UpdateWorkloadOverrides(clusterID, workloadID string, overrides *types.Overrides) error {
	return nil
}

func (s testStorage) GetAuditEvents(clusterID string, since time.Time) ([]types.AuditEventRecord, error) {
	return nil, nil
}

func (s testStorage) GetAuditEventsForWorkload(clusterID, workloadID string, since time.Time) ([]types.AuditEventRecord, error) {
	return nil, nil
}

func (s testStorage) GetSnapshotsInRange(clusterID string, startTime, endTime time.Time) ([]types.SnapshotRecord, error) {
	return nil, nil
}

func (s testStorage) GetSettings(clusterID string) (*types.ClusterSettings, error) {
	return nil, nil
}

func (s testStorage) UpdateSettings(clusterID string, settings *types.ClusterSettings) error {
	return nil
}

func (s testStorage) GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error) {
	return nil, nil
}

func TestAdjustResourcesBuildsExpectedPatches(t *testing.T) {
	deps := HandlerDependencies{
		Storage: testStorage{
			statFn: func(clusterID, workloadID string) (*types.WorkloadStat, error) {
				if clusterID != "cluster-a" {
					t.Fatalf("unexpected clusterID: %s", clusterID)
				}
				if workloadID != "Deployment:default:api" {
					t.Fatalf("unexpected workloadID: %s", workloadID)
				}
				return testWorkloadStat(), nil
			},
		},
		Config: testHandlerConfig(false),
	}

	patches, err := deps.adjustResources(context.Background(), testPod(), "cluster-a")
	if err != nil {
		t.Fatalf("adjustResources returned error: %v", err)
	}

	assertHasPatch(t, patches, "remove", "/spec/containers/0/resources/limits/cpu", nil)
	assertHasPatch(t, patches, "replace", "/spec/containers/0/resources/requests/cpu", "300m")
	assertHasPatch(t, patches, "replace", "/spec/containers/0/resources/requests/memory", "200M")
	assertHasPatch(t, patches, "replace", "/spec/containers/0/resources/limits/memory", "512M")
}

func TestAdjustResourcesSkipsMemoryPatchesWhenDisabled(t *testing.T) {
	deps := HandlerDependencies{
		Storage: testStorage{
			statFn: func(clusterID, workloadID string) (*types.WorkloadStat, error) {
				return testWorkloadStat(), nil
			},
		},
		Config: testHandlerConfig(true),
	}

	patches, err := deps.adjustResources(context.Background(), testPod(), "cluster-a")
	if err != nil {
		t.Fatalf("adjustResources returned error: %v", err)
	}

	assertHasPatch(t, patches, "remove", "/spec/containers/0/resources/limits/cpu", nil)
	assertHasPatch(t, patches, "replace", "/spec/containers/0/resources/requests/cpu", "300m")
	assertNoPatchAtPath(t, patches, "/spec/containers/0/resources/requests/memory")
	assertNoPatchAtPath(t, patches, "/spec/containers/0/resources/limits/memory")
}

func TestBuildDisruptionAnnotationPatchesRemovesAnnotationsAndMarksModified(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-pod",
			Annotations: map[string]string{
				"karpenter.sh/do-not-disrupt": utils.TrueValue,
			},
		},
	}

	patches := buildDisruptionAnnotationPatches(context.Background(), pod, &types.WorkloadStat{
		Constraints: &types.WorkloadConstraints{
			DoNotDisruptAnnotation: true,
		},
	}, &types.Overrides{
		DisruptionWindows: []types.DisruptionWindow{activeDisruptionWindow()},
	})

	assertHasPatch(t, patches, "remove", "/metadata/annotations/karpenter.sh~1do-not-disrupt", nil)
	assertHasPatch(t, patches, "add", "/metadata/annotations/"+escapeJSONPointer(utils.AnnotationModified), utils.TrueValue)
}

func testHandlerConfig(disableMemory bool) *config.Config {
	return &config.Config{
		RecommendationSettings: config.RecommendationSettings{
			DisableMemoryApplication: disableMemory,
		},
		Controller: config.ControllerConfig{
			Tasks: map[string]*config.TaskConfig{
				config.ApplyRecommendationKey: {
					Enabled:  true,
					Metadata: map[string]interface{}{},
				},
			},
		},
	}
}

func testPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-pod",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "api-abcdef",
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("300M"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("600M"),
						},
					},
				},
			},
		},
	}
}

func testWorkloadStat() *types.WorkloadStat {
	return &types.WorkloadStat{
		ContainerStats: []types.ContainerStats{
			{
				ContainerName: "app",
				CPUStats: &types.CPUStats{
					Max: 0.25,
				},
				MemoryStats: &types.MemoryStats{
					Max: 150,
				},
				SimplePredictionsCPU: &types.SimplePrediction{
					MaxValue: 0.3,
				},
				SimplePredictionsMemory: &types.SimplePrediction{
					MaxValue: 200,
				},
			},
		},
	}
}

func activeDisruptionWindow() types.DisruptionWindow {
	now := time.Now().UTC()
	start := now.Add(-1 * time.Minute)
	end := now.Add(1 * time.Minute)

	return types.DisruptionWindow{
		StartCron: fmt.Sprintf("%d %d * * *", start.Minute(), start.Hour()),
		EndCron:   fmt.Sprintf("%d %d * * *", end.Minute(), end.Hour()),
	}
}

func assertHasPatch(t *testing.T, patches []map[string]any, op, path string, value any) {
	t.Helper()

	for _, patch := range patches {
		if patch["op"] != op || patch["path"] != path {
			continue
		}
		if value == nil {
			if _, exists := patch["value"]; exists {
				t.Fatalf("patch %s %s unexpectedly had value %v", op, path, patch["value"])
			}
			return
		}
		if patch["value"] != value {
			t.Fatalf("patch %s %s had value %v, want %v", op, path, patch["value"], value)
		}
		return
	}

	t.Fatalf("missing patch %s %s", op, path)
}

func assertNoPatchAtPath(t *testing.T, patches []map[string]any, path string) {
	t.Helper()

	for _, patch := range patches {
		if patch["path"] == path {
			t.Fatalf("unexpected patch at %s: %#v", path, patch)
		}
	}
}
