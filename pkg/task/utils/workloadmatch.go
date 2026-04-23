package utils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// MatchWorkloadBySelector returns the first Deployment/StatefulSet/DaemonSet in cache whose selector matches the pod.
func MatchWorkloadBySelector(pod *corev1.Pod, workloadCache []WorkloadLabelSelectorList) (WorkloadInfo, bool) {
	if pod == nil {
		return WorkloadInfo{}, false
	}
	podLabels := labels.Set(pod.Labels)
	for _, workload := range workloadCache {
		if workload.Namespace == pod.Namespace && workload.Selector.Matches(podLabels) {
			return WorkloadInfo{
				Kind:      workload.Kind,
				Namespace: workload.Namespace,
				Name:      workload.Name,
			}, true
		}
	}
	return WorkloadInfo{}, false
}
