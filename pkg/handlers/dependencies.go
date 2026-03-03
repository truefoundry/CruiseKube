package handlers

import (
	"context"

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
