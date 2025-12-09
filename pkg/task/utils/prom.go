package utils

import (
	"fmt"
	"strings"
	"time"
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

func BuildBatchOOMMemoryExpression(namespace string) string {
	podInfo := buildBatchPodInfoExpression(namespace)

	template := `max by (created_by_kind, created_by_name, namespace, container) (
		(
			sum by (namespace, pod, container, node) (
				kube_pod_container_resource_limits{job="kube-state-metrics",namespace="%s"}
			)
			* on (namespace, pod, container) group_right(node)
			(
				sum by (namespace, pod, container) (
					kube_pod_container_status_last_terminated_reason{job="kube-state-metrics",namespace="%s",reason="OOMKilled"}
					* on (namespace, pod, container)
					sum by (namespace, pod, container) (
						increase(
							kube_pod_container_status_restarts_total{job="kube-state-metrics",namespace="%s"}[1m]
						)
					)
				)
				>
				bool 0
			)
		)
		* on (namespace, pod, node) group_left(created_by_kind, created_by_name)
		(%s)
	)`

	return fmt.Sprintf(template, namespace, namespace, namespace, podInfo)
}
