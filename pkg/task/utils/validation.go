package utils

const MinimumCPURecommendation = 0.001

const MinimumMemoryRecommendation = 16.0

func EnforceMinimumCPU(cpu float64) float64 {
	if cpu != cpu {
		return MinimumCPURecommendation
	}
	if cpu < 0 {
		return MinimumCPURecommendation
	}
	if cpu < MinimumCPURecommendation {
		return MinimumCPURecommendation
	}
	return cpu
}

func EnforceMinimumMemory(memory float64) float64 {
	if memory != memory {
		return MinimumMemoryRecommendation
	}
	if memory < 0 {
		return MinimumMemoryRecommendation
	}
	if memory < MinimumMemoryRecommendation {
		return MinimumMemoryRecommendation
	}
	return memory
}
