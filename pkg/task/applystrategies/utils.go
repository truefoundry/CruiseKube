package applystrategies

import (
	"github.com/truefoundry/autopilot-oss/pkg/task/utils"
	"github.com/truefoundry/autopilot-oss/pkg/types"
)

func isEvictionExcludedPod(podInfo *utils.PodInfo, evictionRanking types.EvictionRanking) bool {
	if podInfo.Stats.Constraints == nil {
		return false
	}
	if podInfo.Stats.Constraints.ExcludedAnnotation {
		return true
	}
	if podInfo.WorkloadKind == utils.DaemonSetKind {
		return true
	}
	if evictionRanking == types.EvictionRankingDisabled {
		return true
	}
	return false
}
