package utils

const MinimumCPURecommendation = 0.001

const MinimumMemoryRecommendation = 16.0

func EnforceMinimumCPU(cpu float64) float64 {
	return max(MinimumCPURecommendation, cpu)
}

func EnforceMinimumMemory(memory float64) float64 {
	return max(MinimumMemoryRecommendation, memory)
}
