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
	DollarCurrentCost     float64             `json:"dollarCurrentCost"`
	DollarCurrentSavings  float64             `json:"dollarCurrentSavings"`
	DollarPossibleSavings float64             `json:"dollarPossibleSavings"`
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
	CriticalityLevel   string                     `json:"criticalityLevel"`
	CruiseEnabled      bool                       `json:"cruiseEnabled"`
	DisruptionSchedule []DisruptionScheduleWindow `json:"disruptionSchedule"`
	InDisruptionWindow bool                       `json:"inDisruptionWindow"`
	HPAEnabled         bool                       `json:"hpaEnabled"`
	ExcludedCodes      []ExcludedCode             `json:"excludedCodes,omitempty"`
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
	ScaledDown                bool                       `json:"scaledDown"`
	Constraints               WorkloadSummaryConstraints `json:"constraints"`
	CPU                       WorkloadCPU                `json:"cpu"`
	Memory                    WorkloadMemory             `json:"memory"`
	DollarSavingsPerMonth     float64                    `json:"dollarSavingsPerMonth"`
	DollarExpenditurePerMonth float64                    `json:"dollarExpenditurePerMonth"`
	Config                    WorkloadConfig             `json:"config"`
}

type WorkloadSummaryResponse struct {
	ImpactSummary   ImpactSummary    `json:"impactSummary"`
	WorkloadDetails []WorkloadDetail `json:"workloadDetails"`
}
