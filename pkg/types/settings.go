package types

type ClusterSettings struct {
	CPUPricePerCorePerHour  float64 `json:"cpuPricePerCorePerHour,omitempty"`
	MemoryPricePerGBPerHour float64 `json:"memoryPricePerGBPerHour,omitempty"`
}
