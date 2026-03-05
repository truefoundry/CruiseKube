package types

// OverviewCoverageBreakdown represents enabled/disabled split values in percentage terms.
type OverviewCoverageBreakdown struct {
	Enabled  float64 `json:"enabled"`
	Disabled float64 `json:"disabled"`
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
	Allocatable float64 `json:"allocatable"`
	Requested   float64 `json:"requested"`
	Usage       float64 `json:"usage"`
	Recommended float64 `json:"recommended"`
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
