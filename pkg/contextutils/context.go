package contextutils

import (
	"context"
)

type contextKey string

const (
	TaskContextKey      contextKey = "task"
	ClusterContextKey   contextKey = "cluster"
	APIContextKey       contextKey = "api"
	NamespaceContextKey contextKey = "namespace"
	QueryIDContextKey   contextKey = "query"
)

func WithTask(ctx context.Context, taskName string) context.Context {
	return context.WithValue(ctx, TaskContextKey, taskName)
}

func WithCluster(ctx context.Context, clusterID string) context.Context {
	return context.WithValue(ctx, ClusterContextKey, clusterID)
}

func WithAPI(ctx context.Context, api string) context.Context {
	return context.WithValue(ctx, APIContextKey, api)
}

func WithNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, NamespaceContextKey, namespace)
}

func WithQueryID(ctx context.Context, queryID string) context.Context {
	return context.WithValue(ctx, QueryIDContextKey, queryID)
}

func GetTask(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(TaskContextKey).(string)
	return val, ok && val != ""
}

func GetCluster(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(ClusterContextKey).(string)
	return val, ok && val != ""
}

func GetAPI(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(APIContextKey).(string)
	return val, ok && val != ""
}

func GetKey(ctx context.Context, key contextKey) (string, bool) {
	val, ok := ctx.Value(key).(string)
	return val, ok && val != ""
}

func GetAttributes(ctx context.Context) map[string]string {
	attrs := make(map[string]string, 3)
	for _, key := range []contextKey{TaskContextKey, ClusterContextKey, APIContextKey, NamespaceContextKey, QueryIDContextKey} {
		if v, ok := GetKey(ctx, key); ok {
			attrs[string(key)] = v
		}
	}
	return attrs
}
