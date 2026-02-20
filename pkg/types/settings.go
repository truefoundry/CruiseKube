package types

type AppSettings struct {
	CPUPricePerCorePerHour    float64 `json:"cpuPricePerCorePerHour"`
	MemoryPricePerGbPerHour   float64 `json:"memoryPricePerGbPerHour"`
	DisruptionWindowStartCron string  `json:"disruptionWindowStartCron"`
	DisruptionWindowEndCron   string  `json:"disruptionWindowEndCron"`
	DisruptionWindowEnabled   bool    `json:"disruptionWindowEnabled"`
}
