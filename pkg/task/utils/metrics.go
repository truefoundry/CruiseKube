package utils

type WorkloadKeyVsWorkloadMetrics map[string]WorkloadMetrics

type WorkloadMetrics struct {
	MedianReplicas float64
}

type ContainerMetrics struct {
	CPUP75           float64
	PSIAdjustedUsage *PSIAdjustedUsage

	MemoryP75  float64

	OOMMemory float64

	Memory7Day Memory7DayStats

	MedianReplicas float64

	HasCPUData    bool
	HasMemoryData bool
}

type PSIAdjustedUsage struct {
	CPUP75  float64
}

type ContainerNameVsContainerMetrics map[string]*ContainerMetrics

type WorkloadKeyVsContainerMetrics map[string]ContainerNameVsContainerMetrics

type NamespaceVsContainerMetrics map[string]WorkloadKeyVsContainerMetrics

type NamespaceVsWorkloadMetrics map[string]WorkloadKeyVsWorkloadMetrics
