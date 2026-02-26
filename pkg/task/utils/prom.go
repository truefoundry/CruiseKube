package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/logging"
)

func CompressQueryForLogging(query string) string {
	compressed := strings.Fields(query)
	return strings.Join(compressed, " ")
}

func EncloseWithinQuantileOverTime(query string, quantileLookbackWindow time.Duration, percentile float64) string {
	template := `quantile_over_time(%.2f, (%s)[%ds:1m])`
	return fmt.Sprintf(template, percentile, query, int(quantileLookbackWindow.Seconds()))
}

func BuildBatchCoreCPUExpression(namespace string, psiAdjusted bool) string {
	throttlingAwareCPU := buildBatchThrottlingAwareCPUExpression(namespace, psiAdjusted)

	template := `(max by (created_by_kind, created_by_name, namespace, container) (%s) or vector(0))`

	return fmt.Sprintf(template, throttlingAwareCPU)
}

func buildBatchThrottlingAwareCPUExpression(namespace string, psiAdjusted bool) string {
	// throttlingRatio := buildBatchThrottlingRatioExpression(namespace)
	cpuUsage := buildBatchCPUUsageExpression(namespace, psiAdjusted)
	podInfo := buildBatchPodInfoExpression(namespace)
	// throttledStartupFilter := fmt.Sprintf(`and on (namespace, pod, container) ((time() - kube_pod_container_state_started{job="kube-state-metrics", namespace="%s", container!~""}) >= %d)`, namespace, CPUThrottledLookbackWindow * 60)
	// throttledStartupFilter := ""
	template := `(
		(
			max by (created_by_kind, created_by_name, namespace, container, node) (
				(%s)
				* on (namespace, pod, node) group_left(created_by_kind, created_by_name)
				(%s)
			) or vector(0)
		)
	)`

	return fmt.Sprintf(template,
		cpuUsage, podInfo,
	)
}

func buildBatchCPUUsageExpression(namespace string, psiAdjusted bool) string {
	psiAdjustedQuery := ""
	if psiAdjusted {
		psiAdjustedExpression := `
		* on (namespace, pod, container, node) (1 + max by (namespace, pod, container, node) (
			rate(container_pressure_cpu_waiting_seconds_total{container!~"",job="kubelet",namespace="%s"}[%dm])
		))
		`
		psiAdjustedQuery = fmt.Sprintf(psiAdjustedExpression, namespace, RateIntervalMinutes)
	}
	template := `max by (namespace, pod, container, node) (
		rate(container_cpu_usage_seconds_total{container!~"",job="kubelet",namespace="%s"}[%dm])
	)
	%s
	`
	return fmt.Sprintf(template, namespace, RateIntervalMinutes, psiAdjustedQuery)
}

func buildBatchPodInfoExpression(namespace string) string {
	template := `max by (namespace, pod, container, node, created_by_kind, created_by_name) (
		kube_pod_info{
			job="kube-state-metrics",
			namespace="%s"
		}
	)`
	return fmt.Sprintf(template, namespace)
}

func BuildBatchMemoryUsageExpression(namespace string) string {
	podInfo := buildBatchPodInfoExpression(namespace)

	template := `max by (created_by_kind, created_by_name, namespace, container) (
      container_memory_working_set_bytes{
        job="kubelet",
        namespace="%s",
        container!~""
      }
      * on (namespace, pod, node) group_left(created_by_kind, created_by_name)
      (%s)
    )`

	return fmt.Sprintf(template, namespace, podInfo)
}

func BuildClusterMemoryUtilizationExpression() string {
	template := `round(
      sum(
        sum by (node) (
          node_memory_MemTotal_bytes{job="node-exporter"} - (node_memory_MemFree_bytes{job="node-exporter"} + node_memory_Buffers_bytes{job="node-exporter"} + node_memory_Cached_bytes{job="node-exporter"})
        )
        unless
        max by (node) (
          max_over_time(kube_node_status_allocatable{job="kube-state-metrics", resource=~"nvidia_com_gpu|amd_com_gpu"}[7d:]) > 0
        )
      )
      / 1000000000,
      0.001
    )`
	return template
}

func BuildClusterMemoryRequestExpression() string {
	template := `round(
      sum(
        sum by (node) (
          (
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource="memory"})
            )
            unless on (namespace, pod)
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource=~"nvidia_com_gpu|amd_com_gpu"})
            )
          )
          * on (namespace, pod) group_left
            sum by (namespace, pod) (kube_pod_status_phase{job="kube-state-metrics", phase!~"Failed|Succeeded|Unknown|Pending"})
        )
        unless on (node)
        (
          max by (node) (
            max_over_time(
              kube_node_status_allocatable{job="kube-state-metrics", resource=~"nvidia_com_gpu|amd_com_gpu"}[7d:]
            )
          )
          >
          0
        )
      ) / 1000000000,
      0.001
    )`
	return template
}

func BuildClusterMemoryAllocatableExpression() string {
	template := `round(
      sum(
        sum by (node) (kube_node_status_allocatable{job="kube-state-metrics", resource="memory"})
        unless (
          sum by (node) (kube_node_spec_taint{job="kube-state-metrics", key="nvidia.com/gpu"})
        )
        unless on (node) (
          kube_node_labels{job="kube-state-metrics", accelerator="nvidia"}
        )
      ) / 1000000000,
      0.001
    )`
	return template
}

func BuildClusterCPUUtilizationExpression() string {
	template := `round(
      sum(
        sum by (node) (
          rate(node_cpu_seconds_total{job="node-exporter", mode=~"user|system"}[1m])
        )
        unless max by (node) (
          max_over_time(kube_node_status_allocatable{
            job="kube-state-metrics",
            resource=~"nvidia_com_gpu|amd_com_gpu"
          }[7d:]) > 0
        )
      ),
      0.001
    )`
	return template
}

func BuildClusterCPURequestExpression() string {
	template := `round(
      sum(
        sum by (node) (
          (
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource="cpu"})
            )
            unless on (namespace, pod)
            (
              sum by (namespace, pod) (kube_pod_container_resource_requests{job="kube-state-metrics", container!="", resource=~"nvidia_com_gpu|amd_com_gpu"})
            )
          )
          * on (namespace, pod) group_left
            sum by (namespace, pod) (kube_pod_status_phase{job="kube-state-metrics", phase!~"Failed|Succeeded|Unknown|Pending"})
        )
        unless on (node)
        (
          max by (node) (
            max_over_time(
              kube_node_status_allocatable{job="kube-state-metrics", resource=~"nvidia_com_gpu|amd_com_gpu"}[7d:]
            )
          )
          >
          0
        )
      ),
      0.001
    )`
	return template
}

func BuildClusterCPUAllocatableExpression() string {
	template := `round(
      sum(
        sum by (node) (kube_node_status_allocatable{job="kube-state-metrics", resource="cpu"})
        unless (
          sum by (node) (
            kube_node_spec_taint{job="kube-state-metrics", key="nvidia.com/gpu"}
          )
        )
        unless on (node) (
          kube_node_labels{job="kube-state-metrics", accelerator="nvidia"}
        )
      ),
      0.001
    )`
	return template
}

func QueryAndParsePrometheusScalar(ctx context.Context, client v1.API, q string) float64 {
	if client == nil {
		logging.Errorf(ctx, "Prometheus client is nil for query: %s", q)
		return 0
	}
	result, _, err := client.Query(ctx, q, time.Now())
	if err != nil {
		logging.Errorf(ctx, "Failed to query Prometheus scalar: %v for query: %s", err, q)
		return 0
	}
	if result == nil {
		logging.Errorf(ctx, "Prometheus result is nil for query: %s", q)
		return 0
	}
	if v, ok := result.(model.Vector); ok && len(v) > 0 {
		return float64(v[0].Value)
	}
	if s, ok := result.(*model.Scalar); ok {
		return float64(s.Value)
	}
	return 0
}
