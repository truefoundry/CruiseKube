package usageheartbeat

import (
	"context"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCollect_nodeCounts(t *testing.T) {
	t.Parallel()
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "a"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "b"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}},
			},
		},
	}
	kube := fake.NewSimpleClientset(&nodes[0], &nodes[1])
	cfg := &config.Config{
		ControllerMode: config.ClusterModeInCluster,
		Controller: config.ControllerConfig{
			TargetNamespace: "prod",
		},
		UsageTelemetry: config.UsageTelemetryConfig{
			HelmChartVersion: "chart-1",
		},
	}
	hb, err := Collect(context.Background(), kube, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if hb.NodeTotal != 2 || hb.NodeReady != 1 {
		t.Fatalf("nodes: total=%d ready=%d", hb.NodeTotal, hb.NodeReady)
	}
	if !hb.TargetNamespaceSet || hb.HelmChartVersion != "chart-1" {
		t.Fatalf("metadata: %#v", hb)
	}
}
