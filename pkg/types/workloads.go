package types

// WorkloadOverridesEffective is the effective overrides for a workload (what the API returns under "overrides").
type WorkloadOverridesEffective struct {
	EvictionRanking   EvictionRanking    `json:"eviction_ranking"`
	Enabled           bool               `json:"enabled"`
	DisruptionWindows []DisruptionWindow `json:"disruption_windows,omitempty"`
}

type WorkloadOverrideInfo struct {
	WorkloadID string                      `json:"workload_id"`
	Name       string                      `json:"name"`
	Namespace  string                      `json:"namespace"`
	Kind       string                      `json:"kind"`
	Overrides  *WorkloadOverridesEffective `json:"overrides"`
}

// EffectiveEnabled returns the effective enabled flag (default false if Overrides is nil).
func (w *WorkloadOverrideInfo) EffectiveEnabled() bool {
	if w == nil || w.Overrides == nil {
		return false
	}
	return w.Overrides.Enabled
}

// EffectiveEvictionRanking returns the effective eviction ranking (default EvictionRankingMedium if Overrides is nil).
func (w *WorkloadOverrideInfo) EffectiveEvictionRanking() EvictionRanking {
	if w == nil || w.Overrides == nil {
		return EvictionRankingMedium
	}
	return w.Overrides.EvictionRanking
}

type WorkloadAnalysisItem struct {
	WorkloadType      string        `json:"workload_type"`
	WorkloadNamespace string        `json:"workload_namespace"`
	WorkloadName      string        `json:"workload_name"`
	ContainerName     string        `json:"container_name"`
	ContainerType     ContainerType `json:"container_type"`
	CPUUsage7Days     string        `json:"cpu_usage_7_days"`
	SpikeRange        float64       `json:"spike_range"`
	RequestGap        float64       `json:"request_gap"`
	BlockingKarpenter string        `json:"blocking_karpenter"`
}

type KillswitchResponse struct {
	Message                string   `json:"message"`
	DeletedMutatingWebhook bool     `json:"deleted_mutating_webhook"`
	PodsAnalyzed           int      `json:"pods_analyzed"`
	PodsKilled             int      `json:"pods_killed"`
	KilledPods             []string `json:"killed_pods"`
	Errors                 []string `json:"errors,omitempty"`
}

type RecommendationAnalysisItem struct {
	WorkloadType           string  `json:"workload_type"`
	WorkloadNamespace      string  `json:"workload_namespace"`
	WorkloadName           string  `json:"workload_name"`
	PodName                string  `json:"pod_name"`
	ContainerName          string  `json:"container_name"`
	CPUUsage7Days          string  `json:"cpu_usage_7_days"`
	SpikeRange             float64 `json:"spike_range"`
	RequestGap             float64 `json:"request_gap"`
	BlockingKarpenter      string  `json:"blocking_karpenter"`
	NodeName               string  `json:"node_name"`
	CurrentRequestedCPU    float64 `json:"current_requested_cpu"`
	RecommendedCPU         float64 `json:"recommended_cpu"`
	CPUDifference          float64 `json:"cpu_difference"`
	CurrentRequestedMemory float64 `json:"current_requested_memory"`
	RecommendedMemory      float64 `json:"recommended_memory"`
	MemoryDifference       float64 `json:"memory_difference"`
}

type RecommendationSummary struct {
	TotalCurrentCPURequests    float64 `json:"total_current_cpu_requests"`
	TotalCPUDifferences        float64 `json:"total_cpu_differences"`
	TotalCurrentMemoryRequests float64 `json:"total_current_memory_requests"`
	TotalMemoryDifferences     float64 `json:"total_memory_differences"`
}

type RecommendationAnalysisResponse struct {
	Analysis []RecommendationAnalysisItem `json:"analysis"`
	Summary  RecommendationSummary        `json:"summary"`
}
