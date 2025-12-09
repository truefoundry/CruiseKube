package utils

type WorkloadKeyVsWorkloadMetrics map[string]WorkloadMetrics

type WorkloadMetrics struct {
	MedianReplicas float64
}

type ContainerMetrics struct {
	CPUP50           float64
	CPUP75           float64
	CPUP90           float64
	CPUP95           float64
	CPUP99           float64
	CPUP999          float64
	CPUMax           float64
	PSIAdjustedUsage *PSIAdjustedUsage

	StartupCPUMax    float64
	NonStartupCPUMax float64

	MemoryP50  float64
	MemoryP75  float64
	MemoryP90  float64
	MemoryP95  float64
	MemoryP99  float64
	MemoryP999 float64
	MemoryMax  float64

	OOMMemory float64

	Memory7Day Memory7DayStats
	CPU7Day    CPU7DayStats

	MedianReplicas float64

	HasCPUData    bool
	HasMemoryData bool
}

type PSIAdjustedUsage struct {
	CPUP50  float64
	CPUP75  float64
	CPUP90  float64
	CPUP95  float64
	CPUP99  float64
	CPUP999 float64
	CPUMax  float64
}

type ContainerNameVsContainerMetrics map[string]*ContainerMetrics

type WorkloadKeyVsContainerMetrics map[string]ContainerNameVsContainerMetrics

type NamespaceVsContainerMetrics map[string]WorkloadKeyVsContainerMetrics

type NamespaceVsWorkloadMetrics map[string]WorkloadKeyVsWorkloadMetrics
