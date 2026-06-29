package utils

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/types"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// mergeContainers returns workload containers first, then appends any pod containers
// not already present (by name). Preserves workload order and capacity semantics.
// If podContainers is empty, returns workload as-is. If workload is empty, returns
// a copy of podContainers.
func mergeContainers(workloadContainers, podContainers []corev1.Container) []corev1.Container {
	if len(podContainers) == 0 {
		return workloadContainers
	}
	workloadNames := make(map[string]struct{}, len(workloadContainers))
	for _, c := range workloadContainers {
		workloadNames[c.Name] = struct{}{}
	}
	merged := make([]corev1.Container, len(workloadContainers), len(workloadContainers)+len(podContainers))
	copy(merged, workloadContainers)
	for _, c := range podContainers {
		if _, ok := workloadNames[c.Name]; !ok {
			merged = append(merged, c)
			workloadNames[c.Name] = struct{}{}
		}
	}
	return merged
}

// WorkloadObject represents any Kubernetes workload that can be managed
type WorkloadObject interface {
	GetKind() string
	GetNamespace() string
	GetName() string
	GetContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container
	GetInitContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container
	GetSelector() (labels.Selector, error)
	GetCreationTime() time.Time
	GetReplicas() int32
	GetWorkloadInfo() WorkloadInfo
}

// DeploymentWrapper wraps appsv1.Deployment to implement WorkloadObject
type DeploymentWrapper struct {
	*appsv1.Deployment
}

func (d DeploymentWrapper) GetKind() string {
	return DeploymentKind
}

func (d DeploymentWrapper) GetInitContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container {
	workloadContainers := d.Spec.Template.Spec.InitContainers
	selector, err := d.GetSelector()
	if err != nil {
		logging.Errorf(ctx, "Error getting selector for deployment %s/%s: %v", d.Namespace, d.Name, err)
		return workloadContainers
	}

	// getting fresh pods as dynamically injected containers are not tracked in workload spec
	pods := GetMatchingPodsFromPodCache(ctx, podCache, d.Namespace, selector)
	podContainers := []corev1.Container{}
	if len(pods) > 0 {
		podContainers = pods[0].Spec.InitContainers
	}
	return mergeContainers(workloadContainers, podContainers)
}

// GetContainerSpecs returns workload container specs plus any from the pod not in the spec (e.g. dynamically injected).
func (d DeploymentWrapper) GetContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container {
	containers := d.Spec.Template.Spec.Containers
	selector, err := d.GetSelector()
	if err != nil {
		logging.Errorf(ctx, "Error getting selector for deployment %s/%s: %v", d.Namespace, d.Name, err)
		return containers
	}

	// getting pods as dynamically injected containers might not be tracked in workload spec
	pods := GetMatchingPodsFromPodCache(ctx, podCache, d.Namespace, selector)
	podContainers := []corev1.Container{}
	if len(pods) > 0 {
		podContainers = pods[0].Spec.Containers
	}
	return mergeContainers(containers, podContainers)
}

func (d DeploymentWrapper) GetSelector() (labels.Selector, error) {
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("LabelSelectorAsSelector failed for selector %v: %w", d.Spec.Selector, err)
	}
	return selector, nil
}

func (d DeploymentWrapper) GetCreationTime() time.Time {
	return d.CreationTimestamp.Time
}

func (d DeploymentWrapper) GetReplicas() int32 {
	if d.Spec.Replicas == nil {
		return 1
	}
	return *d.Spec.Replicas
}

func (d DeploymentWrapper) GetWorkloadInfo() WorkloadInfo {
	return WorkloadInfo{
		Kind:      DeploymentKind,
		Namespace: d.Namespace,
		Name:      d.Name,
	}
}

// StatefulSetWrapper wraps appsv1.StatefulSet to implement WorkloadObject
type StatefulSetWrapper struct {
	*appsv1.StatefulSet
}

func (s StatefulSetWrapper) GetKind() string {
	return StatefulSetKind
}

func (s StatefulSetWrapper) GetInitContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container {
	workloadContainers := s.Spec.Template.Spec.InitContainers
	selector, err := s.GetSelector()
	if err != nil {
		logging.Errorf(ctx, "Error getting selector for statefulset %s/%s: %v", s.Namespace, s.Name, err)
		return workloadContainers
	}

	// getting fresh pods as dynamically injected containers are not tracked in workload spec
	pods := GetMatchingPodsFromPodCache(ctx, podCache, s.Namespace, selector)
	podContainers := []corev1.Container{}
	if len(pods) > 0 {
		podContainers = pods[0].Spec.InitContainers
	}

	return mergeContainers(workloadContainers, podContainers)
}

// GetContainerSpecs returns workload container specs plus any from the pod not in the spec (e.g. dynamically injected).
func (s StatefulSetWrapper) GetContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container {
	containers := s.Spec.Template.Spec.Containers
	selector, err := s.GetSelector()
	if err != nil {
		logging.Errorf(ctx, "Error getting selector for statefulset %s/%s: %v", s.Namespace, s.Name, err)
		return containers
	}

	// getting pods as dynamically injected containers might not be tracked in workload spec
	pods := GetMatchingPodsFromPodCache(ctx, podCache, s.Namespace, selector)
	podContainers := []corev1.Container{}
	if len(pods) > 0 {
		podContainers = pods[0].Spec.Containers
	}
	return mergeContainers(containers, podContainers)
}

func (s StatefulSetWrapper) GetSelector() (labels.Selector, error) {
	selector, err := metav1.LabelSelectorAsSelector(s.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("LabelSelectorAsSelector failed for selector %v: %w", s.Spec.Selector, err)
	}
	return selector, nil
}

func (s StatefulSetWrapper) GetCreationTime() time.Time {
	return s.CreationTimestamp.Time
}

func (s StatefulSetWrapper) GetReplicas() int32 {
	if s.Spec.Replicas == nil {
		return 1
	}
	return *s.Spec.Replicas
}

func (s StatefulSetWrapper) GetWorkloadInfo() WorkloadInfo {
	return WorkloadInfo{
		Kind:      StatefulSetKind,
		Namespace: s.Namespace,
		Name:      s.Name,
	}
}

// DaemonSetWrapper wraps appsv1.DaemonSet to implement WorkloadObject
type DaemonSetWrapper struct {
	*appsv1.DaemonSet
}

func (d DaemonSetWrapper) GetKind() string {
	return DaemonSetKind
}

func (d DaemonSetWrapper) GetInitContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container {
	workloadContainers := d.Spec.Template.Spec.InitContainers
	selector, err := d.GetSelector()
	if err != nil {
		logging.Errorf(ctx, "Error getting selector for daemonset %s/%s: %v", d.Namespace, d.Name, err)
		return workloadContainers
	}

	// getting fresh pods as dynamically injected containers are not tracked in workload spec
	pods := GetMatchingPodsFromPodCache(ctx, podCache, d.Namespace, selector)
	podContainers := []corev1.Container{}
	if len(pods) > 0 {
		podContainers = pods[0].Spec.InitContainers
	}

	return mergeContainers(workloadContainers, podContainers)
}

// GetContainerSpecs returns workload container specs plus any from the pod not in the spec (e.g. dynamically injected).
func (d DaemonSetWrapper) GetContainerSpecs(ctx context.Context, podCache map[string][]corev1.Pod) []corev1.Container {
	containers := d.Spec.Template.Spec.Containers
	selector, err := d.GetSelector()
	if err != nil {
		logging.Errorf(ctx, "Error getting selector for daemonset %s/%s: %v", d.Namespace, d.Name, err)
		return containers
	}

	// getting pods as dynamically injected containers might not be tracked in workload spec
	pods := GetMatchingPodsFromPodCache(ctx, podCache, d.Namespace, selector)
	podContainers := []corev1.Container{}
	if len(pods) > 0 {
		podContainers = pods[0].Spec.Containers
	}
	return mergeContainers(containers, podContainers)
}

func (d DaemonSetWrapper) GetSelector() (labels.Selector, error) {
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("LabelSelectorAsSelector failed for selector %v: %w", d.Spec.Selector, err)
	}
	return selector, nil
}

func (d DaemonSetWrapper) GetCreationTime() time.Time {
	return d.CreationTimestamp.Time
}

func (d DaemonSetWrapper) GetReplicas() int32 {
	return d.Status.DesiredNumberScheduled
}

func (d DaemonSetWrapper) GetWorkloadInfo() WorkloadInfo {
	return WorkloadInfo{
		Kind:      DaemonSetKind,
		Namespace: d.Namespace,
		Name:      d.Name,
	}
}

// GetWorkloadObject retrieves a workload object by kind, namespace, and name
func GetWorkloadObject(ctx context.Context, kubeClient *kubernetes.Clientset, kind, namespace, name string) (WorkloadObject, error) {
	switch kind {
	case DeploymentKind:
		deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting deployment %s/%s: %w", namespace, name, err)
		}
		return DeploymentWrapper{deployment}, nil

	case StatefulSetKind:
		statefulSet, err := kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting statefulset %s/%s: %w", namespace, name, err)
		}
		return StatefulSetWrapper{statefulSet}, nil

	case DaemonSetKind:
		daemonSet, err := kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting daemonset %s/%s: %w", namespace, name, err)
		}
		return DaemonSetWrapper{daemonSet}, nil

	default:
		return nil, fmt.Errorf("unsupported workload kind: %s", kind)
	}
}

// ListAllWorkloadObjects lists all workloads of all supported types in a namespace
func ListAllWorkloadObjects(ctx context.Context, kubeClient *kubernetes.Clientset, targetNamespace string) ([]WorkloadObject, error) {
	var workloadObjects []WorkloadObject

	// List Deployments
	deployments, err := kubeClient.AppsV1().Deployments(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	for _, deployment := range deployments.Items {
		if deployment.Spec.Selector != nil {
			workloadObjects = append(workloadObjects, DeploymentWrapper{&deployment})
		}
	}

	// List StatefulSets
	statefulSets, err := kubeClient.AppsV1().StatefulSets(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list statefulsets: %w", err)
	}
	for _, statefulSet := range statefulSets.Items {
		if statefulSet.Spec.Selector != nil {
			workloadObjects = append(workloadObjects, StatefulSetWrapper{&statefulSet})
		}
	}

	// List DaemonSets
	daemonSets, err := kubeClient.AppsV1().DaemonSets(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list daemonsets: %w", err)
	}
	for _, daemonSet := range daemonSets.Items {
		if daemonSet.Spec.Selector != nil {
			workloadObjects = append(workloadObjects, DaemonSetWrapper{&daemonSet})
		}
	}

	return workloadObjects, nil
}

// ListAllWorkloadsWithSelectors lists all workloads with their label selectors
func ListAllWorkloadsWithSelectors(ctx context.Context, kubeClient *kubernetes.Clientset, targetNamespace string) ([]WorkloadLabelSelectorList, error) {
	var workloads []WorkloadLabelSelectorList

	// List Deployments
	deployments, err := kubeClient.AppsV1().Deployments(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Could not list deployments: %v", err)
	} else {
		for _, deployment := range deployments.Items {
			if deployment.Spec.Selector != nil {
				selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
				if err != nil {
					logging.Errorf(ctx, "Invalid selector for deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
					continue
				}
				workloads = append(workloads, WorkloadLabelSelectorList{
					Kind:      DeploymentKind,
					Namespace: deployment.Namespace,
					Name:      deployment.Name,
					Selector:  selector,
				})
			}
		}
	}

	// List StatefulSets
	statefulSets, err := kubeClient.AppsV1().StatefulSets(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Could not list statefulsets: %v", err)
	} else {
		for _, statefulSet := range statefulSets.Items {
			if statefulSet.Spec.Selector != nil {
				selector, err := metav1.LabelSelectorAsSelector(statefulSet.Spec.Selector)
				if err != nil {
					logging.Errorf(ctx, "Invalid selector for statefulset %s/%s: %v", statefulSet.Namespace, statefulSet.Name, err)
					continue
				}
				workloads = append(workloads, WorkloadLabelSelectorList{
					Kind:      StatefulSetKind,
					Namespace: statefulSet.Namespace,
					Name:      statefulSet.Name,
					Selector:  selector,
				})
			}
		}
	}

	// List DaemonSets
	daemonSets, err := kubeClient.AppsV1().DaemonSets(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Could not list daemonsets: %v", err)
	} else {
		for _, daemonSet := range daemonSets.Items {
			if daemonSet.Spec.Selector != nil {
				selector, err := metav1.LabelSelectorAsSelector(daemonSet.Spec.Selector)
				if err != nil {
					logging.Errorf(ctx, "Invalid selector for daemonset %s/%s: %v", daemonSet.Namespace, daemonSet.Name, err)
					continue
				}
				workloads = append(workloads, WorkloadLabelSelectorList{
					Kind:      DaemonSetKind,
					Namespace: daemonSet.Namespace,
					Name:      daemonSet.Name,
					Selector:  selector,
				})
			}
		}
	}

	return workloads, nil
}

// ExtractUniqueNamespaces extracts all unique namespaces from a workload map
func ExtractUniqueNamespaces(workloads map[string]WorkloadObject) []string {
	namespaceSet := make(map[string]bool)
	for _, workloadObject := range workloads {
		if workloadObject.GetNamespace() != "" {
			namespaceSet[workloadObject.GetNamespace()] = true
		}
	}

	namespaces := make([]string, 0, len(namespaceSet))
	for namespace := range namespaceSet {
		namespaces = append(namespaces, namespace)
	}

	return namespaces
}

// CollectHPAInfo lists HorizontalPodAutoscalers (once) and returns, keyed by
// workload key, the HPA that scales each workload together with its per-resource
// metric targets. Workloads without a scaling HPA are simply absent from the map.
//
// The returned info is used both to mark workloads as horizontally autoscaled
// and to coordinate vertical right-sizing with the HPA's target utilization.
func CollectHPAInfo(ctx context.Context, dynamicClient dynamic.Interface, targetNamespace string) (map[string]*types.HPAInfo, error) {
	hpaGVR := schema.GroupVersionResource{
		Group:    "autoscaling",
		Version:  "v2",
		Resource: "horizontalpodautoscalers",
	}

	var hpaList *unstructured.UnstructuredList
	var err error
	if targetNamespace == "" {
		hpaList, err = dynamicClient.Resource(hpaGVR).List(ctx, metav1.ListOptions{})
	} else {
		hpaList, err = dynamicClient.Resource(hpaGVR).Namespace(targetNamespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		logging.Errorf(ctx, "Could not list HPAs: %v", err)
		return nil, fmt.Errorf("failed to list HPAs: %w", err)
	}

	result := make(map[string]*types.HPAInfo)
	for _, hpaUnstructured := range hpaList.Items {
		var hpa autoscalingv2.HorizontalPodAutoscaler
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(hpaUnstructured.Object, &hpa); err != nil {
			logging.Errorf(ctx, "Could not convert HPA %s/%s to structured object: %v",
				hpaUnstructured.GetNamespace(), hpaUnstructured.GetName(), err)
			continue
		}

		targetKey := hpaTargetWorkloadKey(hpa)
		if targetKey == "" {
			continue
		}

		info := &types.HPAInfo{
			Name:        hpa.Name,
			Namespace:   hpa.Namespace,
			MaxReplicas: hpa.Spec.MaxReplicas,
		}
		if hpa.Spec.MinReplicas != nil {
			info.MinReplicas = *hpa.Spec.MinReplicas
		}

		for _, metric := range hpa.Spec.Metrics {
			if metric.Type != autoscalingv2.ResourceMetricSourceType || metric.Resource == nil {
				continue
			}
			target := hpaResourceTarget(metric.Resource.Target)
			// Only CPU/memory resource metrics couple with vertical right-sizing.
			switch metric.Resource.Name { //nolint:exhaustive // only CPU and memory are relevant here
			case corev1.ResourceCPU:
				info.CPU = target
			case corev1.ResourceMemory:
				info.Memory = target
			}
		}

		if info.CPU == nil && info.Memory == nil {
			// HPA scales only on custom/external metrics; no coexistence conflict.
			continue
		}
		result[targetKey] = info
	}

	return result, nil
}

// UpdateHPATargetUtilizations sets the target averageUtilization of the named
// HPA's Resource metrics to the desired values (one per resource). Only metrics
// whose target type is Utilization are touched. It returns whether the HPA was
// actually changed. A no-op (already at the desired values) returns (false, nil).
func UpdateHPATargetUtilizations(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string, desired map[corev1.ResourceName]int32) (bool, error) {
	hpaGVR := schema.GroupVersionResource{
		Group:    "autoscaling",
		Version:  "v2",
		Resource: "horizontalpodautoscalers",
	}

	obj, err := dynamicClient.Resource(hpaGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get HPA %s/%s: %w", namespace, name, err)
	}

	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &hpa); err != nil {
		return false, fmt.Errorf("failed to convert HPA %s/%s: %w", namespace, name, err)
	}

	changed := false
	for i := range hpa.Spec.Metrics {
		metric := &hpa.Spec.Metrics[i]
		if metric.Type != autoscalingv2.ResourceMetricSourceType || metric.Resource == nil {
			continue
		}
		want, ok := desired[metric.Resource.Name]
		if !ok || metric.Resource.Target.Type != autoscalingv2.UtilizationMetricType {
			continue
		}
		if metric.Resource.Target.AverageUtilization == nil || *metric.Resource.Target.AverageUtilization != want {
			value := want
			metric.Resource.Target.AverageUtilization = &value
			changed = true
		}
	}

	if !changed {
		return false, nil
	}

	updated, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&hpa)
	if err != nil {
		return false, fmt.Errorf("failed to marshal HPA %s/%s: %w", namespace, name, err)
	}
	obj.Object = updated

	if _, err := dynamicClient.Resource(hpaGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
		return false, fmt.Errorf("failed to update HPA %s/%s: %w", namespace, name, err)
	}
	return true, nil
}

func hpaResourceTarget(target autoscalingv2.MetricTarget) *types.HPAResourceTarget {
	rt := &types.HPAResourceTarget{MetricType: string(target.Type)}
	if target.AverageUtilization != nil {
		rt.AverageUtilization = *target.AverageUtilization
	}
	if target.AverageValue != nil {
		rt.AverageValue = target.AverageValue.String()
	}
	return rt
}

// hpaTargetWorkloadKey returns the workload key the HPA scales, or "" if it
// cannot be resolved. Argo Rollouts are treated as Deployments for workload
// management purposes since they extend Kubernetes Deployment functionality.
func hpaTargetWorkloadKey(hpa autoscalingv2.HorizontalPodAutoscaler) string {
	hpaTargetName := hpa.Spec.ScaleTargetRef.Name
	hpaTargetKind := hpa.Spec.ScaleTargetRef.Kind
	hpaTargetAPIVersion := hpa.Spec.ScaleTargetRef.APIVersion

	if hpaTargetKind == RolloutKind && hpaTargetAPIVersion == "argoproj.io/v1alpha1" {
		hpaTargetKind = DeploymentKind
	}

	if hpaTargetName == "" || hpaTargetKind == "" {
		return ""
	}
	return GetWorkloadKey(hpaTargetKind, hpa.Namespace, hpaTargetName)
}

func DetectWorkloadConstraints(ctx context.Context, kubeClient *kubernetes.Clientset, dynamicClient dynamic.Interface, workloadObj WorkloadObject, pdbCache map[string][]policyv1.PodDisruptionBudget) (*WorkloadConstraints, error) {
	constraints := &WorkloadConstraints{}
	if _, ok := workloadObj.(DaemonSetWrapper); ok {
		constraints.BlockingConsolidation = false
		return constraints, nil
	}

	constraints.PDB = checkWorkloadAgainstPDBs(ctx, workloadObj, pdbCache)

	podTemplate := GetPodTemplateSpec(workloadObj)
	if podTemplate != nil {
		constraints.DoNotDisruptAnnotation = checkDoNotDisruptAnnotation(podTemplate)
		constraints.Volume = checkVolumes(podTemplate)
		constraints.Affinity = checkUncommonAffinity(podTemplate)
		constraints.TopologySpreadConstraint = checkTopologySpreadConstraints(podTemplate)
		constraints.PodAntiAffinity = checkPodAntiAffinity(podTemplate)
		constraints.ExcludedAnnotation = PodExcludedByAnnotation(podTemplate)
	}

	constraints.BlockingConsolidation =
		constraints.PDB ||
			constraints.DoNotDisruptAnnotation

	return constraints, nil
}

func FetchPDBsForNamespaces(ctx context.Context, kubeClient *kubernetes.Clientset, namespaces []string) (map[string][]policyv1.PodDisruptionBudget, error) {
	pdbCache := make(map[string][]policyv1.PodDisruptionBudget)
	failedNamespaces := make([]string, 0)

	for _, namespace := range namespaces {
		pdbList, err := kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logging.Warnf(ctx, "Error listing PodDisruptionBudgets for namespace %s: %v", namespace, err)
			failedNamespaces = append(failedNamespaces, namespace)
			continue
		}
		pdbCache[namespace] = pdbList.Items
	}
	if len(failedNamespaces) > 0 {
		return pdbCache, fmt.Errorf("failed to prefetch pdbs for namespaces: %s", strings.Join(failedNamespaces, ", "))
	}

	return pdbCache, nil
}

func FindMatchingPDBs(ctx context.Context, podLabels labels.Set, pdbs []policyv1.PodDisruptionBudget) []*policyv1.PodDisruptionBudget {
	var matching []*policyv1.PodDisruptionBudget
	for i := range pdbs {
		pdb := &pdbs[i]
		if pdb.Spec.Selector == nil {
			continue
		}
		pdbSelector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			logging.Errorf(ctx, "Error parsing PDB selector for %s/%s: %v", pdb.Namespace, pdb.Name, err)
			continue
		}
		if pdbSelector.Matches(podLabels) {
			matching = append(matching, pdb)
		}
	}
	return matching
}

func checkWorkloadAgainstPDBs(ctx context.Context, workloadObj WorkloadObject, pdbCache map[string][]policyv1.PodDisruptionBudget) bool {
	podTemplate := GetPodTemplateSpec(workloadObj)
	if podTemplate == nil {
		return false
	}
	podLabels := labels.Set(podTemplate.Labels)

	for _, pdb := range pdbCache[workloadObj.GetNamespace()] {
		if pdb.Spec.Selector == nil {
			continue
		}

		pdbSelector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			logging.Errorf(ctx, "Error parsing PDB selector for %s: %v", pdb.Name, err)
			continue
		}

		if pdbSelector.Matches(podLabels) {
			return true
		}
	}

	return false
}

func FetchPodsForNamespaces(ctx context.Context, kubeClient *kubernetes.Clientset, namespaces []string) (map[string][]corev1.Pod, error) {
	podCache := make(map[string][]corev1.Pod)
	failedNamespaces := make([]string, 0)

	for _, namespace := range namespaces {
		podList, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logging.Warnf(ctx, "Error listing Pods for namespace %s: %v", namespace, err)
			failedNamespaces = append(failedNamespaces, namespace)
			continue
		}
		podCache[namespace] = podList.Items
	}
	if len(failedNamespaces) > 0 {
		return podCache, fmt.Errorf("failed to prefetch pods for namespaces: %s", strings.Join(failedNamespaces, ", "))
	}

	return podCache, nil
}

func GetPodTemplateSpec(workloadObj WorkloadObject) *corev1.PodTemplateSpec {
	switch w := workloadObj.(type) {
	case DeploymentWrapper:
		return &w.Spec.Template
	case StatefulSetWrapper:
		return &w.Spec.Template
	case DaemonSetWrapper:
		return &w.Spec.Template
	default:
		return nil
	}
}

func checkDoNotDisruptAnnotation(podTemplate *corev1.PodTemplateSpec) bool {
	if podTemplate.Annotations == nil {
		return false
	}

	for annotationKey, expectedValue := range GetDoNotDisruptAnnotations() {
		if value, exists := podTemplate.Annotations[annotationKey]; exists && strings.ToLower(value) == expectedValue {
			return true
		}
	}
	return false
}

func checkVolumes(podTemplate *corev1.PodTemplateSpec) bool {
	if len(podTemplate.Spec.Volumes) == 0 {
		return false
	}

	for _, volume := range podTemplate.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil ||
			volume.HostPath != nil ||
			volume.NFS != nil ||
			volume.Glusterfs != nil ||
			volume.RBD != nil ||
			volume.CephFS != nil ||
			volume.Cinder != nil ||
			volume.FC != nil ||
			volume.FlexVolume != nil ||
			volume.Flocker != nil ||
			volume.AWSElasticBlockStore != nil ||
			volume.GCEPersistentDisk != nil ||
			volume.AzureDisk != nil ||
			volume.AzureFile != nil ||
			volume.VsphereVolume != nil ||
			volume.Quobyte != nil ||
			volume.ISCSI != nil ||
			volume.PhotonPersistentDisk != nil ||
			volume.PortworxVolume != nil ||
			volume.ScaleIO != nil ||
			volume.StorageOS != nil ||
			volume.CSI != nil {
			return true
		}
	}

	return false
}

func checkUncommonAffinity(podTemplate *corev1.PodTemplateSpec) bool {
	if podTemplate.Spec.Affinity == nil {
		return false
	}

	affinity := podTemplate.Spec.Affinity

	if affinity.NodeAffinity != nil {
		if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for _, term := range affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				for _, expr := range term.MatchExpressions {
					if !isCommonNodeAffinityKey(expr.Key) {
						return true
					}
				}
			}
		}
	}

	return false
}

func isCommonNodeAffinityKey(key string) bool {
	commonKeys := []string{
		"kubernetes.io/arch",
		"kubernetes.io/os",
		"topology.kubernetes.io/region",
		"karpenter.sh/nodepool",
		"class.truefoundry.com/component",
	}

	return slices.Contains(commonKeys, key)
}

func checkTopologySpreadConstraints(podTemplate *corev1.PodTemplateSpec) bool {
	if len(podTemplate.Spec.TopologySpreadConstraints) == 0 {
		return false
	}

	for _, constraint := range podTemplate.Spec.TopologySpreadConstraints {
		if constraint.TopologyKey != "topology.kubernetes.io/zone" {
			return true
		}
	}

	return false
}

func checkPodAntiAffinity(podTemplate *corev1.PodTemplateSpec) bool {
	if podTemplate.Spec.Affinity == nil || podTemplate.Spec.Affinity.PodAntiAffinity == nil {
		return false
	}

	antiAffinity := podTemplate.Spec.Affinity.PodAntiAffinity

	return len(antiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 ||
		len(antiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0
}
