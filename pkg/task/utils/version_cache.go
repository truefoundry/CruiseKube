package utils

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"golang.org/x/sync/singleflight"
	"k8s.io/client-go/kubernetes"
)

type versionCacheEntry struct {
	major int
	minor int
}

// versionResult holds the result of a singleflight fetch for use by callers.
type versionResult struct {
	major int
	minor int
}

var (
	versionCache      sync.Map
	versionFetchGroup singleflight.Group
)

// CheckIfClusterVersionAbove returns true if the cluster server version is >= targetMajor.targetMinor.
// Results are cached per clusterID (no expiration) to avoid repeated Discovery().ServerVersion() calls.
// Safe for concurrent use.
func CheckIfClusterVersionAbove(ctx context.Context, clusterID string, kubeClient *kubernetes.Clientset, targetMajor, targetMinor int) bool {
	if kubeClient == nil {
		return false
	}
	if v, ok := versionCache.Load(clusterID); ok {
		entry := v.(versionCacheEntry)
		return isVersionAtLeast(entry.major, entry.minor, targetMajor, targetMinor)
	}

	v, err, _ := versionFetchGroup.Do(clusterID, func() (interface{}, error) {
		major, minor, err := fetchServerVersion(ctx, kubeClient)
		if err != nil {
			return nil, err
		}
		versionCache.Store(clusterID, versionCacheEntry{major: major, minor: minor})
		return &versionResult{major: major, minor: minor}, nil
	})
	if err != nil {
		return false
	}
	res := v.(*versionResult)
	return isVersionAtLeast(res.major, res.minor, targetMajor, targetMinor)
}

func isVersionAtLeast(major, minor, targetMajor, targetMinor int) bool {
	return major > targetMajor || (major == targetMajor && minor >= targetMinor)
}

func fetchServerVersion(ctx context.Context, kubeClient *kubernetes.Clientset) (int, int, error) {
	version, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		logging.Errorf(ctx, "[version cache] Error getting cluster version: %v", err)
		return 0, 0, fmt.Errorf("get server version: %w", err)
	}
	gitVersion := strings.TrimPrefix(version.GitVersion, "v")
	parts := strings.Split(gitVersion, ".")
	if len(parts) < 2 {
		logging.Errorf(ctx, "[version cache] Invalid version format: %s", version.GitVersion)
		return 0, 0, errors.New("invalid version format")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		logging.Errorf(ctx, "[version cache] Error parsing major version: %v", err)
		return 0, 0, fmt.Errorf("parse major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		logging.Errorf(ctx, "[version cache] Error parsing minor version: %v", err)
		return 0, 0, fmt.Errorf("parse minor version %q: %w", parts[1], err)
	}
	return major, minor, nil
}
