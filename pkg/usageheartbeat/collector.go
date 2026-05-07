package usageheartbeat

import (
	"context"
	"fmt"
	"strings"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/ports"
	"k8s.io/client-go/kubernetes"
)

// Collect gathers anonymous cluster metadata for a usage heartbeat.
func Collect(ctx context.Context, kube kubernetes.Interface, cfg *config.Config) (ports.UsageHeartbeat, error) {
	var hb ports.UsageHeartbeat
	hb.HelmChartVersion = strings.TrimSpace(cfg.UsageTelemetry.HelmChartVersion)

	sv, err := kube.Discovery().ServerVersion()
	if err != nil {
		return hb, fmt.Errorf("server version: %w", err)
	}
	hb.K8sMajor = sv.Major
	hb.K8sMinor = sv.Minor

	return hb, nil
}
