package types

type ClusterResourceDTO struct {
	Utilised    float64 `json:"utilised"`
	Requested   float64 `json:"requested"`
	Allocatable float64 `json:"allocatable"`
}

type ClusterResourcesDTO struct {
	CPU    ClusterResourceDTO `json:"cpu"`
	Memory ClusterResourceDTO `json:"memory"`
}

type ImpactSummary struct {
	DollarCurrentCost     int                 `json:"dollarCurrentCost"`
	DollarCurrentSavings  int                 `json:"dollarCurrentSavings"`
	DollarPossibleSavings int                 `json:"dollarPossibleSavings"`
	ClusterResources      ClusterResourcesDTO `json:"clusterResources"`
}

type WorkloadSummaryConstraints struct {
	BlockingConsolidation    bool `json:"blockingConsolidation"`
	PDB                      bool `json:"pdb"`
	DoNotDisruptAnnotation   bool `json:"doNotDisruptAnnotation"`
	Volume                   bool `json:"volume"`
	Affinity                 bool `json:"affinity"`
	TopologySpreadConstraint bool `json:"topologySpreadConstraint"`
	PodAntiAffinity          bool `json:"podAntiAffinity"`
	ExcludedAnnotation       bool `json:"excludedAnnotation"`
	IsGPUWorkload            bool `json:"isGPUWorkload"`
}

type CPURecommended struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Change float64 `json:"change"`
}

type MemoryRecommended struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Change float64 `json:"change"`
}

type DisruptionScheduleWindow struct {
	WindowStartCron string `json:"windowStartCron"`
	WindowEndCron   string `json:"windowEndCron"`
}

type WorkloadConfig struct {
	Priority           string                     `json:"priority"`
	CruiseEnabled      bool                       `json:"cruiseEnabled"`
	DisruptionSchedule []DisruptionScheduleWindow `json:"disruptionSchedule"`
	InDisruptionWindow bool                       `json:"inDisruptionWindow"`
}

type WorkloadCPU struct {
	CurrentPerPod float64        `json:"current"`
	Recommended   CPURecommended `json:"recommended"`
}

type WorkloadMemory struct {
	CurrentPerPod float64           `json:"current"`
	Recommended   MemoryRecommended `json:"recommended"`
}

type WorkloadDetail struct {
	WorkloadID                string                     `json:"workloadID"`
	Kind                      string                     `json:"kind"`
	Namespace                 string                     `json:"namespace"`
	Name                      string                     `json:"name"`
	UpdatedAt                 int64                      `json:"updatedAt"`
	PodsCount                 int                        `json:"podsCount"`
	Constraints               WorkloadSummaryConstraints `json:"constraints"`
	CPU                       WorkloadCPU                `json:"cpu"`
	Memory                    WorkloadMemory             `json:"memory"`
	DollarSavingsPerMonth     int                        `json:"dollarSavingsPerMonth"`
	DollarExpenditurePerMonth int                        `json:"dollarExpenditurePerMonth"`
	Config                    WorkloadConfig             `json:"config"`
}

type WorkloadSummaryResponse struct {
	ImpactSummary   ImpactSummary    `json:"impactSummary"`
	WorkloadDetails []WorkloadDetail `json:"workloadDetails"`
}
