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
		restMemory := restMemoryForContainer(c)
		if restCPU >= 0 {
			maxRestCPU = math.Max(maxRestCPU, restCPU)
		}
		if restMemory >= 0 {
			maxRestMemory = math.Max(maxRestMemory, restMemory)
		}
	}

	return cpuCategory(maxRestCPU), memoryCategory(maxRestMemory)
}

func restCPUForContainer(c *types.ContainerStats) float64 {
	if c.CPUStats == nil {
		return -1
	}

	p75 := c.CPUStats.P75
	if c.PSIAdjustedUsage != nil {
		p75 = c.PSIAdjustedUsage.P75
	}

	var maxCPU float64
	if c.SimplePredictionsCPU != nil {
		maxCPU = c.SimplePredictionsCPU.MaxValue
	} else {
		maxCPU = c.CPUStats.Max
	}
	if maxCPU < p75 {
		return 0
	}
	return maxCPU - p75
}

func restMemoryForContainer(c *types.ContainerStats) float64 {
	if c.MemoryStats == nil {
		return -1
	}

	p75 := c.MemoryStats.P75
	if c.MemoryStats.OOMMemory > 0 && c.MemoryStats.OOMMemory > p75 {
		return 0
	}

	var maxMemory float64
	if c.SimplePredictionsMemory != nil {
		maxMemory = c.SimplePredictionsMemory.MaxValue
	} else {
		maxMemory = c.MemoryStats.Max
	}
	if maxMemory < p75 {
		return 0
	}
	return maxMemory - p75
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
