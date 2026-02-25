package utils

import (
	"math"

	"github.com/truefoundry/cruisekube/pkg/types"
)

func HeadroomCategories(stat *WorkloadStat) (string, string) {
	if stat == nil || stat.ContainerStats == nil {
		return "", ""
	}
	var maxRestCPU, maxRestMemory float64
	for i := range stat.ContainerStats {
		c := &stat.ContainerStats[i]
		if c.ContainerType == types.InitContainer {
			continue
		}
		restCPU := restCPUForContainer(c)
		restMem := restMemoryForContainer(c)
		if restCPU >= 0 {
			maxRestCPU = math.Max(maxRestCPU, restCPU)
		}
		if restMem >= 0 {
			maxRestMemory = math.Max(maxRestMemory, restMem)
		}
	}
	return cpuCategory(maxRestCPU), memoryCategory(maxRestMemory)
}

func restCPUForContainer(c *types.ContainerStats) float64 {
	if c.CPUStats == nil {
		return -1
	}
	px := c.CPUStats.P75
	if c.PSIAdjustedUsage != nil {
		px = c.PSIAdjustedUsage.P75
	}
	var pmax float64
	if c.SimplePredictionsCPU != nil {
		pmax = c.SimplePredictionsCPU.MaxValue
	} else {
		pmax = c.CPUStats.Max
	}
	if pmax < px {
		return 0
	}
	return pmax - px
}

func restMemoryForContainer(c *types.ContainerStats) float64 {
	if c.MemoryStats == nil {
		return -1
	}
	p75 := c.MemoryStats.P75
	if c.MemoryStats.OOMMemory > 0 && c.MemoryStats.OOMMemory > p75 {
		return 0
	}
	var high float64
	if c.SimplePredictionsMemory != nil {
		high = c.SimplePredictionsMemory.MaxValue
	} else {
		high = c.MemoryStats.Max
	}
	if high < p75 {
		return 0
	}
	return high - p75
}

func cpuCategory(restCPU float64) string {
	if restCPU > SpikeCPUHighThresholdCores {
		return HeadroomCategoryHigh
	}
	if restCPU >= SpikeCPUMediumThresholdCores {
		return HeadroomCategoryMedium
	}
	return HeadroomCategoryLow
}

func memoryCategory(restMemoryMB float64) string {
	if restMemoryMB > SpikeMemoryHighThresholdMB {
		return HeadroomCategoryHigh
	}
	if restMemoryMB >= SpikeMemoryMediumThresholdMB {
		return HeadroomCategoryMedium
	}
	return HeadroomCategoryLow
}
