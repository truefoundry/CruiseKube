package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// UnscheduledPodsNodeName is a synthetic node key for pods with no spec.nodeName (e.g. unscheduled Pending).
// It must not be a valid Kubernetes node name.
const UnscheduledPodsNodeName = "__cruisekube_unscheduled__"

const maxOwnerResolveDepth = 12

// WorkloadKindSupportedForOptimization returns true for workload kinds Cruisekube can mutate today.
func WorkloadKindSupportedForOptimization(kind string) bool {
	switch kind {
	case DeploymentKind, StatefulSetKind, DaemonSetKind:
		return true
	default:
		return false
	}
}

// ResolveRootWorkloadFromPod walks controller owner references (with API lookups) to a stable root workload.
// WorkloadInfo.Kind is the API kind of that root (e.g. CronJob, Deployment, Job).
// Returns ok=false when the pod has no controller (bare pod / system pod without owner).
func ResolveRootWorkloadFromPod(ctx context.Context, kube kubernetes.Interface, pod *corev1.Pod) (WorkloadInfo, bool) {
	if pod == nil {
		return WorkloadInfo{}, false
	}
	ns := pod.Namespace
	cur := metav1.GetControllerOf(pod)
	if cur == nil {
		return WorkloadInfo{}, false
	}

	for depth := 0; depth < maxOwnerResolveDepth; depth++ {
		kind := cur.Kind
		name := cur.Name

		switch kind {
		case StatefulSetKind, DaemonSetKind:
			return WorkloadInfo{Kind: kind, Namespace: ns, Name: name}, true

		case DeploymentKind:
			return WorkloadInfo{Kind: DeploymentKind, Namespace: ns, Name: name}, true

		case ReplicaSetKind:
			rs, err := kube.AppsV1().ReplicaSets(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					if deployName, ok := ExtractWorkloadFromReplicaSet(name); ok {
						return WorkloadInfo{Kind: DeploymentKind, Namespace: ns, Name: deployName}, true
					}
				}
				return WorkloadInfo{Kind: ReplicaSetKind, Namespace: ns, Name: name}, true
			}
			if parent := metav1.GetControllerOf(rs); parent != nil && parent.Kind == DeploymentKind {
				cur = parent
				continue
			}
			return WorkloadInfo{Kind: ReplicaSetKind, Namespace: rs.Namespace, Name: rs.Name}, true

		case JobKind:
			job, err := kube.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return WorkloadInfo{Kind: JobKind, Namespace: ns, Name: name}, true
			}
			if parent := metav1.GetControllerOf(job); parent != nil && parent.Kind == CronJobKind {
				return WorkloadInfo{Kind: CronJobKind, Namespace: job.Namespace, Name: parent.Name}, true
			}
			return WorkloadInfo{Kind: JobKind, Namespace: job.Namespace, Name: job.Name}, true

		case CronJobKind:
			return WorkloadInfo{Kind: CronJobKind, Namespace: ns, Name: name}, true

		case ReplicationControllerKind:
			return WorkloadInfo{Kind: ReplicationControllerKind, Namespace: ns, Name: name}, true

		default:
			return WorkloadInfo{Kind: kind, Namespace: ns, Name: name}, true
		}
	}

	return WorkloadInfo{Kind: cur.Kind, Namespace: ns, Name: cur.Name}, true
}
