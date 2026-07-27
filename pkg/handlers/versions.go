package handlers

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/buildmetadata"
	"github.com/truefoundry/cruisekube/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
)

const (
	// CruiseKube requires Kubernetes 1.33+ so that pod in-place resource updates
	// (in-place vertical scaling) are available — that feature is beta / enabled
	// by default from 1.33. It is a kubelet/node-level feature, so both the
	// per-node kubelet version and the server (control-plane) version must meet
	// the 1.33 floor. PSI (pressure stall information) metrics are supported but
	// optional: their absence does not block CruiseKube (see the "psi" metric
	// group in preflight.go).

	// defaultMinKubeVersion is the minimum per-node kubelet version.
	defaultMinKubeVersion = "v1.33.0"
	// defaultMinKubernetesVersion is the minimum Kubernetes server (control-plane)
	// version.
	defaultMinKubernetesVersion = "v1.33.0"
	// defaultMinPrometheusVersion is the minimum Prometheus version CruiseKube
	// requires.
	defaultMinPrometheusVersion = "2.30.0"
)

// NodeVersionInfo describes the version of a single node in the cluster and
// whether it satisfies the required minimum kubelet version.
type NodeVersionInfo struct {
	Name             string `json:"name"`
	KubeletVersion   string `json:"kubelet_version"`
	KubeProxyVersion string `json:"kube_proxy_version"`
	OSImage          string `json:"os_image"`
	ContainerRuntime string `json:"container_runtime"`
	Kernel           string `json:"kernel_version"`
	Architecture     string `json:"architecture"`
	MeetsMinimum     bool   `json:"meets_minimum"`
	Error            string `json:"error,omitempty"`
}

// PrometheusVersionInfo describes the detected Prometheus version and whether it
// satisfies the required minimum Prometheus version.
type PrometheusVersionInfo struct {
	Version      string `json:"version"`
	MeetsMinimum bool   `json:"meets_minimum"`
	Error        string `json:"error,omitempty"`
}

// KubernetesVersionInfo describes the detected Kubernetes server (control-plane)
// version and whether it satisfies the required minimum.
type KubernetesVersionInfo struct {
	Version      string `json:"version"`
	MeetsMinimum bool   `json:"meets_minimum"`
	Error        string `json:"error,omitempty"`
}

// VersionReport is the version-check payload embedded in the preflight response.
// The Prometheus field is filled in by the caller (from Prometheus buildinfo, or
// from the prometheus_build_info metric as a fallback).
type VersionReport struct {
	Passed               bool                  `json:"passed"`
	CruisekubeVersion    string                `json:"cruisekube_version"`
	MinKubeVersion       string                `json:"min_kube_version"`
	MinKubernetesVersion string                `json:"min_kubernetes_version"`
	MinPrometheusVersion string                `json:"min_prometheus_version"`
	Kubernetes           KubernetesVersionInfo `json:"kubernetes"`
	Nodes                []NodeVersionInfo     `json:"nodes"`
	NodeCount            int                   `json:"node_count"`
	NodesBelowMinimum    int                   `json:"nodes_below_minimum"`
	NodeError            string                `json:"node_error,omitempty"`
	Prometheus           PrometheusVersionInfo `json:"prometheus"`
}

// finalize recomputes Passed from the Kubernetes server, node, and Prometheus
// results. Call it after the caller has populated the Prometheus field.
func (r *VersionReport) finalize() {
	r.Passed = r.NodeError == "" &&
		r.NodesBelowMinimum == 0 &&
		r.Kubernetes.MeetsMinimum &&
		r.Prometheus.MeetsMinimum
}

// collectNodeVersions lists the cluster's nodes and evaluates each node's
// kubelet version against minKubeVer. It returns the per-node info, the number
// of nodes below the minimum, and an error only when the node list itself could
// not be retrieved.
func collectNodeVersions(ctx context.Context, kubeClient kubernetes.Interface, minKubeVer *version.Version) ([]NodeVersionInfo, int, error) {
	if kubeClient == nil {
		return nil, 0, fmt.Errorf("kubernetes client is not configured for this cluster")
	}
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list nodes: %w", err)
	}
	infos := make([]NodeVersionInfo, 0, len(nodes.Items))
	below := 0
	for i := range nodes.Items {
		node := &nodes.Items[i]
		info := node.Status.NodeInfo
		ni := NodeVersionInfo{
			Name:             node.Name,
			KubeletVersion:   info.KubeletVersion,
			KubeProxyVersion: info.KubeProxyVersion,
			OSImage:          info.OSImage,
			ContainerRuntime: info.ContainerRuntimeVersion,
			Kernel:           info.KernelVersion,
			Architecture:     info.Architecture,
		}
		kubeletVer, perr := version.ParseGeneric(info.KubeletVersion)
		if perr != nil {
			ni.Error = fmt.Sprintf("could not parse kubelet version %q: %v", info.KubeletVersion, perr)
		} else {
			ni.MeetsMinimum = kubeletVer.AtLeast(minKubeVer)
		}
		if !ni.MeetsMinimum {
			below++
		}
		infos = append(infos, ni)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, below, nil
}

// detectKubernetesServerVersion queries the cluster's discovery endpoint for the
// Kubernetes server (control-plane) version and checks it against minVer.
func detectKubernetesServerVersion(kubeClient kubernetes.Interface, minVer *version.Version) KubernetesVersionInfo {
	if kubeClient == nil {
		return KubernetesVersionInfo{Error: "kubernetes client is not configured for this cluster"}
	}
	info, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		return KubernetesVersionInfo{Error: fmt.Sprintf("failed to get kubernetes server version: %v", err)}
	}
	if info.GitVersion == "" {
		return KubernetesVersionInfo{Error: "kubernetes server version is empty"}
	}
	k8sVer, perr := version.ParseGeneric(info.GitVersion)
	if perr != nil {
		return KubernetesVersionInfo{Version: info.GitVersion, Error: fmt.Sprintf("could not parse kubernetes version %q: %v", info.GitVersion, perr)}
	}
	return KubernetesVersionInfo{Version: info.GitVersion, MeetsMinimum: k8sVer.AtLeast(minVer)}
}

// buildVersionReport populates the Kubernetes server version and the node portion
// of a VersionReport. The caller is responsible for setting the Prometheus field
// and then calling finalize().
func buildVersionReport(ctx context.Context, kubeClient kubernetes.Interface, minKubeVer, minK8sVer, minPromVer *version.Version) VersionReport {
	report := VersionReport{
		CruisekubeVersion:    buildmetadata.Version,
		MinKubeVersion:       minKubeVer.String(),
		MinKubernetesVersion: minK8sVer.String(),
		MinPrometheusVersion: minPromVer.String(),
		Nodes:                []NodeVersionInfo{},
	}
	report.Kubernetes = detectKubernetesServerVersion(kubeClient, minK8sVer)
	nodes, below, err := collectNodeVersions(ctx, kubeClient, minKubeVer)
	if err != nil {
		report.NodeError = err.Error()
		return report
	}
	report.Nodes = nodes
	report.NodeCount = len(nodes)
	report.NodesBelowMinimum = below
	return report
}

// evaluatePrometheusVersion compares a known Prometheus version string against
// the required minimum.
func evaluatePrometheusVersion(versionStr string, minVer *version.Version) PrometheusVersionInfo {
	info := PrometheusVersionInfo{Version: versionStr}
	if versionStr == "" {
		info.Error = "prometheus version could not be determined"
		return info
	}
	promVer, err := version.ParseGeneric(versionStr)
	if err != nil {
		info.Error = fmt.Sprintf("could not parse prometheus version %q: %v", versionStr, err)
		return info
	}
	info.MeetsMinimum = promVer.AtLeast(minVer)
	return info
}

// detectPrometheusVersion queries the prometheus_build_info metric to determine
// the running Prometheus version and checks it against the supplied minimum.
// Used as a fallback when the Prometheus buildinfo API is unavailable.
func detectPrometheusVersion(ctx context.Context, client promAPI, minVer *version.Version) PrometheusVersionInfo {
	if client == nil {
		return PrometheusVersionInfo{Error: "prometheus client is not configured for this cluster"}
	}

	result, _, err := client.Query(ctx, "prometheus_build_info", time.Now())
	if err != nil {
		logging.Errorf(ctx, "Failed to query prometheus_build_info: %v", err)
		return PrometheusVersionInfo{Error: fmt.Sprintf("failed to query prometheus_build_info: %v", err)}
	}

	vec, ok := result.(model.Vector)
	if !ok || len(vec) == 0 {
		return PrometheusVersionInfo{Error: "prometheus_build_info returned no samples; version could not be determined"}
	}

	versionStr := string(vec[0].Metric["version"])
	if versionStr == "" {
		return PrometheusVersionInfo{Error: "prometheus_build_info sample did not contain a version label"}
	}
	return evaluatePrometheusVersion(versionStr, minVer)
}
