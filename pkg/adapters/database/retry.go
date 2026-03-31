package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"github.com/truefoundry/cruisekube/pkg/types"
)

// retryingDatabase wraps GormDB and retries transient database errors on every ports.Database call.
type retryingDatabase struct {
	inner *GormDB
}

func newRetryingDatabase(inner *GormDB) *retryingDatabase {
	return &retryingDatabase{inner: inner}
}

func dbRetryOpts() []backoff.RetryOption {
	return []backoff.RetryOption{
		backoff.WithMaxElapsedTime(30 * time.Second),
		backoff.WithMaxTries(12),
		backoff.WithNotify(func(err error, d time.Duration) {
			logging.Debugf(context.Background(), "database retry in %s: %v", d, err)
		}),
	}
}

func isTransientDBError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	if pgconn.Timeout(err) || pgconn.SafeToRetry(err) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40001", "40P01", "55P03", "08000", "08003", "08006", "08001", "57P01", "53300":
			return true
		}
	}
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) {
		switch sqlErr.Code {
		case sqlite3.ErrBusy, sqlite3.ErrLocked:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "server closed the connection") ||
		strings.Contains(msg, "bad connection") ||
		strings.Contains(msg, "eof") {
		return true
	}
	return false
}

func withDBRetry[T any](ctx context.Context, op func() (T, error)) (T, error) {
	v, err := backoff.Retry(ctx, func() (T, error) {
		v, err := op()
		if err == nil {
			return v, nil
		}
		if isTransientDBError(err) {
			return v, fmt.Errorf("database retry: %w", err)
		}
		return v, backoff.Permanent(err)
	}, dbRetryOpts()...)
	if err != nil {
		return v, fmt.Errorf("database retries exhausted: %w", err)
	}
	return v, nil
}

func withDBRetryVoid(ctx context.Context, op func() error) error {
	_, err := withDBRetry(ctx, func() (struct{}, error) {
		if err := op(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (r *retryingDatabase) retryErr(op func() error) error {
	return withDBRetryVoid(context.Background(), op)
}

func dbRetryVal[T any](op func() (T, error)) (T, error) {
	return withDBRetry(context.Background(), op)
}

func (r *retryingDatabase) Close() error {
	return r.inner.Close()
}

func (r *retryingDatabase) UpsertStat(clusterID, workloadID string, stat types.WorkloadStat, generatedAt time.Time) error {
	return r.retryErr(func() error { return r.inner.UpsertStat(clusterID, workloadID, stat, generatedAt) })
}

func (r *retryingDatabase) HasRecentStat(clusterID, workloadID string, withinMinutes int) (bool, error) {
	return dbRetryVal(func() (bool, error) { return r.inner.HasRecentStat(clusterID, workloadID, withinMinutes) })
}

func (r *retryingDatabase) HasCluster(clusterID string) (bool, error) {
	return dbRetryVal(func() (bool, error) { return r.inner.HasCluster(clusterID) })
}

func (r *retryingDatabase) HasWorkloadForCluster(clusterID, workloadID string) (bool, error) {
	return dbRetryVal(func() (bool, error) { return r.inner.HasWorkloadForCluster(clusterID, workloadID) })
}

func (r *retryingDatabase) GetStatsForCluster(clusterID string) ([]types.WorkloadStat, error) {
	return dbRetryVal(func() ([]types.WorkloadStat, error) { return r.inner.GetStatsForCluster(clusterID) })
}

func (r *retryingDatabase) GetWorkloadsInCluster(clusterID string) ([]*types.WorkloadInCluster, error) {
	return dbRetryVal(func() ([]*types.WorkloadInCluster, error) { return r.inner.GetWorkloadsInCluster(clusterID) })
}

func (r *retryingDatabase) GetStatForWorkload(clusterID, workloadID string) (*types.WorkloadStat, error) {
	return dbRetryVal(func() (*types.WorkloadStat, error) { return r.inner.GetStatForWorkload(clusterID, workloadID) })
}

func (r *retryingDatabase) GetStatCountForCluster(clusterID string) (int, error) {
	return dbRetryVal(func() (int, error) { return r.inner.GetStatCountForCluster(clusterID) })
}

func (r *retryingDatabase) GetStatOverridesForWorkload(clusterID, workloadID string) (*types.Overrides, error) {
	return dbRetryVal(func() (*types.Overrides, error) { return r.inner.GetStatOverridesForWorkload(clusterID, workloadID) })
}

func (r *retryingDatabase) DeleteWorkloadsForCluster(clusterID string) error {
	return r.retryErr(func() error { return r.inner.DeleteWorkloadsForCluster(clusterID) })
}

func (r *retryingDatabase) DeleteWorkload(clusterID, workloadID string) error {
	return r.retryErr(func() error { return r.inner.DeleteWorkload(clusterID, workloadID) })
}

func (r *retryingDatabase) DeleteWorkloadsNotInCluster(clusterID string, keepIDs []string) (int, error) {
	return dbRetryVal(func() (int, error) { return r.inner.DeleteWorkloadsNotInCluster(clusterID, keepIDs) })
}

func (r *retryingDatabase) UpdateStatOverridesForWorkload(clusterID, workloadID string, overrides *types.Overrides) error {
	return r.retryErr(func() error { return r.inner.UpdateStatOverridesForWorkload(clusterID, workloadID, overrides) })
}

func (r *retryingDatabase) BatchUpdateStatOverridesForWorkloads(clusterID string, workloadIDs []string, overrides *types.Overrides) ([]string, error) {
	return dbRetryVal(func() ([]string, error) {
		return r.inner.BatchUpdateStatOverridesForWorkloads(clusterID, workloadIDs, overrides)
	})
}

func (r *retryingDatabase) InsertOOMEvent(event *types.OOMEvent) error {
	return r.retryErr(func() error { return r.inner.InsertOOMEvent(event) })
}

func (r *retryingDatabase) GetOOMEventsByWorkload(clusterID, workloadID string, since time.Time) ([]types.OOMEvent, error) {
	return dbRetryVal(func() ([]types.OOMEvent, error) { return r.inner.GetOOMEventsByWorkload(clusterID, workloadID, since) })
}

func (r *retryingDatabase) GetLatestOOMEventForContainer(clusterID, containerID, podName string) (*types.OOMEvent, error) {
	return dbRetryVal(func() (*types.OOMEvent, error) {
		return r.inner.GetLatestOOMEventForContainer(clusterID, containerID, podName)
	})
}

func (r *retryingDatabase) DeleteOldOOMEvents(clusterID string, olderThan time.Time) (int64, error) {
	return dbRetryVal(func() (int64, error) { return r.inner.DeleteOldOOMEvents(clusterID, olderThan) })
}

func (r *retryingDatabase) SavePodRecommendations(clusterID string, rows []types.PodResourceRecommendationRow) error {
	return r.retryErr(func() error { return r.inner.SavePodRecommendations(clusterID, rows) })
}

func (r *retryingDatabase) GetPodRecommendationsForCluster(clusterID string) ([]types.PodResourceRecommendationRow, error) {
	return dbRetryVal(func() ([]types.PodResourceRecommendationRow, error) {
		return r.inner.GetPodRecommendationsForCluster(clusterID)
	})
}

func (r *retryingDatabase) GetPodRecommendationsForWorkload(clusterID, workloadID string) ([]types.PodResourceRecommendationRow, error) {
	return dbRetryVal(func() ([]types.PodResourceRecommendationRow, error) {
		return r.inner.GetPodRecommendationsForWorkload(clusterID, workloadID)
	})
}

func (r *retryingDatabase) InsertAuditEvent(clusterID string, event types.AuditEvent) error {
	return r.retryErr(func() error { return r.inner.InsertAuditEvent(clusterID, event) })
}

func (r *retryingDatabase) GetAuditEvents(clusterID string, since time.Time) ([]types.AuditEventRecord, error) {
	return dbRetryVal(func() ([]types.AuditEventRecord, error) { return r.inner.GetAuditEvents(clusterID, since) })
}

func (r *retryingDatabase) GetAuditEventsForWorkload(clusterID, workloadID string, since time.Time) ([]types.AuditEventRecord, error) {
	return dbRetryVal(func() ([]types.AuditEventRecord, error) {
		return r.inner.GetAuditEventsForWorkload(clusterID, workloadID, since)
	})
}

func (r *retryingDatabase) DeleteOldAuditEvents(clusterID string, olderThan time.Time) (int64, error) {
	return dbRetryVal(func() (int64, error) { return r.inner.DeleteOldAuditEvents(clusterID, olderThan) })
}

func (r *retryingDatabase) InsertSnapshot(snapshot *types.SnapshotPayload) error {
	return r.retryErr(func() error { return r.inner.InsertSnapshot(snapshot) })
}

func (r *retryingDatabase) GetSnapshotsInRange(clusterID string, startTime, endTime time.Time) ([]types.SnapshotRecord, error) {
	return dbRetryVal(func() ([]types.SnapshotRecord, error) {
		return r.inner.GetSnapshotsInRange(clusterID, startTime, endTime)
	})
}

func (r *retryingDatabase) DeleteOldSnapshots(clusterID string, olderThan time.Time) (int64, error) {
	return dbRetryVal(func() (int64, error) { return r.inner.DeleteOldSnapshots(clusterID, olderThan) })
}

func (r *retryingDatabase) GetClusterSettings(clusterID string) (*types.ClusterSettings, error) {
	return dbRetryVal(func() (*types.ClusterSettings, error) { return r.inner.GetClusterSettings(clusterID) })
}

func (r *retryingDatabase) UpdateClusterSettings(clusterID string, settings *types.ClusterSettings) error {
	return r.retryErr(func() error { return r.inner.UpdateClusterSettings(clusterID, settings) })
}

var _ ports.Database = (*retryingDatabase)(nil)
