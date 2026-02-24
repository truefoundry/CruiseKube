package utils

import "time"

const (
	ExcludedAnnotation                   = "cruisekube.truefoundry.com/excluded"
	ContinuousOptimizationRatioThreshold = 3.0
	ContinuousOptimizationDiffThreshold  = 0.001
	BytesToMBDivisor                     = 1000_000
	CPUClampValue                        = 20.0
)

const (
	HeadroomGroupCPULabel    = "cruisekube.com/headroom-group-cpu"
	HeadroomGroupMemoryLabel = "cruisekube.com/headroom-group-memory"
	HeadroomCategoryHigh     = "high"
	HeadroomCategoryMedium   = "medium"
	HeadroomCategoryLow      = "low"
)

const (
	SpikeCPUHighThresholdCores   = 1.0
	SpikeCPUMediumThresholdCores = 0.1
	SpikeMemoryHighThresholdMB   = 10000
	SpikeMemoryMediumThresholdMB = 1000
)

const (
	TrueValue  = "true"
	FalseValue = "false"
)

const (
	AnnotationModified          = "cruisekube.truefoundry.com/modified"
	AnnotationPDBMaxUnavailable = "cruisekube.truefoundry.com/pdb.maxUnavailable"
	AnnotationPDBMinAvailable   = "cruisekube.truefoundry.com/pdb.minAvailable"
)

var doNotDisruptAnnotations = map[string]string{
	"cluster-autoscaler.kubernetes.io/safe-to-evict": FalseValue,
	"karpenter.sh/do-not-evict":                      TrueValue,
	"karpenter.sh/do-not-disrupt":                    TrueValue,
}

func GetDoNotDisruptAnnotations() map[string]string {
	result := make(map[string]string, len(doNotDisruptAnnotations))
	for k, v := range doNotDisruptAnnotations {
		result[k] = v
	}
	return result
}

const (
	DeploymentKind  = "Deployment"
	StatefulSetKind = "StatefulSet"
	DaemonSetKind   = "DaemonSet"
	ReplicaSetKind  = "ReplicaSet"
	RolloutKind     = "Rollout"
)

const (
	CPULookbackWindow        = 10 * time.Minute
	CPU7DayLookbackWindow    = 7 * 24 * time.Hour
	ReplicaLookbackWindow    = 7 * 24 * time.Hour
	MemoryLookbackWindow     = 30 * time.Minute
	Memory7DayLookbackWindow = 7 * 24 * time.Hour

	MLLookbackWindow    = 7 * 24 * time.Hour
	RateIntervalMinutes = 1
	ResolutionMinutes   = 1
	CPUDecimalScale     = 1000.0

	BytesPerMB                 = 1_000_000
	MemoryDecimalPlaces        = 1
	RecentStatsLookbackMinutes = 10
)
