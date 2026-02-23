package types

type ClusterSettings struct {
	CPUPricePerCorePerHour  float64            `json:"cpuPricePerCorePerHour,omitempty"`
	MemoryPricePerGBPerHour float64            `json:"memoryPricePerGbPerHour,omitempty"`
	DisruptionWindowEnabled bool               `json:"disruptionWindowEnabled"`
	DisruptionWindows       []DisruptionWindow `json:"disruptionWindows,omitempty"`
}
