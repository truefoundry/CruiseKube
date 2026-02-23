package types

type AppSettings struct {
	CPUPricePerCorePerHour    *float64 `json:"cpuPricePerCorePerHour,omitempty"`
	MemoryPricePerGBPerHour   *float64 `json:"memoryPricePerGbPerHour,omitempty"`
	DisruptionWindowStartCron string   `json:"disruptionWindowStartCron,omitempty"`
	DisruptionWindowEndCron   string   `json:"disruptionWindowEndCron,omitempty"`
	DisruptionWindowEnabled   *bool    `json:"disruptionWindowEnabled,omitempty"`
}
