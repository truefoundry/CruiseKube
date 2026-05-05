package task

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/adapters/metricsprovider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type CreateStatsMetadata struct {
	SkipMemory bool `yaml:"skipMemory" json:"skipMemory" mapstructure:"skipMemory"`
}

type CreateStatsTaskConfig struct {
	Name                       string
	Enabled                    bool
	Schedule                   string
	ClusterID                  string
	TargetClusterID            string
	TargetNamespace            string
	RecentStatsLookbackMinutes int
	TimeStepSize               time.Duration
	MLLookbackWindow           time.Duration
	Metadata                   CreateStatsMetadata
}

type CreateStatsTask struct {
	kubeClient    *kubernetes.Clientset
	dynamicClient dynamic.Interface
	promClient    *prometheus.PrometheusProvider
	storage       *storage.Storage
	config        *CreateStatsTaskConfig
}

func NewCreateStatsTask(ctx context.Context, kubeClient *kubernetes.Clientset, dynamicClient dynamic.Interface, promClient *prometheus.PrometheusProvider, storage *storage.Storage, config *CreateStatsTaskConfig, taskConfig *config.TaskConfig) *CreateStatsTask {
	var createStatsMetadata CreateStatsMetadata
	if err := taskConfig.ConvertMetadataToStruct(&createStatsMetadata); err != nil {
		logging.Errorf(ctx, "Error converting metadata to struct: %v", err)
		return nil
	}

	config.Metadata = createStatsMetadata
	return &CreateStatsTask{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		promClient:    promClient,
		storage:       storage,
		config:        config,
	}
}

func (c *CreateStatsTask) GetCoreTask() any {
	return c
}

func (c *CreateStatsTask) GetName() string {
	return c.config.Name
}

func (c *CreateStatsTask) GetSchedule() string {
	return c.config.Schedule
}

func (c *CreateStatsTask) IsEnabled() bool {
	return c.config.Enabled
}

func (c *CreateStatsTask) Run(ctx context.Context) error {
	ctx = contextutils.WithTask(ctx, c.config.Name)
	ctx = contextutils.WithCluster(ctx, c.config.ClusterID)

	targetNamespace := c.config.TargetNamespace

	startTime := time.Now().UTC()
	logging.Infof(ctx, "Running task: CreateStats")

	workloadObjectsList, err := utils.ListAllWorkloadObjects(ctx, c.kubeClient, targetNamespace)
	if err != nil {
		logging.Errorf(ctx, "Error getting workload list: %v", err)
		return fmt.Errorf("failed to list workloads: %w", err)
	}

	isPSIEnabled := c.isPSIEnabled(ctx)
	logging.Infof(ctx, "PSI is enabled: %v", isPSIEnabled)

	uniqueWorkloads := make(map[string]utils.WorkloadObject)
	filteredCount := 0
	for _, workloadObject := range workloadObjectsList {
		uniqueWorkloads[utils.GetWorkloadKey(workloadObject.GetKind(), workloadObject.GetWorkloadInfo().Namespace, workloadObject.GetWorkloadInfo().Name)] = workloadObject
	}

	logging.Infof(ctx, "Filtered out %d workloads with recent stats (within %d minutes)", filteredCount, utils.RecentStatsLookbackMinutes)
	namespaces := utils.ExtractUniqueNamespaces(uniqueWorkloads)
	logging.Infof(ctx, "Found %d unique namespaces to process: %v", len(namespaces), namespaces)

	namespaceQueryResults, namespaceVsWorkloadMetrics, err := c.promClient.FetchStatsForNamespaces(ctx, c.config.ClusterID, namespaces, isPSIEnabled)
	if err != nil {
		logging.Errorf(ctx, "Error executing batch queries: %v", err)
		return fmt.Errorf("failed to fetch stats for namespaces: %w", err)
	}

	namespaceVsSimpleCPUPredictions, err := utils.PredictSimpleStatsFromTimeSeriesModel(ctx, namespaces, c.promClient.GetClient(), "cpu", isPSIEnabled)
	if err != nil {
		logging.Errorf(ctx, "Error predicting simple CPU stats from time series model: %v", err)
		return fmt.Errorf("failed to predict simple CPU stats: %w", err)
	}
	var namespaceVsSimpleMemoryPredictions map[string]map[string]utils.SimplePrediction
	if !c.config.Metadata.SkipMemory {
		namespaceVsSimpleMemoryPredictions, err = utils.PredictSimpleStatsFromTimeSeriesModel(ctx, namespaces, c.promClient.GetClient(), "memory", isPSIEnabled)
		if err != nil {
			logging.Errorf(ctx, "Error predicting simple memory stats from time series model: %v", err)
			return fmt.Errorf("failed to predict simple memory stats: %w", err)
		}
	}

	workloadHpaCpuMap := make(map[string]bool)
	workloadHpaMemoryMap := make(map[string]bool)
	for key := range uniqueWorkloads {
		workloadHpaCpuMap[key] = false
		workloadHpaMemoryMap[key] = false
	}

	if c.dynamicClient != nil {
		if err = utils.CheckHPAOnCPU(ctx, c.dynamicClient, targetNamespace, workloadHpaCpuMap); err != nil {
			logging.Errorf(ctx, "Error checking HPA on CPU: %v", err)
			return fmt.Errorf("failed to check HPA on CPU: %w", err)
		}
		if err = utils.CheckHPAOnMemory(ctx, c.dynamicClient, targetNamespace, workloadHpaMemoryMap); err != nil {
			logging.Errorf(ctx, "Error checking HPA on memory: %v", err)
			return fmt.Errorf("failed to check HPA on memory: %w", err)
		}
	}

	pdbCache, err := utils.FetchPDBsForNamespaces(ctx, c.kubeClient, namespaces)
	if err != nil {
		logging.Errorf(ctx, "Error fetching PDBs: %v", err)
		return fmt.Errorf("failed to fetch PDBs for namespaces: %w", err)
	}

	// namespaceVsWorkloadPredictions := map[string]map[string]contextutils.WorkloadPrediction{}

	var newStats []*utils.WorkloadStat
	for key, workloadObject := range uniqueWorkloads {
		if stat := c.prepareStatsFromMetrics(
			ctx,
			c.kubeClient,
			key,
			workloadObject,
			namespaceQueryResults,
			namespaceVsWorkloadMetrics,
			namespaceVsSimpleCPUPredictions[workloadObject.GetNamespace()],
			namespaceVsSimpleMemoryPredictions[workloadObject.GetNamespace()],
			workloadHpaCpuMap,
			workloadHpaMemoryMap,
			c.dynamicClient,
			pdbCache,
		); stat != nil {
			newStats = append(newStats, stat)
		}
	}

	if len(newStats) > 0 {
		currentWorkloadIds := make(map[string]struct{}, len(workloadObjectsList))
		for _, w := range workloadObjectsList {
			currentWorkloadIds[utils.GetWorkloadKey(w.GetKind(), w.GetNamespace(), w.GetName())] = struct{}{}
		}
		if deleted, err := c.storage.DeleteStaleWorkloads(c.config.ClusterID, currentWorkloadIds); err != nil {
			logging.Errorf(ctx, "Error deleting stale workloads: %v", err)
		} else if deleted > 0 {
			logging.Infof(ctx, "Deleted %d stale workloads from DB", deleted)
		}

		if err := c.storeStats(ctx, c.config.ClusterID, newStats, startTime); err != nil {
			logging.Errorf(ctx, "Error writing stats to file: %v", err)
			return err
		}
	}

	logging.Infof(ctx, "Task completed in %v", time.Since(startTime))
	return nil
}

func (c *CreateStatsTask) isPSIEnabled(ctx context.Context) bool {
	query := "max(max_over_time(container_pressure_cpu_waiting_seconds_total[1m]))"

	result, _, err := c.promClient.ExecuteQueryWithRetry(ctx, c.config.ClusterID, query, "PSI_CHECK")
	if err != nil {
		logging.Errorf(ctx, "[isPSIEnabled] Error executing PSI query: %v", err)
		return false
	}

	vector := result.(model.Vector)
	return len(vector) != 0
}

func (c *CreateStatsTask) storeStats(ctx context.Context, clusterID string, newStats []*utils.WorkloadStat, generatedAt time.Time) error {
	var allStats = utils.StatsResponse{}
	for _, stat := range newStats {
		allStats.Stats = append(allStats.Stats, *stat)
	}

	if err := c.storage.WriteClusterStats(clusterID, allStats, generatedAt); err != nil {
		return fmt.Errorf("error writing stats for cluster %s: %w", clusterID, err)
	}

	logging.Infof(ctx, "Successfully wrote %d stats for cluster %s", len(allStats.Stats), clusterID)
	return nil
}

func (c *CreateStatsTask) prepareStatsFromMetrics(
	ctx context.Context,
	kubeClient *kubernetes.Clientset,
	workloadKey string,
	workloadObject utils.WorkloadObject,
	nsVsContainerMetrics utils.NamespaceVsContainerMetrics,
	nsVsWorkloadMetrics utils.NamespaceVsWorkloadMetrics,
	workloadContainerKeyVsSimpleCPUPrediction map[string]utils.SimplePrediction,
	workloadContainerKeyVsSimpleMemoryPrediction map[string]utils.SimplePrediction,
	workloadHpaCpuMap map[string]bool,
	workloadHpaMemoryMap map[string]bool,
	dynamicClient dynamic.Interface,
	pdbCache map[string][]policyv1.PodDisruptionBudget,
) *utils.WorkloadStat {
	containerSpecs := workloadObject.GetContainerSpecs(ctx, kubeClient)
	initContainerSpecs := workloadObject.GetInitContainerSpecs(ctx, kubeClient)

	containerResources := c.getAllContainerResourcesFromContainerSpecs(ctx, containerSpecs, initContainerSpecs)
	workloadStat := c.buildBaseWorkloadStat(workloadKey, workloadObject, containerResources, nsVsWorkloadMetrics)

	workloadKeyVsContainerMetrics, exists := nsVsContainerMetrics[workloadObject.GetNamespace()]
	switch {
	case !exists:
		logging.Warnf(ctx, "No batch cache found for namespace %s; storing workload %s as incomplete", workloadObject.GetNamespace(), workloadKey)
		c.markWorkloadStatIncomplete(workloadStat)
	case workloadKeyVsContainerMetrics[workloadKey] == nil:
		logging.Warnf(ctx, "No prometheus metrics found for workload %s; storing incomplete workload stat", workloadKey)
		c.markWorkloadStatIncomplete(workloadStat)
	default:
		metricsStat := utils.BuildContainerStatFromCache(ctx, workloadObject.GetWorkloadInfo(), workloadKeyVsContainerMetrics, containerResources)
		if metricsStat == nil {
			logging.Warnf(ctx, "Could not build container stat for %s; storing incomplete workload stat", workloadKey)
			c.markWorkloadStatIncomplete(workloadStat)
		} else {
			workloadStat.ContainerStats = metricsStat.ContainerStats
			if workloadStat.Replicas <= 0 && metricsStat.Replicas > 0 {
				workloadStat.Replicas = metricsStat.Replicas
			}
		}
	}

	// Check if this workload is horizontally autoscaled on CPU and/or memory
	hpaOnCPU := workloadHpaCpuMap != nil && workloadHpaCpuMap[workloadKey]
	hpaOnMemory := workloadHpaMemoryMap != nil && workloadHpaMemoryMap[workloadKey]
	workloadStat.IsHorizontallyAutoscaledOnCPU = hpaOnCPU
	workloadStat.IsHorizontallyAutoscaledOnMem = hpaOnMemory
	excludedCodes := []types.ExcludedCode{}
	if hpaOnCPU || hpaOnMemory {
		switch {
		case hpaOnCPU && hpaOnMemory:
			excludedCodes = append(excludedCodes, types.ExcludedCodeCPUHPA, types.ExcludedCodeMemoryHPA)
		case hpaOnCPU:
			excludedCodes = append(excludedCodes, types.ExcludedCodeCPUHPA)
		case hpaOnMemory:
			excludedCodes = append(excludedCodes, types.ExcludedCodeMemoryHPA)
		}
		if workloadStat.Metadata == nil {
			workloadStat.Metadata = &types.WorkloadStatMetadata{}
		}
		workloadStat.Metadata.Excluded = true
		workloadStat.Metadata.ExcludedCodes = mergeExcludedCodes(workloadStat.Metadata.ExcludedCodes, excludedCodes)
	}

	if utils.WorkloadHasGPU(containerSpecs, initContainerSpecs) {
		logging.Infof(ctx, "Workload %s has GPU requests/limits, excluding from stats", workloadKey)
		excludedCodes = appendExcludedCode(excludedCodes, types.ExcludedCodeGPUWorkload)
		if workloadStat.Metadata != nil {
			workloadStat.Metadata.ExcludedCodes = mergeExcludedCodes(workloadStat.Metadata.ExcludedCodes, excludedCodes)
			workloadStat.Metadata.IsGPUWorkload = true
		} else {
			workloadStat.Metadata = &types.WorkloadStatMetadata{
				Excluded:      true,
				ExcludedCodes: excludedCodes,
				IsGPUWorkload: true,
			}
		}
	}

	// Detect workload constraints
	if dynamicClient != nil {
		constraints, err := utils.DetectWorkloadConstraints(ctx, kubeClient, dynamicClient, workloadObject, pdbCache)
		if err != nil {
			logging.Errorf(ctx, "Error detecting constraints for workload %s: %v", workloadKey, err)
		} else {
			workloadStat.Constraints = constraints
		}
	}

	for i := range workloadStat.ContainerStats {
		containerStat := &workloadStat.ContainerStats[i]
		workloadContainerKey := utils.GetWorkloadContainerKey(workloadObject.GetKind(), workloadObject.GetNamespace(), workloadObject.GetName(), containerStat.ContainerName)

		if simpleCPUPrediction, exists := workloadContainerKeyVsSimpleCPUPrediction[workloadContainerKey]; exists {
			containerStat.SimplePredictionsCPU = &simpleCPUPrediction
		}

		if simpleMemoryPrediction, exists := workloadContainerKeyVsSimpleMemoryPrediction[workloadContainerKey]; exists {
			containerStat.SimplePredictionsMemory = &simpleMemoryPrediction
		}
	}

	if !workloadStat.IsIncomplete() && !hasCompleteSimplePredictions(workloadStat, containerResources) {
		logging.Warnf(ctx, "Workload %s has missing simple predictions for one or more non-init containers; storing incomplete workload stat", workloadKey)
		c.markWorkloadStatIncomplete(workloadStat)
	}

	switch {
	case workloadStat.Constraints != nil && workloadStat.Constraints.DoNotDisruptAnnotation:
		workloadStat.EvictionRanking = types.EvictionRankingDisabled

	case workloadObject.GetKind() == utils.StatefulSetKind || workloadStat.Replicas == 1:
		workloadStat.EvictionRanking = types.EvictionRankingMedium

	default:
		workloadStat.EvictionRanking = types.EvictionRankingHigh
	}

	logging.Infof(ctx, "Successfully created container-level stat for %s with %d containers", workloadKey, len(workloadStat.ContainerStats))

	return workloadStat
}

func (c *CreateStatsTask) buildBaseWorkloadStat(
	workloadKey string,
	workloadObj utils.WorkloadObject,
	containerResources []utils.OriginalContainerResources,
	nsVsWorkloadMetrics utils.NamespaceVsWorkloadMetrics,
) *utils.WorkloadStat {
	workloadStat := &utils.WorkloadStat{
		WorkloadIdentifier:         workloadKey,
		Kind:                       workloadObj.GetKind(),
		Namespace:                  workloadObj.GetNamespace(),
		Name:                       workloadObj.GetName(),
		CreationTime:               workloadObj.GetCreationTime(),
		UpdatedAt:                  time.Now(),
		Replicas:                   workloadObj.GetReplicas(),
		ContainerStats:             buildIncompleteContainerStats(containerResources),
		OriginalContainerResources: containerResources,
	}

	if workloadMetricsByNamespace, exists := nsVsWorkloadMetrics[workloadObj.GetNamespace()]; exists {
		if workloadMetrics, exists := workloadMetricsByNamespace[workloadStat.WorkloadIdentifier]; exists && workloadMetrics.MedianReplicas > 0 {
			workloadStat.Replicas = int32(workloadMetrics.MedianReplicas)
		}
	}

	return workloadStat
}

func buildIncompleteContainerStats(containerResources []utils.OriginalContainerResources) []utils.ContainerStats {
	containerStats := make([]utils.ContainerStats, 0, len(containerResources))
	for _, containerRes := range containerResources {
		containerStats = append(containerStats, utils.ContainerStats{
			ContainerName: containerRes.Name,
			ContainerType: containerRes.Type,
		})
	}
	return containerStats
}

func (c *CreateStatsTask) markWorkloadStatIncomplete(workloadStat *utils.WorkloadStat) {
	if workloadStat == nil {
		return
	}
	workloadStat.ContainerStats = buildIncompleteContainerStats(workloadStat.OriginalContainerResources)
	if workloadStat.Metadata == nil {
		workloadStat.Metadata = &types.WorkloadStatMetadata{}
	}
	workloadStat.Metadata.Incomplete = true
	workloadStat.Metadata.Excluded = true
	workloadStat.Metadata.ExcludedCodes = appendExcludedCode(workloadStat.Metadata.ExcludedCodes, types.ExcludedCodeIncomplete)
}

func appendExcludedCode(codes []types.ExcludedCode, code types.ExcludedCode) []types.ExcludedCode {
	for _, existing := range codes {
		if existing == code {
			return codes
		}
	}
	return append(codes, code)
}

func mergeExcludedCodes(existing, additions []types.ExcludedCode) []types.ExcludedCode {
	merged := existing
	for _, code := range additions {
		merged = appendExcludedCode(merged, code)
	}
	return merged
}

func hasCompleteSimplePredictions(workloadStat *utils.WorkloadStat, containerResources []utils.OriginalContainerResources) bool {
	if workloadStat == nil {
		return false
	}

	containerStatsByName := make(map[string]*utils.ContainerStats, len(workloadStat.ContainerStats))
	for i := range workloadStat.ContainerStats {
		containerStat := &workloadStat.ContainerStats[i]
		containerStatsByName[containerStat.ContainerName] = containerStat
	}

	for _, containerRes := range containerResources {
		if containerRes.Type == types.InitContainer {
			continue
		}

		containerStat, exists := containerStatsByName[containerRes.Name]
		if !exists {
			return false
		}
		if containerStat.SimplePredictionsCPU == nil || containerStat.SimplePredictionsMemory == nil {
			return false
		}
	}

	return true
}

func (c *CreateStatsTask) getAllContainerResourcesFromContainerSpecs(_ context.Context, containerSpecs []corev1.Container, initContainerSpecs []corev1.Container) []utils.OriginalContainerResources {
	containerResources := make([]utils.OriginalContainerResources, 0, len(containerSpecs)+len(initContainerSpecs))

	for _, container := range containerSpecs {
		containerRes := utils.OriginalContainerResources{
			Name: container.Name,
			Type: types.AppContainer,
		}
		c.setResourceRequestAndLimit(&container, &containerRes)
		containerResources = append(containerResources, containerRes)
	}

	for _, container := range initContainerSpecs {
		var ContainerType types.ContainerType
		if utils.IsSidecarContainer(container) {
			ContainerType = types.SidecarContainer
		} else {
			ContainerType = types.InitContainer
		}

		containerRes := utils.OriginalContainerResources{
			Name: container.Name,
			Type: ContainerType,
		}
		c.setResourceRequestAndLimit(&container, &containerRes)
		containerResources = append(containerResources, containerRes)
	}

	return containerResources
}

func (c *CreateStatsTask) setResourceRequestAndLimit(container *corev1.Container, containerRes *utils.OriginalContainerResources) {
	if cpuRequest := container.Resources.Requests[corev1.ResourceCPU]; !cpuRequest.IsZero() {
		containerRes.CPURequest = float64(cpuRequest.MilliValue()) / 1000.0
	}
	if cpuLimit := container.Resources.Limits[corev1.ResourceCPU]; !cpuLimit.IsZero() {
		containerRes.CPULimit = float64(cpuLimit.MilliValue()) / 1000.0
	}

	if memRequest := container.Resources.Requests[corev1.ResourceMemory]; !memRequest.IsZero() {
		containerRes.MemoryRequest = float64(memRequest.Value()) / utils.BytesPerMB
	}
	if memLimit := container.Resources.Limits[corev1.ResourceMemory]; !memLimit.IsZero() {
		containerRes.MemoryLimit = float64(memLimit.Value()) / utils.BytesPerMB
	}
}
