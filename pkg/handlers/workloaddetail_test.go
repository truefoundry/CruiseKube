package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/types"
)

type workloadDetailTestStorage struct {
	workloadsFn func(clusterID string) ([]*types.WorkloadInCluster, error)
	recsFn      func(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error)
}

func (s workloadDetailTestStorage) ClusterStatsExists(clusterID string) (bool, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) ReadClusterStats(clusterID string, target *types.StatsResponse) error {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetWorkloadOverrides(clusterID, workloadID string) (*types.Overrides, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetWorkloadsInCluster(clusterID string) ([]*types.WorkloadInCluster, error) {
	return s.workloadsFn(clusterID)
}

func (s workloadDetailTestStorage) GetPodRecommendationsForWorkload(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error) {
	return s.recsFn(clusterID, workloadID)
}

func (s workloadDetailTestStorage) GetAllStatsForCluster(clusterID string) ([]types.WorkloadStat, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) UpdateWorkloadOverrides(clusterID, workloadID string, overrides *types.Overrides) error {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) BatchUpdateWorkloadOverrides(clusterID string, workloadIDs []string, overrides *types.Overrides) ([]string, []string, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetAuditEvents(clusterID string, since time.Time) ([]types.AuditEventRecord, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetAuditEventsForWorkload(clusterID, workloadID string, since time.Time) ([]types.AuditEventRecord, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetSnapshotsInRange(clusterID string, startTime, endTime time.Time) ([]types.SnapshotRecord, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetSettings(clusterID string) (*types.ClusterSettings, error) {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) UpdateSettings(clusterID string, settings *types.ClusterSettings) error {
	panic("unexpected call")
}

func (s workloadDetailTestStorage) GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error) {
	panic("unexpected call")
}

func TestHandleWorkloadDetailIncludesCurrentPodAverages(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stat := &types.WorkloadStat{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "api",
		Replicas:  2,
		OriginalContainerResources: []types.OriginalContainerResources{
			{Name: "main", Type: types.AppContainer, CPURequest: 1, CPULimit: 2, MemoryRequest: 512, MemoryLimit: 1024},
		},
	}

	deps := HandlerDependencies{
		Storage: workloadDetailTestStorage{
			workloadsFn: func(clusterID string) ([]*types.WorkloadInCluster, error) {
				return []*types.WorkloadInCluster{
					{
						ClusterID:  clusterID,
						WorkloadID: "Deployment:default:api",
						Stat:       stat,
					},
				}, nil
			},
			recsFn: func(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error) {
				return []types.PodResourceRecommendationRow{
					{
						WorkloadID:     workloadID,
						Namespace:      "default",
						Pod:            "api-0",
						Container:      "main",
						Recommendation: `{"cpu_request":0.5,"memory_request":256}`,
						Current:        `{"cpu_request":0.8,"memory_request":400}`,
					},
					{
						WorkloadID:     workloadID,
						Namespace:      "default",
						Pod:            "api-1",
						Container:      "main",
						Recommendation: `{"cpu_request":0.6,"memory_request":300}`,
						Current:        `{"cpu_request":1.0,"memory_request":500}`,
					},
				}, nil
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "default"},
		{Key: "namespace", Value: "default"},
		{Key: "workloadName", Value: "api"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/default/workloads/default/api/detail", nil)
	c.Request = req

	deps.HandleWorkloadDetail(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp types.WorkloadDetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.CurrentPodAvgCPU != 0.9 {
		t.Fatalf("expected current pod avg cpu 0.9, got %v", resp.CurrentPodAvgCPU)
	}
	if resp.CurrentPodAvgMemory != 450 {
		t.Fatalf("expected current pod avg memory 450, got %v", resp.CurrentPodAvgMemory)
	}
}
