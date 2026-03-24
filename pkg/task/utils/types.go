package utils

import (
	"github.com/truefoundry/cruisekube/pkg/types"

	"k8s.io/apimachinery/pkg/labels"
)

type WorkloadInfo struct {
	Kind      string
	Namespace string
	Name      string
}

type PodKey struct {
	Namespace string
	PodName   string
}

type WorkloadLabelSelectorList struct {
	Kind      string
	Namespace string
	Name      string
	Selector  labels.Selector
}

type RawBatchResult map[string]float64

type WorkloadPrediction struct {
	Median float64
	P90    float64
	P95    float64
	P99    float64
}

type SimpleTimeSeriesData struct {
	EntityName string    `json:"entity_name"`
	Timestamps []string  `json:"timestamps"`
	Values     []float64 `json:"values"`
}

type SimplePredictionResponse struct {
	EntityName        string  `json:"entity_name"`
	WeeklyPrediction  float64 `json:"weekly_prediction"`
	HourlyPrediction  float64 `json:"hourly_prediction"`
	CurrentPrediction float64 `json:"current_prediction"`
	MaxValue          float64 `json:"max_value"`
}

type SimplePrediction = types.SimplePrediction

type PodMetrics struct {
	TotalRecommendedCPU    float64
	TotalRecommendedMemory float64
	MaxRestCPU             float64
	MaxRestMemory          float64
	EvictionRanking        types.EvictionRanking
}

type PodContainerRecommendation struct {
	PodInfo       PodInfo `json:"pod_info"`
	ContainerName string  `json:"container_name"`
	CPU           float64 `json:"recommended_cpu"`
	Memory        float64 `json:"recommended_memory"`
	Evict         bool    `json:"evict"`
}

type NodeOptimizationData struct {
	NodeName          string
	AllocatableCPU    float64
	AllocatableMemory float64
	PodInfos          []PodInfo
}

type OptimizationResult struct {
	PodContainerRecommendations []PodContainerRecommendation `json:"pod_container_recommendations"`
	MaxRestCPU                  float64                      `json:"max_rest"`
	MaxRestMemory               float64                      `json:"max_rest_memory"`
}
