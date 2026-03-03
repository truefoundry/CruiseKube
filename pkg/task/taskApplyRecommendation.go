package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/truefoundry/cruisekube/pkg/adapters/metricsProvider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/audit"
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
	NonOptimizablePods          []utils.PodInfo
	OptimizableButExcludedPods  []utils.PodInfo
	OptimizablePods             []utils.PodInfo
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

	if !utils.CheckIfClusterVersionAbove(ctx, a.config.ClusterID, a.kubeClient, 1, 33) {
		applyChanges = false
		logging.Infof(ctx, "Cluster version is not above 1.33, running in dry run mode")
	}

	nodeRecommendationMap, err := a.GenerateNodeStatsForCluster(ctx)
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

	supportsMemoryLimitReduction := utils.CheckIfClusterVersionAbove(ctx, a.config.ClusterID, a.kubeClient, 1, 34)

	recommendationResults, err := a.ApplyRecommendationsWithStrategy(
		ctx,
		nodeRecommendationMap,
		overridesMap,
		applystrategies.NewAdjustAmongstPodsDistributedStrategy(ctx),
		applyChanges,
		false,
		supportsMemoryLimitReduction,
	)
	if err != nil {
		logging.Errorf(ctx, "Error applying recommendations: %v", err)
		return err
	}

	if err := a.buildAndSaveSnapshot(ctx, nodeRecommendationMap, recommendationResults); err != nil {
		logging.Errorf(ctx, "Error saving node snapshot: %v", err)
		return err
	}

	return nil
}

func (a *ApplyRecommendationTask) ApplyRecommendationsWithStrategy(
	ctx context.Context,
	nodeStatsMap map[string]utils.NodeResourceInfo,
	overridesMap map[string]*types.WorkloadOverrideInfo,
	strategy applystrategies.AdjustAmongstPodsDistributedStrategy,
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
			OptimizablePods:             make([]utils.PodInfo, 0),
			NonOptimizablePods:          make([]utils.PodInfo, 0),
			OptimizableButExcludedPods:  make([]utils.PodInfo, 0),
		}
		recommendationResults = append(recommendationResults, recommendationResult)

		optimizablePods, optimizableButExcludedPods, nonOptimizablePods := a.segregateOptimizableNonOptimizablePods(ctx, nodeInfo.Pods, overridesMap)

		recommendationResult.NonOptimizablePods = nonOptimizablePods
		recommendationResult.OptimizableButExcludedPods = optimizableButExcludedPods
		recommendationResult.OptimizablePods = optimizablePods

		metrics.ClusterNonOptimizablePodsCount.WithLabelValues(a.config.ClusterID, nodeName).Set(float64(len(nonOptimizablePods)))
		metrics.ClusterOptimizablePodsCount.WithLabelValues(a.config.ClusterID, nodeName).Set(float64(len(optimizablePods)))
		metrics.ClusterOptimizableButExcludedPodsCount.WithLabelValues(a.config.ClusterID, nodeName).Set(float64(len(optimizableButExcludedPods)))

		availableCPU := nodeInfo.AllocatableCPU
		availableMemory := nodeInfo.AllocatableMemory
		// reducing the available resources by the pods i can't touch
		for _, nonOptimizablePod := range nonOptimizablePods {
			availableMemory -= nonOptimizablePod.RequestedMemory
			availableCPU -= nonOptimizablePod.RequestedCPU
		}
		for _, optimizableButExcludedPod := range optimizableButExcludedPods {
			availableMemory -= optimizableButExcludedPod.RequestedMemory
			availableCPU -= optimizableButExcludedPod.RequestedCPU
		}

		// adding dummy recommendations for non-optimizable pods and optimizable but excluded pods
		for _, nonOptimizablePod := range nonOptimizablePods {
			for _, container := range nonOptimizablePod.ContainerResources {
				recommendationResult.PodContainerRecommendations = append(recommendationResult.PodContainerRecommendations, utils.PodContainerRecommendation{
					PodInfo:       nonOptimizablePod,
					ContainerName: container.Name,
					CPU:           container.CPURequest,
					Memory:        container.MemoryRequest,
					Evict:         false,
				})
			}
		}

		for _, optimizableButExcludedPod := range optimizableButExcludedPods {
			for _, container := range optimizableButExcludedPod.ContainerResources {
				containerStat, err := optimizableButExcludedPod.Stats.GetContainerStats(container.Name)
				if err != nil {
					logging.Errorf(ctx, "Error getting container stats for container %s: %v", container.Name, err)
					continue
				}
				recommendedCPU, restCPU := strategy.GetRecommendedAndRestCPU(ctx, optimizableButExcludedPod, *containerStat)
				recommendedMemory, restMemory := strategy.GetRecommendedAndRestMemory(ctx, optimizableButExcludedPod, *containerStat)

				recommendationResult.PodContainerRecommendations = append(recommendationResult.PodContainerRecommendations, utils.PodContainerRecommendation{
					PodInfo:       optimizableButExcludedPod,
					ContainerName: container.Name,
					CPU:           recommendedCPU + restCPU,
					Memory:        recommendedMemory + restMemory,
					Evict:         false,
				})
			}
		}

		result, err := strategy.OptimizeNode(ctx, a.kubeClient, overridesMap, utils.NodeOptimizationData{
			NodeName:          nodeName,
			AllocatableCPU:    availableCPU,
			AllocatableMemory: availableMemory,
			PodInfos:          optimizablePods,
		})
		if err != nil {
			logging.Errorf(ctx, "Error optimizing node %s: %v", nodeName, err)
			continue
		}
		recommendationResult.PodContainerRecommendations = append(recommendationResult.PodContainerRecommendations, result.PodContainerRecommendations...)
		recommendationResult.MaxRestCPU = result.MaxRestCPU
		recommendationResult.MaxRestMemory = result.MaxRestMemory
	}

	rows := a.buildPodRecommendationRows(ctx, recommendationResults)
	if err := a.storage.SavePodRecommendations(a.config.ClusterID, rows); err != nil {
		return nil, fmt.Errorf("failed to save pod recommendations: %w", err)
	}

	optimizableWorkloadIDs := make(map[string]struct{})
	for _, recommendationResult := range recommendationResults {
		for _, pod := range recommendationResult.OptimizablePods {
			optimizableWorkloadIDs[pod.Stats.WorkloadIdentifier] = struct{}{}
		}
	}
	for _, recommendationResult := range recommendationResults {
		nodeName := recommendationResult.NodeName
		nodeInfo := recommendationResult.NodeInfo
		podContainerRecommendation := recommendationResult.PodContainerRecommendations
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

		for _, rec := range podContainerRecommendation {
			if _, ok := optimizableWorkloadIDs[utils.GetWorkloadKey(rec.PodInfo.WorkloadKind, rec.PodInfo.Namespace, rec.PodInfo.WorkloadName)]; !ok {
				continue
			}
			freshPod, found := podsOnNode[utils.GetPodKey(rec.PodInfo.Namespace, rec.PodInfo.Name)]
			if !found {
				logging.Errorf(ctx, "Pod %s/%s not found on node %s", rec.PodInfo.Namespace, rec.PodInfo.Name, nodeName)
				continue
			}

			if utils.ToBeEvicted(rec) {
				podsToEvict[fmt.Sprintf("%s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)] = true
				logging.Infof(ctx, "[Decision] Evicting pod %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
				if applyChanges {
					logging.Infof(ctx, "[Action] Evicting pod %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
					success, errStr := utils.EvictPod(ctx, a.kubeClient, freshPod)
					if !success {
						logging.Errorf(ctx, "Error evicting pod %s/%s: %v", rec.PodInfo.Namespace, rec.PodInfo.Name, errStr)
						continue
					}
					if audit.Recorder != nil {
						workloadID := ""
						if rec.PodInfo.Stats != nil {
							workloadID = rec.PodInfo.Stats.WorkloadIdentifier
						}
						audit.Recorder.Record(ctx, a.config.ClusterID, types.AuditEvent{
							Type:     types.EventTypeNormal,
							Category: types.EventCategoryPODEviction,
							Payload: types.AuditPayload{
								Message: fmt.Sprintf("Pod %s/%s evicted for resource optimization", rec.PodInfo.Namespace, rec.PodInfo.Name),
								Target:  map[string]interface{}{"kind": rec.PodInfo.WorkloadKind, "namespace": rec.PodInfo.Namespace, "name": rec.PodInfo.Name},
								Details: map[string]interface{}{
									"workloadId":    workloadID,
									"node":          nodeName,
									"containerName": rec.ContainerName,
								},
							},
						})
					}
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
				if audit.Recorder != nil {
					recommendedCPURequest, _, recommendedCPULimit, _ := utils.ComputeRecommendedResourceValues(ctx, rec, nodeInfo.AllocatableCPU)
					before := make(map[string]interface{})
					if q := currentContainerResources.Requests[corev1.ResourceCPU]; !q.IsZero() {
						before[types.AuditDetailCPURequestMillis] = q.MilliValue()
					}
					if q := currentContainerResources.Limits[corev1.ResourceCPU]; !q.IsZero() {
						before[types.AuditDetailCPULimitMillis] = q.MilliValue()
					}
					after := make(map[string]interface{})
					after[types.AuditDetailCPURequestMillis] = int64(recommendedCPURequest * 1000)
					after[types.AuditDetailCPULimitMillis] = int64(recommendedCPULimit * 1000)
					audit.Recorder.Record(ctx, a.config.ClusterID, types.AuditEvent{
						Type:     types.EventTypeNormal,
						Category: types.EventCategoryCPURecommendationApplied,
						Payload: types.AuditPayload{
							Message: fmt.Sprintf("CPU recommendation applied for pod %s/%s container %s", rec.PodInfo.Namespace, rec.PodInfo.Name, rec.ContainerName),
							Target:  map[string]interface{}{"kind": rec.PodInfo.WorkloadKind, "namespace": rec.PodInfo.Namespace, "name": rec.PodInfo.Name},
							Details: map[string]interface{}{
								"workloadId":    utils.GetWorkloadKey(rec.PodInfo.WorkloadKind, rec.PodInfo.Namespace, rec.PodInfo.WorkloadName),
								"node":          nodeName,
								"containerName": rec.ContainerName,
								"before":        before,
								"after":         after},
						},
					})
				}
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
					if audit.Recorder != nil {
						_, recommendedMemoryRequest, _, recommendedMemoryLimit := utils.ComputeRecommendedResourceValues(ctx, rec, 0)
						before := make(map[string]interface{})
						if q := currentContainerResources.Requests[corev1.ResourceMemory]; !q.IsZero() {
							before[types.AuditDetailMemoryRequestMB] = float64(q.Value()) / utils.BytesToMBDivisor
						}
						if q := currentContainerResources.Limits[corev1.ResourceMemory]; !q.IsZero() {
							before[types.AuditDetailMemoryLimitMB] = float64(q.Value()) / utils.BytesToMBDivisor
						}
						audit.Recorder.Record(ctx, a.config.ClusterID, types.AuditEvent{
							Type:     types.EventTypeNormal,
							Category: types.EventCategoryMemoryRecommendationApplied,
							Payload: types.AuditPayload{
								Message: fmt.Sprintf("Memory recommendation applied for pod %s/%s container %s", rec.PodInfo.Namespace, rec.PodInfo.Name, rec.ContainerName),
								Target:  map[string]interface{}{"kind": rec.PodInfo.WorkloadKind, "namespace": rec.PodInfo.Namespace, "name": rec.PodInfo.Name},
								Details: map[string]interface{}{
									"workloadId":    utils.GetWorkloadKey(rec.PodInfo.WorkloadKind, rec.PodInfo.Namespace, rec.PodInfo.WorkloadName),
									"node":          nodeName,
									"containerName": rec.ContainerName,
									"before":        before,
									"after": map[string]interface{}{
										types.AuditDetailMemoryRequestMB: recommendedMemoryRequest,
										types.AuditDetailMemoryLimitMB:   recommendedMemoryLimit,
									},
								},
							},
						})
					}
				}
			} else {
				logging.Infof(ctx, "Skipping memory recommendation application for pod since memory recommendation application is disabled: %s/%s", rec.PodInfo.Namespace, rec.PodInfo.Name)
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
	// We use to set the max limit
	allocatableCPU := 0.0
	_, recommendedMemoryRequest, _, recommendedMemoryLimit := utils.ComputeRecommendedResourceValues(ctx, rec, allocatableCPU)

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
		msg := fmt.Sprintf("memory changed too much from %.1f MB to %.1f MB", currentMemoryRequest, containerResource.MemoryRequest)
		logging.Infof(ctx, "pod %s/%s %s, skipping applying memory recommendation", rec.PodInfo.Namespace, rec.PodInfo.Name, msg)
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
			logging.Debugf(ctx, "[dry run] pod %v/%v memory request updated: %v -> %v", rec.PodInfo.Namespace, rec.PodInfo.Name, currentMemoryRequest, recommendedMemoryRequest)
			return false, false, nil
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
		msg := fmt.Sprintf("cpu changed too much from %.1f to %.1f", currentCPURequest, containerResource.CPURequest)
		logging.Infof(ctx, "pod %s/%s %s, skipping applying cpu recommendation", rec.PodInfo.Namespace, rec.PodInfo.Name, msg)
		return false, nil
	}

	currentCPULimitQuantity := currentContainerResources.Limits[corev1.ResourceCPU]
	currentCPULimit := float64(currentCPULimitQuantity.MilliValue()) / 1000.0

	recommendedCPURequest, _, recommendedCPULimit, _ := utils.ComputeRecommendedResourceValues(ctx, rec, allocatableCPU)
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
			return false, nil
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

// countNodeHealth lists cluster nodes and returns counts of healthy (Ready) vs unhealthy (NotReady/Unknown) nodes.
func (a *ApplyRecommendationTask) countNodeHealth(ctx context.Context) (int, int) {
	nodeList, err := a.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Warnf(ctx, "snapshot: failed to list nodes for health count: %v", err)
		return 0, 0
	}
	var healthy, unhealthy = 0, 0
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if isNodeReady(node) {
			healthy++
		} else {
			unhealthy++
		}
	}
	return healthy, unhealthy
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// countPodsByStatus lists pods (in TargetNamespace or all namespaces) and returns counts by pod phase (Running, Pending, etc.).
func (a *ApplyRecommendationTask) countPodsByStatus(ctx context.Context) types.SnapshotPodsCount {
	ns := a.config.TargetNamespace
	var podList *corev1.PodList
	var err error
	if ns != "" {
		podList, err = a.kubeClient.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	} else {
		podList, err = a.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		logging.Warnf(ctx, "snapshot: failed to list pods for status count: %v", err)
		return types.SnapshotPodsCount{}
	}
	counts := make(types.SnapshotPodsCount)
	for i := range podList.Items {
		phase := podList.Items[i].Status.Phase
		if phase == "" {
			phase = corev1.PodUnknown
		}
		counts[string(phase)]++
	}
	return counts
}

// buildAndSaveSnapshot aggregates cluster-level CPU/Memory metrics from nodeRecommendationMap
// (allocatable, requested) and recommendationResults (workload/recommended request totals), then
// persists one row to the node_snapshots table (timestamp in GMT).
func (a *ApplyRecommendationTask) buildAndSaveSnapshot(ctx context.Context, nodeRecommendationMap map[string]utils.NodeResourceInfo, recommendationResults []*RecommendationResult) error {
	var currentAllocatableCPU, currentAllocatableMemory float64
	var currentRequestedCPU, currentRequestedMemory float64
	var currentUtilizedCPU, currentUtilizedMemory float64
	var workloadRequestedCPU, workloadRequestedMemory float64
	var recommendedRequestedCPU, recommendedRequestedMemory float64

	// Use nodeRecommendationMap for cluster allocatable and requested (all nodes).
	for _, ni := range nodeRecommendationMap {
		currentAllocatableCPU += ni.AllocatableCPU
		currentAllocatableMemory += ni.AllocatableMemory
		currentRequestedCPU += ni.RequestedCPU
		currentRequestedMemory += ni.RequestedMemory

		for _, pod := range ni.Pods {
			if pod.Stats == nil {
				continue
			}
			workloadRequestedCPU += pod.Stats.CalculateTotalCPURequest()
			workloadRequestedMemory += pod.Stats.CalculateTotalMemoryRequest()
		}
	}

	currentUtilizedCPU = utils.QueryAndParsePrometheusScalar(ctx, a.promClient.GetClient(), utils.BuildClusterCPUUtilizationExpression())
	currentUtilizedMemory = utils.QueryAndParsePrometheusScalar(ctx, a.promClient.GetClient(), utils.BuildClusterMemoryUtilizationExpression())

	recommendedPods := 0
	for _, res := range recommendationResults {
		for _, rec := range res.PodContainerRecommendations {
			recommendedPods++
			recommendedRequestedCPU += rec.CPU
			recommendedRequestedMemory += rec.Memory
		}
	}

	cpu := types.SnapshotResourceMetrics{
		CurrentAllocatable:   currentAllocatableCPU,
		CurrentRequested:     currentRequestedCPU,
		CurrentUtilized:      currentUtilizedCPU,
		WorkloadRequested:    workloadRequestedCPU,
		RecommendedRequested: recommendedRequestedCPU,
	}
	memory := types.SnapshotResourceMetrics{
		CurrentAllocatable:   currentAllocatableMemory / 1000.0,
		CurrentRequested:     currentRequestedMemory / 1000.0,
		CurrentUtilized:      currentUtilizedMemory,
		WorkloadRequested:    workloadRequestedMemory / 1000.0,
		RecommendedRequested: recommendedRequestedMemory / 1000.0,
	}

	if currentUtilizedCPU == 0 || currentUtilizedMemory == 0 {
		logging.Warnf(ctx, "snapshot: no CPU/Memory utilization found from prometheus, skipping snapshot")
		return nil
	}

	if recommendedPods == 0 {
		logging.Warnf(ctx, "snapshot: no pods were recommended, skipping snapshot")
		return nil
	}

	healthyNodes, unhealthyNodes := a.countNodeHealth(ctx)
	podsCount := a.countPodsByStatus(ctx)
	snapshot := &types.SnapshotPayload{
		ClusterID: a.config.ClusterID,
		Data: types.SnapshotData{
			CPU:       cpu,
			Memory:    memory,
			Nodes:     types.SnapshotNodes{Healthy: healthyNodes, Unhealthy: unhealthyNodes},
			PodsCount: podsCount,
		},
	}
	if err := a.storage.InsertSnapshot(snapshot); err != nil {
		return fmt.Errorf("insert node snapshot: %w", err)
	}
	return nil
}

func (a *ApplyRecommendationTask) buildPodRecommendationRows(ctx context.Context, recommendationResults []*RecommendationResult) []types.PodResourceRecommendationRow {
	rows := make([]types.PodResourceRecommendationRow, 0)
	for _, res := range recommendationResults {
		nodeName := res.NodeName
		allocatableCPU := res.NodeInfo.AllocatableCPU
		optimizableWorkloadIds := make(map[string]struct{})
		nonOptimizableWorkloadIds := make(map[string]struct{})
		optimizableButExcludedWorkloadIds := make(map[string]struct{})

		for _, pod := range res.OptimizablePods {
			optimizableWorkloadIds[pod.Stats.WorkloadIdentifier] = struct{}{}
		}
		for _, pod := range res.NonOptimizablePods {
			if pod.Stats == nil {
				continue
			}
			nonOptimizableWorkloadIds[pod.Stats.WorkloadIdentifier] = struct{}{}
		}
		for _, pod := range res.OptimizableButExcludedPods {
			optimizableButExcludedWorkloadIds[pod.Stats.WorkloadIdentifier] = struct{}{}
		}

		for _, rec := range res.PodContainerRecommendations {
			kind, namespace, name := rec.PodInfo.WorkloadKind, rec.PodInfo.Namespace, rec.PodInfo.WorkloadName
			workloadID := utils.GetWorkloadKey(kind, namespace, name)
			cpuRequest, memoryRequest, cpuLimit, memoryLimit := utils.ComputeRecommendedResourceValues(ctx, rec, allocatableCPU)
			recommendationType := types.RecommendationTypeNonOptimizable
			if _, ok := optimizableWorkloadIds[workloadID]; ok {
				recommendationType = types.RecommendationTypeOptimizable
			} else if _, ok := nonOptimizableWorkloadIds[workloadID]; ok {
				recommendationType = types.RecommendationTypeNonOptimizable
			} else if _, ok := optimizableButExcludedWorkloadIds[workloadID]; ok {
				recommendationType = types.RecommendationTypeOptimizableButExcluded
			}

			payload := types.PodResourceRecommendation{
				RecommendationType: recommendationType,
				CPURequest:         cpuRequest,
				MemoryRequest:      memoryRequest,
				CPULimit:           cpuLimit,
				MemoryLimit:        memoryLimit,
				ToBeEvicted:        utils.ToBeEvicted(rec),
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
	}
	return rows
}

func (a *ApplyRecommendationTask) segregateOptimizableNonOptimizablePods(ctx context.Context, allPodInfos []utils.PodInfo, overridesMap map[string]*types.WorkloadOverrideInfo) ([]utils.PodInfo, []utils.PodInfo, []utils.PodInfo) {
	optimizablePods := make([]utils.PodInfo, 0)
	optimizableButExcludedPods := make([]utils.PodInfo, 0)
	nonOptimizablePods := make([]utils.PodInfo, 0)

	input := utils.ApplyCheckInput{
		ApplyBlacklistedNamespaces: a.config.RecommendationSettings.ApplyBlacklistedNamespaces,
		K8sVersionGE133:            true, // caller sets applyChanges=false when cluster < 1.33
		K8sMemoryGE134:             true, // caller uses supportsMemoryReduction separately
		OptimizeGuaranteedPods:     a.config.RecommendationSettings.OptimizeGuaranteedPods,
		DisableMemoryApplication:   a.config.RecommendationSettings.DisableMemoryApplication,
		NewWorkloadThresholdHours:  a.config.RecommendationSettings.NewWorkloadThresholdHours,
		SkipMemory:                 a.config.Metadata.SkipMemory,
		PodExcludedByAnnotation:    utils.PodExcludedByAnnotation(nil), // when podForExclusion is nil, value is taken from podInfo.Stats.Constraints
	}

	for _, podInfo := range allPodInfos {
		if podInfo.Stats == nil {
			nonOptimizablePods = append(nonOptimizablePods, podInfo)
			continue
		}
		override := overridesMap[podInfo.Stats.WorkloadIdentifier]

		shouldGenerate, reason := utils.ShouldGenerateRecommendation(ctx, &podInfo, input)
		if !shouldGenerate {
			logging.Infof(ctx, "Skipping pod %s/%s: %s", podInfo.Namespace, podInfo.Name, reason)
			nonOptimizablePods = append(nonOptimizablePods, podInfo)
			continue
		}

		shouldApply, reason := utils.ShouldApplyRecommendationToPod(ctx, &podInfo, override, input)
		if !shouldApply {
			logging.Infof(ctx, "Skipping pod %s/%s: %s", podInfo.Namespace, podInfo.Name, reason)
			optimizableButExcludedPods = append(optimizableButExcludedPods, podInfo)
			continue
		}
		optimizablePods = append(optimizablePods, podInfo)
	}

	return optimizablePods, optimizableButExcludedPods, nonOptimizablePods
}

// GenerateNodeStatsForCluster builds the node -> pods/resources map using cluster state and stored stats.
func (a *ApplyRecommendationTask) GenerateNodeStatsForCluster(ctx context.Context) (map[string]utils.NodeResourceInfo, error) {
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
		statsFile, err = utils.LoadStatsFromClusterStorage(a.config.ClusterID)
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
