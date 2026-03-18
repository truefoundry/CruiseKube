package types

type OverviewCoverageBreakdown struct {
	Optimizable            int `json:"optimizable"`
	NonOptimizable         int `json:"nonOptimizable"`
	OptimizableButExcluded int `json:"optimizableButExcluded"`
	Total                  int `json:"total"`
}

// OverviewCoverageBreakdownTypo mirrors contract keys where "enabed" is expected.
type OverviewCoverageBreakdownTypo struct {
	Enabed   float64 `json:"enabed"`
	Disabled float64 `json:"disabled"`
}

type OverviewCoverage struct {
	Adoption       OverviewCoverageBreakdown     `json:"adoption"`
	CPUCoverage    OverviewCoverageBreakdownTypo `json:"cpuCoverage"`
	MemoryCoverage OverviewCoverageBreakdownTypo `json:"memoryCoverage"`
}

type OverviewResourceStats struct {
	Allocatable       float64 `json:"allocatable"`
	Requested         float64 `json:"requested"`
	WorkloadRequested float64 `json:"workloadRequested"`
	Usage             float64 `json:"usage"`
	Recommended       float64 `json:"recommended"`
}

type OverviewResponse struct {
	CurrentMonthlyCost int                   `json:"currentMonthlyCost"`
	CurrentSavings     int                   `json:"currentSavings"`
	PossibleSavings    int                   `json:"possibleSavings"`
	ClusterUtilisation float64               `json:"clusterUtilisation"`
	NodeCount          int                   `json:"nodeCount"`
	Coverage           OverviewCoverage      `json:"coverage"`
	CPUStats           OverviewResourceStats `json:"cpuStats"`
	MemoryStats        OverviewResourceStats `json:"memoryStats"`
}
