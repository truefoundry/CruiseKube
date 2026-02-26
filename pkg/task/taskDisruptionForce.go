package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truefoundry/cruisekube/pkg/audit"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type ReconcileState int

const (
	StateIn          ReconcileState = iota + 1
	StateAboutToExit                // currently in, about to be out in the next run
	StateOut
)

type DisruptionForceTaskConfig struct {
	Name                     string
	Enabled                  bool
	Schedule                 string
	ClusterID                string
	IsClusterWriteAuthorized bool
}

type DisruptionForceTask struct {
	kubeClient *kubernetes.Clientset
	storage    *storage.Storage
	config     *DisruptionForceTaskConfig
}

func NewDisruptionForceTask(_ context.Context, kubeClient *kubernetes.Clientset, storage *storage.Storage, taskConfig *DisruptionForceTaskConfig) *DisruptionForceTask {
	return &DisruptionForceTask{
		kubeClient: kubeClient,
		storage:    storage,
		config:     taskConfig,
	}
}

func (t *DisruptionForceTask) GetCoreTask() any {
	return t
}

func (t *DisruptionForceTask) GetName() string {
	return t.config.Name
}

func (t *DisruptionForceTask) GetSchedule() string {
	return t.config.Schedule
}

func (t *DisruptionForceTask) IsEnabled() bool {
	return t.config.Enabled
}

func (t *DisruptionForceTask) Run(ctx context.Context) error {
	ctx = contextutils.WithTask(ctx, t.config.Name)
	ctx = contextutils.WithCluster(ctx, t.config.ClusterID)

	if !t.config.IsClusterWriteAuthorized {
		logging.Infof(ctx, "Cluster %s is not write authorized, skipping DisruptionForceTask", t.config.ClusterID)
		return nil
	}

	logging.Infof(ctx, "Running disruption force task")

	now := time.Now()

	scheduleDuration, err := time.ParseDuration(t.config.Schedule)
	if err != nil {
		return fmt.Errorf("failed to parse task schedule %q: %w", t.config.Schedule, err)
	}

	workloads, err := t.storage.GetWorkloadsInCluster(t.config.ClusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get workloads: %v", err)
		return fmt.Errorf("failed to get workloads: %w", err)
	}

	allPDBs, err := t.kubeClient.PolicyV1().PodDisruptionBudgets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Failed to list PDBs: %v", err)
		return fmt.Errorf("failed to list PDBs: %w", err)
	}

	pdbsByNamespace := make(map[string][]policyv1.PodDisruptionBudget)
	for i := range allPDBs.Items {
		pdb := &allPDBs.Items[i]
		pdbsByNamespace[pdb.Namespace] = append(pdbsByNamespace[pdb.Namespace], *pdb)
	}

	reconciledPods := 0
	reconciledPDBs := 0
	blockingCount := 0

	for _, w := range workloads {
		stat := w.GetStat()
		if stat == nil || stat.Constraints == nil || !stat.Constraints.DoNotDisruptAnnotation {
			continue
		}
		blockingCount++

		overrides := w.GetOverrides()
		effectiveState := StateOut
		if overrides != nil && len(overrides.DisruptionWindows) > 0 {
			logging.Debugf(ctx, "Workload %s/%s/%s has %d disruption window(s), computing state",
				stat.Kind, stat.Namespace, stat.Name, len(overrides.DisruptionWindows))
			effectiveState = t.computeStateFromWindows(ctx, now, scheduleDuration, overrides.DisruptionWindows)
		}

		workloadInfo := utils.WorkloadInfo{Kind: stat.Kind, Namespace: stat.Namespace, Name: stat.Name}

		workloadObj, err := utils.GetWorkloadObject(ctx, t.kubeClient, stat.Kind, stat.Namespace, stat.Name)
		if err != nil {
			logging.Errorf(ctx, "Failed to get workload %s/%s/%s: %v", stat.Kind, stat.Namespace, stat.Name, err)
			continue
		}

		selector, err := workloadObj.GetSelector()
		if err != nil {
			logging.Errorf(ctx, "Failed to get selector for workload %s/%s/%s: %v", stat.Kind, stat.Namespace, stat.Name, err)
		} else {
			pods, err := utils.GetPods(ctx, t.kubeClient, stat.Namespace, selector)
			if err != nil {
				logging.Errorf(ctx, "Failed to list pods for workload %s/%s/%s: %v", stat.Kind, stat.Namespace, stat.Name, err)
			} else {
				for i := range pods.Items {
					pod := &pods.Items[i]

					hasModifiedMarker := pod.Annotations != nil && pod.Annotations[utils.AnnotationModified] == utils.TrueValue
					if !t.hasBlockingAnnotations(pod) && !hasModifiedMarker {
						continue
					}

					modified, err := t.reconcilePod(ctx, pod, &workloadInfo, effectiveState)
					if err != nil {
						logging.Errorf(ctx, "Failed to reconcile pod %s/%s: %v", pod.Namespace, pod.Name, err)
						continue
					}
					if modified {
						reconciledPods++
					}
				}
			}
		}

		podTemplate := utils.GetPodTemplateSpec(workloadObj)
		matchingPDBs := utils.FindMatchingPDBs(ctx, podTemplate.Labels, pdbsByNamespace[stat.Namespace])
		for _, pdb := range matchingPDBs {
			modified, err := t.reconcilePDB(ctx, pdb, effectiveState)
			if err != nil {
				logging.Errorf(ctx, "Failed to reconcile PDB %s/%s: %v", pdb.Namespace, pdb.Name, err)
				continue
			}
			if modified {
				reconciledPDBs++
			}
		}
	}

	logging.Infof(ctx, "Total workloads with blocking consolidation: %d", blockingCount)
	logging.Infof(ctx, "Disruption force task completed: reconciled %d pods, %d PDBs modified", reconciledPods, reconciledPDBs)
	return nil
}

func (t *DisruptionForceTask) computeStateFromWindows(ctx context.Context, now time.Time, scheduleDuration time.Duration, windows []types.DisruptionWindow) ReconcileState {
	nextRun := now.Add(scheduleDuration)

	inNow, inNext := false, false
	for _, w := range windows {
		if utils.InEvictionWindow(ctx, w.StartCron, w.EndCron, now) {
			inNow = true
		}
		if utils.InEvictionWindow(ctx, w.StartCron, w.EndCron, nextRun) {
			inNext = true
		}
	}
	if inNow {
		if inNext {
			return StateIn
		}
		return StateAboutToExit
	}
	return StateOut
}

func (t *DisruptionForceTask) hasBlockingAnnotations(pod *corev1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}

	for annotationKey, expectedValue := range utils.GetDoNotDisruptAnnotations() {
		if value, exists := pod.Annotations[annotationKey]; exists && strings.EqualFold(value, expectedValue) {
			return true
		}
	}

	return false
}

func (t *DisruptionForceTask) reconcilePod(ctx context.Context, pod *corev1.Pod, workloadInfo *utils.WorkloadInfo, state ReconcileState) (bool, error) {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	modified := false

	switch state {
	case StateIn:
		if pod.Annotations[utils.AnnotationModified] != utils.TrueValue {
			for key := range utils.GetDoNotDisruptAnnotations() {
				if _, exists := pod.Annotations[key]; exists {
					delete(pod.Annotations, key)
					modified = true
				}
			}
			if modified {
				pod.Annotations[utils.AnnotationModified] = utils.TrueValue
			}
		}
	case StateAboutToExit, StateOut:
		if pod.Annotations[utils.AnnotationModified] == utils.TrueValue {
			workloadSpec, err := utils.GetWorkloadPodSpec(ctx, t.kubeClient, workloadInfo)
			if err != nil {
				logging.Errorf(ctx, "Failed to get workload spec for pod %s: %v", pod.Name, err)
				return false, fmt.Errorf("failed to get workload pod spec: %w", err)
			}

			if workloadSpec != nil && workloadSpec.Annotations != nil {
				for key := range utils.GetDoNotDisruptAnnotations() {
					if val, exists := workloadSpec.Annotations[key]; exists {
						pod.Annotations[key] = val
					}
				}
			}

			delete(pod.Annotations, utils.AnnotationModified)
			modified = true
		}
	}

	if modified {
		_, err := t.kubeClient.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update pod: %w", err)
		}
		logging.Infof(ctx, "Updated pod %s/%s", pod.Namespace, pod.Name)
		if audit.Recorder != nil {
			target := map[string]interface{}{"kind": pod.Kind, "namespace": pod.Namespace, "name": pod.Name}
			if state == StateIn {
				audit.Recorder.Record(ctx, t.config.ClusterID, types.AuditEvent{
					Type:     types.EventTypeNormal,
					Category: types.EventCategoryPODDisruptionBlockRemoved,
					Payload: types.AuditPayload{
						Message: fmt.Sprintf("DND annotations removed for disruption window for pod %s/%s", pod.Namespace, pod.Name),
						Target:  target,
						Details: map[string]interface{}{
							"workloadId": utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name),
							"node":       pod.Spec.NodeName,
						},
					},
				})
			} else {
				audit.Recorder.Record(ctx, t.config.ClusterID, types.AuditEvent{
					Type:     types.EventTypeNormal,
					Category: types.EventCategoryPODDisruptionBlockRestored,
					Payload: types.AuditPayload{
						Message: fmt.Sprintf("DND annotations restored after disruption window for pod %s/%s", pod.Namespace, pod.Name),
						Target:  target,
						Details: map[string]interface{}{
							"workloadId": utils.GetWorkloadKey(workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name),
							"node":       pod.Spec.NodeName,
						},
					},
				})
			}
		}
	}

	return modified, nil
}

func (t *DisruptionForceTask) reconcilePDB(ctx context.Context, pdb *policyv1.PodDisruptionBudget, state ReconcileState) (bool, error) {
	if pdb.Annotations == nil {
		pdb.Annotations = make(map[string]string)
	}

	modified := false

	switch state {
	case StateIn:
		if pdb.Annotations[utils.AnnotationModified] != utils.TrueValue {
			if pdb.Spec.MaxUnavailable != nil {
				pdb.Annotations[utils.AnnotationPDBMaxUnavailable] = pdb.Spec.MaxUnavailable.String()
			}
			if pdb.Spec.MinAvailable != nil {
				pdb.Annotations[utils.AnnotationPDBMinAvailable] = pdb.Spec.MinAvailable.String()
			}

			minAvailable := intstr.FromInt32(0)
			pdb.Spec.MinAvailable = &minAvailable
			pdb.Spec.MaxUnavailable = nil
			pdb.Annotations[utils.AnnotationModified] = utils.TrueValue
			modified = true
		}
	case StateAboutToExit, StateOut:
		if pdb.Annotations[utils.AnnotationModified] == utils.TrueValue {
			if val, exists := pdb.Annotations[utils.AnnotationPDBMaxUnavailable]; exists {
				maxUnavailable := intstr.Parse(val)
				pdb.Spec.MaxUnavailable = &maxUnavailable
				pdb.Spec.MinAvailable = nil
			} else if val, exists := pdb.Annotations[utils.AnnotationPDBMinAvailable]; exists {
				minAvailable := intstr.Parse(val)
				pdb.Spec.MinAvailable = &minAvailable
				pdb.Spec.MaxUnavailable = nil
			}

			delete(pdb.Annotations, utils.AnnotationPDBMaxUnavailable)
			delete(pdb.Annotations, utils.AnnotationPDBMinAvailable)
			delete(pdb.Annotations, utils.AnnotationModified)
			modified = true
		}
	}

	if modified {
		_, err := t.kubeClient.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Update(ctx, pdb, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update PDB: %w", err)
		}
		logging.Infof(ctx, "Updated PDB %s/%s", pdb.Namespace, pdb.Name)
		if audit.Recorder != nil {
			target := map[string]interface{}{"kind": pdb.Kind, "namespace": pdb.Namespace, "name": pdb.Name}
			if state == StateIn {
				audit.Recorder.Record(ctx, t.config.ClusterID, types.AuditEvent{
					Type:     types.EventTypeNormal,
					Category: types.EventCategoryPDBRelaxed,
					Payload: types.AuditPayload{
						Message: fmt.Sprintf("PDB %s/%s relaxed for disruption window", pdb.Namespace, pdb.Name),
						Target:  target,
						Details: map[string]interface{}{},
					},
				})
			} else {
				audit.Recorder.Record(ctx, t.config.ClusterID, types.AuditEvent{
					Type:     types.EventTypeNormal,
					Category: types.EventCategoryPDBRestored,
					Payload: types.AuditPayload{
						Message: fmt.Sprintf("PDB %s/%s restored after disruption window", pdb.Namespace, pdb.Name),
						Target:  target,
						Details: map[string]interface{}{},
					},
				})
			}
		}
	}

	return modified, nil
}
