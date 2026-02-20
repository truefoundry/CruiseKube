package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/truefoundry/cruisekube/pkg/adapters/metricsProvider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/client"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/metrics"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/applystrategies"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type RecommendationResult struct {
	NodeName                    string
	NodeInfo                    utils.NodeResourceInfo
	PodContainerRecommendations []utils.PodContainerRecommendation
	NonOptimizablePods          []utils.NonOptimizablePodInfo
	MaxRestCPU                  float64
	MaxRestMemory               float64
}

type ApplyRecommendationMetadata struct {
	DryRun       bool             `yaml:"dryRun" json:"dryRun" mapstructure:"dryRun"`
	NodeStatsURL config.URLConfig `yaml:"nodeStatsURL" json:"nodeStatsURL" mapstructure:"nodeStatsURL"`
	OverridesURL config.URLConfig `yaml:"overridesURL" json:"overridesURL" mapstructure:"overridesURL"`
	SkipMemory   bool             `yaml:"skipMemory" json:"skipMemory" mapstructure:"skipMemory"`
}

type ApplyRecommendationTaskConfig struct {
	Name                     string
	Enabled                  bool
	Schedule                 string
	ClusterID                string
	TargetClusterID          string
	TargetNamespace          string
	IsClusterWriteAuthorized bool
	BasicAuth                config.BasicAuthConfig
	RecommendationSettings   config.RecommendationSettings
	Metadata                 ApplyRecommendationMetadata
}

type ApplyRecommendationTask struct {
	config        *ApplyRecommendationTaskConfig
	kubeClient    *kubernetes.Clientset
	dynamicClient dynamic.Interface
	promClient    *prometheus.PrometheusProvider
	storage       *storage.Storage
}

func NewApplyRecommendationTask(ctx context.Context, kubeClient *kubernetes.Clientset, dynamicClient dynamic.Interface, promClient *prometheus.PrometheusProvider, storage *storage.Storage, config *ApplyRecommendationTaskConfig, taskConfig *config.TaskConfig) *ApplyRecommendationTask {
	var applyRecommendationMetadata ApplyRecommendationMetadata
	if err := taskConfig.ConvertMetadataToStruct(&applyRecommendationMetadata); err != nil {
		logging.Errorf(ctx, "Error converting metadata to struct: %v", err)
		return nil
	}
	config.Metadata = applyRecommendationMetadata

	return &ApplyRecommendationTask{
		config:        config,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		promClient:    promClient,
		storage:       storage,
	}
}

func (a *ApplyRecommendationTask) GetCoreTask() any {
	return a
}

func (a *ApplyRecommendationTask) GetName() string {
	return a.config.Name
}

func (a *ApplyRecommendationTask) GetSchedule() string {
	return a.config.Schedule
}

func (a *ApplyRecommendationTask) IsEnabled() bool {
	return a.config.Enabled
}

func (a *ApplyRecommendationTask) Run(ctx context.Context) error {
	ctx = contextutils.WithTask(ctx, a.config.Name)
	ctx = contextutils.WithCluster(ctx, a.config.ClusterID)

	applyChanges := !a.config.Metadata.DryRun

	if !a.config.IsClusterWriteAuthorized {
		logging.Infof(ctx, "Cluster %s is not write authorized, skipping ApplyRecommendation task", a.config.ClusterID)
		return nil
	}

	if !utils.CheckIfClusterVersionAbove(ctx, a.kubeClient, 1, 33) {
		applyChanges = false
		logging.Infof(ctx, "Cluster version is not above 1.33, running in dry run mode")
	}

	nodeRecommendationMap, err := a.GenerateNodeStatsForCluster(ctx, nil)
	if err != nil {
		logging.Errorf(ctx, "Error generating node recommendations: %v", err)
		return err
	}
	logging.Infof(ctx, "Loaded %d node recommendations", len(nodeRecommendationMap))

	recommenderClient := client.NewRecommenderServiceClientWithBasicAuth(
		a.config.Metadata.NodeStatsURL.Host,
		a.config.BasicAuth.Username,
		a.config.BasicAuth.Password,
	)
	workloadOverrides, err := recommenderClient.ListWorkloads(ctx, a.config.ClusterID)
	if err != nil {
		logging.Errorf(ctx, "Error loading workload overrides from client: %v", err)
		return fmt.Errorf("failed to list workloads from recommender service: %w", err)
	}

	overridesMap := make(map[string]*types.WorkloadOverrideInfo)
	for _, override := range workloadOverrides {
		overridesMap[override.WorkloadID] = &override
	}

	supportsMemoryReduction := utils.CheckIfClusterVersionAbove(ctx, a.kubeClient, 1, 34)

	_, err = a.ApplyRecommendationsWithStrategy(
		ctx,
		nodeRecommendationMap,
		overridesMap,
		applystrategies.NewAdjustAmongstPodsDistributedStrategy(ctx),
		applyChanges,
		false,
		supportsMemoryReduction,
	)
	if err != nil {
		logging.Errorf(ctx, "Error applying recommendations: %v", err)
		return err
	}

	return nil
}

func (a *ApplyRecommendationTask) ApplyRecommendationsWithStrategy(
	ctx context.Context,
	nodeStatsMap map[string]utils.NodeResourceInfo,
	overridesMap map[string]*types.WorkloadOverrideInfo,
	strategy utils.OptimizationStrategy,
	applyChanges bool,
	generateRecommendationOnly bool,
	supportsMemoryReduction bool,
) ([]*RecommendationResult, error) {
	logging.Infof(ctx, "Starting recommendation application using strategy: %s", strategy.GetName())
	if !applyChanges {
		logging.Infof(ctx, "DRY RUN MODE: Changes will be calculated but not applied")
	}

	recommendationResults := []*RecommendationResult{}
	for nodeName, nodeInfo := range nodeStatsMap {
		// if nodeName != "ip-10-99-47-228.ec2.internal" {
		// 	continue
		// }
		logging.Infof(ctx, "Processing node: %s", nodeName)

		recommendationResult := &RecommendationResult{
			NodeName:                    nodeName,
			NodeInfo:                    nodeInfo,
			PodContainerRecommendations: make([]utils.PodContainerRecommendation, 0),
			NonOptimizablePods:          make([]utils.NonOptimizablePodInfo, 0),
		}
		recommendationResults = append(recommendationResults, recommendationResult)

		optimizablePods, nonOptimizablePods := a.segregateOptimizableNonOptimizablePods(ctx, nodeInfo.Pods, overridesMap)
		recommendationResult.NonOptimizablePods = nonOptimizablePods

		metrics.ClusterNonOptimizablePodsCount.WithLabelValues(a.config.ClusterID, nodeName).Set(float64(len(nonOptimizablePods)))
		metrics.ClusterOptimizablePodsCount.WithLabelValues(a.config.ClusterID, nodeName).Set(float64(len(optimizablePods)))

		availableCPU := nodeInfo.AllocatableCPU
		availableMemory := nodeInfo.AllocatableMemory
		// reducing the available resources by the pods i can't touch
		for _, nonOptimizablePod := range nonOptimizablePods {
			availableMemory -= nonOptimizablePod.CurrentMemory
			availableCPU -= nonOptimizablePod.CurrentCPU
		}

		result, err := strategy.OptimizeNode(a.kubeClient, overridesMap, utils.NodeOptimizationData{
			NodeName:          nodeName,
			AllocatableCPU:    availableCPU,
			AllocatableMemory: availableMemory,
			PodInfos:          optimizablePods,
		})
		if err != nil {
			logging.Errorf(ctx, "Error optimizing node %s: %v", nodeName, err)
			continue
		}
		recommendationResult.PodContainerRecommendations = result.PodContainerRecommendations
		recommendationResult.MaxRestCPU = result.MaxRestCPU
		recommendationResult.MaxRestMemory = result.MaxRestMemory
	}

	rows := a.buildPodRecommendationRows(ctx, recommendationResults)
	if err := a.storage.SavePodRecommendations(a.config.ClusterID, rows); err != nil {
		return nil, fmt.Errorf("failed to save pod recommendations: %w", err)
	}

	for _, recommendationResult := range recommendationResults {
		nodeName := recommendationResult.NodeName
		nodeInfo := recommendationResult.NodeInfo
		result := recommendationResult.PodContainerRecommendations
		if generateRecommendationOnly {
			logging.Infof(ctx, "Skipping applying recommendations for node %s", nodeName)
			continue
		}

		podsOnNode, err := a.getFreshPodsOnNode(ctx, nodeName)
		if err != nil {
			logging.Errorf(ctx, "Error getting fresh pods on node %s: %v", nodeName, err)
			continue
		}

		podsToEvict := make(map[string]bool)
		appliedRecommendations := make(map[string]utils.PodContainerRecommendation)

		for _, rec := range result {
			freshPod, found := podsOnNode[utils.GetPodKey(rec.PodInfo.Namespace, rec.PodInfo.Name)]
			if !found {
				logging.Errorf(ctx, "Pod %s/%s not found on node %s", rec.PodInfo.Namespace, rec.PodInfo.Name, nodeName)
				continue
			}

			if utils.ToBeEvicted(rec) {
				podsToEvict[fmt.Sprintf("%s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)] = true
				logging.Infof(ctx, "Evicting pod %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
				if applyChanges {
					utils.EvictPod(ctx, a.kubeClient, freshPod)
				}
				continue
			}

			var currentContainerResources corev1.ResourceRequirements
			for _, container := range freshPod.Spec.Containers {
				if container.Name == rec.ContainerName {
					currentContainerResources = container.Resources
				}
			}

			applied, err := a.applyCPURecommendation(ctx, freshPod, currentContainerResources, rec, applyChanges, nodeInfo.AllocatableCPU)
			if err != nil {
				logging.Errorf(ctx, "Error applying CPU recommendation for pod %s/%s: %v", rec.PodInfo.Namespace, rec.PodInfo.Name, err)
			}
			if applied {
				appliedRecommendations[fmt.Sprintf("%s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)] = rec
			}

			if !a.config.RecommendationSettings.DisableMemoryApplication && !a.config.Metadata.SkipMemory {
				applied, skipped, err := a.applyMemoryRecommendation(ctx, freshPod, currentContainerResources, rec, applyChanges, supportsMemoryReduction)
				if skipped {
					logging.Infof(ctx, "Skipping memory recommendation for pod %s/%s: %v", rec.PodInfo.Namespace, rec.PodInfo.Name, err)
				} else if err != nil {
					logging.Errorf(ctx, "Error applying memory recommendation for pod %s/%s: %v", rec.PodInfo.Namespace, rec.PodInfo.Name, err)
				}

				if applied {
					appliedRecommendations[fmt.Sprintf("%s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)] = rec
				}
			} else {
				logging.Infof(ctx, "Skipping memory recommendation application for pod since memory recommendationapplication is disabled: %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
			}
		}

		logging.Infof(ctx, "Successfully applied %d recommendations and evicted %d pods", len(appliedRecommendations), len(podsToEvict))
	}

	totalSpikeCPU := 0.0
	totalSpikeMemory := 0.0
	for _, result := range recommendationResults {
		totalSpikeCPU += result.MaxRestCPU
		totalSpikeMemory += result.MaxRestMemory
	}

	metrics.ClusterSpikeCPU.WithLabelValues(a.config.ClusterID).Set(totalSpikeCPU)
	metrics.ClusterSpikeMemory.WithLabelValues(a.config.ClusterID).Set(totalSpikeMemory * 1000 * 1000)

	return recommendationResults, nil
}

func (a *ApplyRecommendationTask) applyMemoryRecommendation(
	ctx context.Context,
	pod *corev1.Pod,
	currentContainerResources corev1.ResourceRequirements,
	rec utils.PodContainerRecommendation,
	applyChanges bool,
	supportsMemoryReduction bool,
) (bool, bool, error) {
	_, recommendedMemoryRequest, _, recommendedMemoryLimit := a.computeRecommendedResourceValues(rec, 0)

	containerResource, err := rec.PodInfo.GetContainerResource(rec.ContainerName)
	if err != nil {
		return false, true, fmt.Errorf("error getting container resource for pod %s/%s: %w", rec.PodInfo.Namespace, rec.PodInfo.Name, err)
	}

	currentMemoryRequestQuantity, exists := currentContainerResources.Requests[corev1.ResourceMemory]
	if !exists {
		return false, true, fmt.Errorf("container %s in pod %s has no memory request", rec.ContainerName, rec.PodInfo.Name)
	}
	currentMemoryRequest := float64(currentMemoryRequestQuantity.Value()) / utils.BytesToMBDivisor
	if math.Abs(currentMemoryRequest-containerResource.MemoryRequest) > utils.MinimumMemoryRecommendation {
		logging.Infof(ctx, "pod %s/%s memory has changed too much from %.1f MB to %.1f MB, skipping applying memory recommendation", rec.PodInfo.Namespace, rec.PodInfo.Name, currentMemoryRequest, containerResource.MemoryRequest)
		return false, true, nil
	}

	currentMemoryLimitQuantity := currentContainerResources.Limits[corev1.ResourceMemory]
	currentMemoryLimit := float64(currentMemoryLimitQuantity.Value()) / utils.BytesToMBDivisor

	if !supportsMemoryReduction && recommendedMemoryLimit < currentMemoryLimit {
		recommendedMemoryLimit = math.Ceil(currentMemoryLimit)
	}
	if currentMemoryLimit == 0 {
		// cannot set memory limit when it is unset
		recommendedMemoryLimit = 0
	}

	if math.Abs(recommendedMemoryRequest-currentMemoryRequest) > 0 {
		if applyChanges {
			applied, errStr := utils.UpdatePodMemoryResources(
				ctx,
				a.kubeClient,
				pod,
				rec.ContainerName,
				recommendedMemoryRequest,
				recommendedMemoryLimit,
			)
			if errStr != "" {
				return false, false, errors.New(errStr)
			}
			if !applied {
				return false, false, fmt.Errorf("update call returned false for pod %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
			}
			logging.Infof(ctx, "pod %v/%v memory request updated: %v -> %v", rec.PodInfo.Namespace, rec.PodInfo.Name, currentMemoryRequest, recommendedMemoryRequest)
			return true, false, nil
		} else {
			logging.Infof(ctx, "[dry run] pod %v/%v memory request updated: %v -> %v", rec.PodInfo.Namespace, rec.PodInfo.Name, currentMemoryRequest, recommendedMemoryRequest)
			return true, false, nil
		}
	} else {
		return false, false, nil
	}
}

func (a *ApplyRecommendationTask) applyCPURecommendation(
	ctx context.Context,
	pod *corev1.Pod,
	currentContainerResources corev1.ResourceRequirements,
	rec utils.PodContainerRecommendation,
	applyChanges bool,
	allocatableCPU float64,
) (bool, error) {
	currentCPURequestQuantity, exists := currentContainerResources.Requests[corev1.ResourceCPU]
	if !exists {
		logging.Infof(ctx, "container %s in pod %s has no CPU request, wont be able to change it", rec.ContainerName, rec.PodInfo.Name)
		return false, nil
	}
	currentCPURequest := float64(currentCPURequestQuantity.MilliValue()) / 1000.0

	containerResource, err := rec.PodInfo.GetContainerResource(rec.ContainerName)
	if err != nil {
		return false, fmt.Errorf("error getting container resource for pod %s/%s: %w", rec.PodInfo.Namespace, rec.PodInfo.Name, err)
	}
	if math.Abs(currentCPURequest-containerResource.CPURequest) > utils.MinimumCPURecommendation {
		logging.Infof(ctx, "pod %s/%s cpu has changed too much from %.1f to %.1f, skipping applying cpu recommendation", rec.PodInfo.Namespace, rec.PodInfo.Name, currentCPURequest, containerResource.CPURequest)
		return false, nil
	}

	currentCPULimitQuantity := currentContainerResources.Limits[corev1.ResourceCPU]
	currentCPULimit := float64(currentCPULimitQuantity.MilliValue()) / 1000.0

	recommendedCPURequest, _, recommendedCPULimit, _ := a.computeRecommendedResourceValues(rec, allocatableCPU)
	if currentCPULimit == 0.0 {
		recommendedCPULimit = 0.0
	}

	if math.Abs(recommendedCPURequest-currentCPURequest) >= 0.001 || math.Abs(recommendedCPULimit-currentCPULimit) >= 0.001 {
		if applyChanges {
			applied, errStr := utils.UpdatePodCPUResources(
				ctx,
				a.kubeClient,
				pod,
				rec.ContainerName,
				recommendedCPURequest,
				recommendedCPULimit,
			)
			if errStr != "" {
				return false, errors.New(errStr)
			}
			if !applied {
				return false, fmt.Errorf("update call returned false for pod %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
			}
			logging.Infof(ctx, "pod %v/%v cpu request updated: %v -> %v", rec.PodInfo.Namespace, rec.PodInfo.Name, currentCPURequest, recommendedCPURequest)
			return true, nil
		} else {
			logging.Infof(ctx, "[dry run] pod %v/%v cpu request updated: %v -> %v", rec.PodInfo.Namespace, rec.PodInfo.Name, currentCPURequest, recommendedCPURequest)
			return true, nil
		}
	} else {
		return false, nil
	}
}

func (a *ApplyRecommendationTask) getFreshPodsOnNode(ctx context.Context, nodeName string) (map[string]*corev1.Pod, error) {
	pods, err := a.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pods on node %s: %w", nodeName, err)
	}
	podMap := make(map[string]*corev1.Pod)
	for _, pod := range pods.Items {
		podMap[utils.GetPodKey(pod.Namespace, pod.Name)] = &pod
	}
	return podMap, nil
}

func (a *ApplyRecommendationTask) computeRecommendedResourceValues(rec utils.PodContainerRecommendation, allocatableCPU float64) (float64, float64, float64, float64) {
	cpuRequest := utils.EnforceMinimumCPU(rec.CPU)
	if cpuRequest > utils.CPUClampValue {
		cpuRequest = utils.CPUClampValue
	}
	memoryRequest := utils.EnforceMinimumMemory(rec.Memory)
	cpuLimit := allocatableCPU
	memoryLimit := memoryRequest * 2
	if rec.PodInfo.Stats != nil {
		if containerStat, err := rec.PodInfo.Stats.GetContainerStats(rec.ContainerName); err == nil {
			var memMax, oom float64
			if containerStat.Memory7Day != nil {
				memMax = containerStat.Memory7Day.Max
			}
			if containerStat.MemoryStats != nil {
				oom = containerStat.MemoryStats.OOMMemory
			}
			memoryLimit = utils.EnforceMinimumMemory(max(memMax, oom) * 2)
		}
	}
	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

func (a *ApplyRecommendationTask) buildPodRecommendationRows(ctx context.Context, recommendationResults []*RecommendationResult) []types.PodResourceRecommendationRow {
	rows := make([]types.PodResourceRecommendationRow, 0)
	for _, res := range recommendationResults {
		nodeName := res.NodeName
		allocatableCPU := res.NodeInfo.AllocatableCPU
		for _, rec := range res.PodContainerRecommendations {
			kind, namespace, name := rec.PodInfo.WorkloadKind, rec.PodInfo.Namespace, rec.PodInfo.WorkloadName
			if rec.PodInfo.Stats != nil {
				kind, namespace, name = rec.PodInfo.Stats.Kind, rec.PodInfo.Stats.Namespace, rec.PodInfo.Stats.Name
			}
			workloadID := utils.GetWorkloadKey(kind, namespace, name)
			cpuRequest, memoryRequest, cpuLimit, memoryLimit := a.computeRecommendedResourceValues(rec, allocatableCPU)
			payload := types.PodResourceRecommendation{
				CPURequest:    cpuRequest,
				MemoryRequest: memoryRequest,
				CPULimit:      cpuLimit,
				MemoryLimit:   memoryLimit,
				ToBeEvicted:   utils.ToBeEvicted(rec),
			}
			recJSON, err := json.Marshal(payload)
			if err != nil {
				logging.Errorf(ctx, "failed to marshal pod recommendation for %s/%s container %s: %v", rec.PodInfo.Namespace, rec.PodInfo.Name, rec.ContainerName, err)
				continue
			}
			rows = append(rows, types.PodResourceRecommendationRow{
				WorkloadID:     workloadID,
				NodeName:       nodeName,
				Namespace:      rec.PodInfo.Namespace,
				Pod:            rec.PodInfo.Name,
				Container:      rec.ContainerName,
				Recommendation: string(recJSON),
			})
		}
		for _, nonOpt := range res.NonOptimizablePods {
			kind, namespace, name := nonOpt.PodInfo.WorkloadKind, nonOpt.PodInfo.Namespace, nonOpt.PodInfo.WorkloadName
			if nonOpt.PodInfo.Stats != nil {
				kind, namespace, name = nonOpt.PodInfo.Stats.Kind, nonOpt.PodInfo.Stats.Namespace, nonOpt.PodInfo.Stats.Name
			}
			for _, cr := range nonOpt.PodInfo.ContainerResources {
				if cr == nil {
					continue
				}
				workloadID := utils.GetWorkloadKey(kind, namespace, name)
				rows = append(rows, types.PodResourceRecommendationRow{
					WorkloadID:     workloadID,
					NodeName:       nodeName,
					Namespace:      nonOpt.PodNamespace,
					Pod:            nonOpt.PodName,
					Container:      cr.Name,
					Recommendation: "",
				})
			}
		}
	}
	return rows
}

func (a *ApplyRecommendationTask) segregateOptimizableNonOptimizablePods(ctx context.Context, allPodInfos []utils.PodInfo, overridesMap map[string]*types.WorkloadOverrideInfo) ([]utils.PodInfo, []utils.NonOptimizablePodInfo) {
	optimizablePods := make([]utils.PodInfo, 0)
	nonOptimizablePods := make([]utils.NonOptimizablePodInfo, 0)

	for _, podInfo := range allPodInfos {
		if len(a.config.RecommendationSettings.ApplyBlacklistedNamespaces) > 0 && slices.Contains(a.config.RecommendationSettings.ApplyBlacklistedNamespaces, podInfo.Namespace) {
			logging.Infof(ctx, "Namespace %s is blacklisted, skipping pod %s/%s", podInfo.Namespace, podInfo.Namespace, podInfo.Name)
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		if podInfo.Stats == nil {
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		if podInfo.IsGuaranteedPod() && !a.config.RecommendationSettings.OptimizeGuaranteedPods {
			logging.Infof(ctx, "Skipping guaranteed pod %s/%s", podInfo.Namespace, podInfo.Name)
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		if podInfo.IsBestEffortPod() {
			logging.Infof(ctx, "Skipping best effort pod %s/%s", podInfo.Namespace, podInfo.Name)
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		overrides, ok := overridesMap[podInfo.Stats.WorkloadIdentifier]
		if !ok || !overrides.EffectiveEnabled() {
			logging.Infof(ctx, "cruisekube not enabled for workload %s (no override or recommend-only mode), skipping apply", podInfo.Stats.WorkloadIdentifier)
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		if podInfo.Stats.CreationTime.After(time.Now().Add(-1 * time.Hour * time.Duration(a.config.RecommendationSettings.NewWorkloadThresholdHours))) {
			logging.Infof(ctx, "Pod %s/%s is from a new workload, skipping", podInfo.Namespace, podInfo.Name)
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		if podInfo.Stats.IsHorizontallyAutoscaledOnCPU || podInfo.Stats.IsHorizontallyAutoscaledOnMem {
			logging.Infof(ctx, "Pod %s/%s is horizontally autoscaled on CPU/Memory, skipping", podInfo.Namespace, podInfo.Name)
			nonOptimizablePods = append(nonOptimizablePods, utils.NonOptimizablePodInfo{
				PodInfo:       podInfo,
				PodName:       podInfo.Name,
				PodNamespace:  podInfo.Namespace,
				CurrentCPU:    podInfo.RequestedCPU,
				CurrentMemory: podInfo.RequestedMemory,
			})
			continue
		}

		optimizablePods = append(optimizablePods, podInfo)
	}

	return optimizablePods, nonOptimizablePods
}

// GenerateNodeStatsForCluster builds the node -> pods/resources map using cluster state and stored stats.
// If since is non-nil, only stats updated after that time are loaded (local storage only).
func (a *ApplyRecommendationTask) GenerateNodeStatsForCluster(ctx context.Context, since *time.Time) (map[string]utils.NodeResourceInfo, error) {
	defer utils.TimeIt(ctx, "Generating node stats for cluster")

	targetNamespace := ""

	podToWorkloadMap, allPods, err := utils.BuildPodToWorkloadMapping(ctx, a.kubeClient, targetNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to build pod-to-workload mapping: %w", err)
	}

	var statsFile *types.StatsResponse

	if a.config.Metadata.NodeStatsURL.Host != "" {
		recommenderClient := client.NewRecommenderServiceClientWithBasicAuth(
			a.config.Metadata.NodeStatsURL.Host,
			a.config.BasicAuth.Username,
			a.config.BasicAuth.Password,
		)
		var err error
		statsFile, err = recommenderClient.GetClusterStats(ctx, a.config.ClusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to load stats from client: %w", err)
		}
	} else {
		var err error
		if since != nil {
			statsFile, err = utils.LoadStatsFromClusterStorageUpdatedSince(a.config.ClusterID, *since)
		} else {
			statsFile, err = utils.LoadStatsFromClusterStorage(a.config.ClusterID)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to load stats from storage: %w", err)
		}
	}

	statsMap := make(map[string]*utils.WorkloadStat)
	for i := range statsFile.Stats {
		rec := &statsFile.Stats[i]
		statsMap[rec.WorkloadIdentifier] = rec
	}

	podStats := utils.CreatePodToStatsMapping(ctx, podToWorkloadMap, statsMap)

	nodeMap, err := utils.CreateNodeStatsMapping(ctx, a.kubeClient, podStats, podToWorkloadMap, allPods)
	if err != nil {
		return nil, fmt.Errorf("failed to create node stats mapping: %w", err)
	}

	logging.Infof(ctx, "Generated node stats for cluster %s", a.config.ClusterID)
	return nodeMap, nil
}
