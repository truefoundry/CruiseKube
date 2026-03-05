package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/truefoundry/cruisekube/pkg/client"
	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/types"
)

type storageReader interface {
	ClusterStatsExists(clusterID string) (bool, error)
	ReadClusterStats(clusterID string, target *types.StatsResponse) error
	GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error)
	GetWorkloadOverrides(clusterID, workloadID string) (*types.Overrides, error)
	GetWorkloadsInCluster(clusterID string) ([]*types.WorkloadInCluster, error)
	GetPodRecommendationsForWorkload(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error)
	GetAllStatsForCluster(clusterID string) ([]types.WorkloadStat, error)
	UpdateWorkloadOverrides(clusterID, workloadID string, overrides *types.Overrides) error
	GetAuditEvents(clusterID string, since time.Time) ([]types.AuditEventRecord, error)
	GetAuditEventsForWorkload(clusterID, workloadID string, since time.Time) ([]types.AuditEventRecord, error)
	GetSnapshotsInRange(clusterID string, startTime, endTime time.Time) ([]types.SnapshotRecord, error)
	GetSettings(clusterID string) (*types.ClusterSettings, error)
	UpdateSettings(clusterID string, settings *types.ClusterSettings) error
	GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error)
}

type auditRecorder interface {
	Record(ctx context.Context, clusterID string, event types.AuditEvent)
}

type recommenderClient interface {
	WebhookMutatingPatch(ctx context.Context, clusterID string, body client.MutatingPatchRequest) ([]client.JSONPatchOp, error)
}

type HandlerDependencies struct {
	Storage           storageReader
	AuditRecorder     auditRecorder
	ClusterManager    cluster.Manager
	Config            *config.Config
	RecommenderClient recommenderClient
}

func NewHandlerDependencies(
	storage storageReader,
	clusterManager cluster.Manager,
	cfg *config.Config,
	audit auditRecorder,
	recommender recommenderClient,
) (HandlerDependencies, error) {
	if storage == nil {
		return HandlerDependencies{}, fmt.Errorf("storage is required")
	}
	if clusterManager == nil {
		return HandlerDependencies{}, fmt.Errorf("cluster manager is required")
	}
	if cfg == nil {
		return HandlerDependencies{}, fmt.Errorf("config is required")
	}

	return HandlerDependencies{
		Storage:           storage,
		AuditRecorder:     audit,
		ClusterManager:    clusterManager,
		Config:            cfg,
		RecommenderClient: recommender,
	}, nil
}

func NewWebhookHandlerDependencies(cfg *config.Config, recommender recommenderClient) (HandlerDependencies, error) {
	if cfg == nil {
		return HandlerDependencies{}, fmt.Errorf("config is required")
	}
	if recommender == nil {
		return HandlerDependencies{}, fmt.Errorf("recommender client is required")
	}

	return HandlerDependencies{
		Config:            cfg,
		RecommenderClient: recommender,
	}, nil
}
