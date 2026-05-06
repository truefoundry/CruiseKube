package usageheartbeat

import (
	"context"
	"fmt"
	"strings"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/ports"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Collect gathers anonymous cluster metadata for a usage heartbeat.
func Collect(ctx context.Context, kube kubernetes.Interface, cfg *config.Config) (ports.UsageHeartbeat, error) {
	var hb ports.UsageHeartbeat
	hb.HelmChartVersion = strings.TrimSpace(cfg.UsageTelemetry.HelmChartVersion)
	hb.ControllerMode = string(cfg.ControllerMode)
	hb.TargetNamespaceSet = strings.TrimSpace(cfg.Controller.TargetNamespace) != ""

	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return hb, fmt.Errorf("list nodes: %w", err)
	}
	hb.NodeTotal = len(nodes.Items)
	for _, n := range nodes.Items {
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				hb.NodeReady++
				break
			}
		}
	}

	sv, err := kube.Discovery().ServerVersion()
	if err != nil {
		return hb, fmt.Errorf("server version: %w", err)
	}
	hb.K8sMajor = sv.Major
	hb.K8sMinor = sv.Minor

	return hb, nil
}
