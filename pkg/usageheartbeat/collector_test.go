package usageheartbeat

import (
	"context"
	"testing"

	"github.com/truefoundry/cruisekube/pkg/config"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestCollect_k8sVersionAndHelm(t *testing.T) {
	t.Parallel()
	kube := fakeclientset.NewSimpleClientset()
	fd, ok := kube.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatal("Discovery() is not *fakediscovery.FakeDiscovery")
	}
	fd.FakedServerVersion = &version.Info{Major: "1", Minor: "28"}

	cfg := &config.Config{
		UsageTelemetry: config.UsageTelemetryConfig{
			HelmChartVersion: "chart-1",
		},
	}
	hb, err := Collect(context.Background(), kube, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if hb.K8sMajor != "1" || hb.K8sMinor != "28" {
		t.Fatalf("k8s version: major=%q minor=%q", hb.K8sMajor, hb.K8sMinor)
	}
	if hb.HelmChartVersion != "chart-1" {
		t.Fatalf("helm chart version: %q", hb.HelmChartVersion)
	}
}
