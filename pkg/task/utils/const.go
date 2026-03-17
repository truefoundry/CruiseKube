package utils

import "time"

const (
	ExcludedAnnotation = "cruisekube.truefoundry.com/excluded"
	BytesToMBDivisor   = 1000_000
	CPUClampValue      = 20.0
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
