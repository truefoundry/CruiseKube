package utils

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"

	"k8s.io/client-go/kubernetes"
)

const defaultVersionCacheTTL = 24 * time.Hour

type versionCacheEntry struct {
	major     int
	minor     int
	expiresAt time.Time
}

var (
	versionCache   = make(map[string]versionCacheEntry)
	versionCacheMu sync.Mutex
)

// CheckIfClusterVersionAbove returns true if the cluster server version is >= targetMajor.targetMinor.
// Results are cached per clusterID with a TTL (default 10 minutes) to avoid repeated Discovery().ServerVersion() calls.
// Safe for concurrent use.
func CheckIfClusterVersionAbove(ctx context.Context, clusterID string, kubeClient *kubernetes.Clientset, targetMajor, targetMinor int) bool {
	if kubeClient == nil {
		return false
	}
	versionCacheMu.Lock()
	entry, ok := versionCache[clusterID]
	if ok && time.Now().Before(entry.expiresAt) {
		versionCacheMu.Unlock()
		return isVersionAtLeast(entry.major, entry.minor, targetMajor, targetMinor)
	}
	versionCacheMu.Unlock()

	major, minor, err := fetchServerVersion(ctx, kubeClient)
	if err != nil {
		return false
	}

	versionCacheMu.Lock()
	versionCache[clusterID] = versionCacheEntry{major: major, minor: minor, expiresAt: time.Now().Add(defaultVersionCacheTTL)}
	versionCacheMu.Unlock()

	return isVersionAtLeast(major, minor, targetMajor, targetMinor)
}

func isVersionAtLeast(major, minor, targetMajor, targetMinor int) bool {
	return major > targetMajor || (major == targetMajor && minor >= targetMinor)
}

func fetchServerVersion(ctx context.Context, kubeClient *kubernetes.Clientset) (major, minor int, err error) {
	version, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		logging.Errorf(ctx, "[version cache] Error getting cluster version: %v", err)
		return 0, 0, err
	}
	gitVersion := strings.TrimPrefix(version.GitVersion, "v")
	parts := strings.Split(gitVersion, ".")
	if len(parts) < 2 {
		logging.Errorf(ctx, "[version cache] Invalid version format: %s", version.GitVersion)
		return 0, 0, errors.New("invalid version format")
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		logging.Errorf(ctx, "[version cache] Error parsing major version: %v", err)
		return 0, 0, err
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		logging.Errorf(ctx, "[version cache] Error parsing minor version: %v", err)
		return 0, 0, err
	}
	return major, minor, nil
}
