package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/repository/storage"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
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
	appConfig  *config.Config
	cronParser cron.Parser
}

func NewDisruptionForceTask(_ context.Context, kubeClient *kubernetes.Clientset, storage *storage.Storage, taskConfig *DisruptionForceTaskConfig, appConfig *config.Config) *DisruptionForceTask {
	return &DisruptionForceTask{
		kubeClient: kubeClient,
		storage:    storage,
		config:     taskConfig,
		appConfig:  appConfig,
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
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
	state := t.getReconcileState(ctx, now)
	logging.Infof(ctx, "Reconcile state: %v", state)

	stats, err := t.storage.GetAllStatsForCluster(t.config.ClusterID)
	if err != nil {
		logging.Errorf(ctx, "Failed to get workload stats: %v", err)
		return fmt.Errorf("failed to get workload stats: %w", err)
	}

	workloadsWithBlockingAnnotations := make([]utils.WorkloadInfo, 0)
	for _, stat := range stats {
		if stat.Constraints != nil && stat.Constraints.DoNotDisruptAnnotation {
			workloadsWithBlockingAnnotations = append(workloadsWithBlockingAnnotations, utils.WorkloadInfo{
				Kind:      stat.Kind,
				Namespace: stat.Namespace,
				Name:      stat.Name,
			})
		}
	}

	logging.Infof(ctx, "Total workloads with blocking annotations: %d", len(workloadsWithBlockingAnnotations))

	reconciledPods := 0
	for _, workloadInfo := range workloadsWithBlockingAnnotations {
		workloadObj, err := utils.GetWorkloadObject(ctx, t.kubeClient, workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name)
		if err != nil {
			logging.Errorf(ctx, "Failed to get workload %s/%s/%s: %v", workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name, err)
			continue
		}

		selector, err := workloadObj.GetSelector()
		if err != nil {
			logging.Errorf(ctx, "Failed to get selector for workload %s/%s/%s: %v", workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name, err)
			continue
		}

		pods, err := utils.GetPods(ctx, t.kubeClient, workloadInfo.Namespace, selector)
		if err != nil {
			logging.Errorf(ctx, "Failed to list pods for workload %s/%s/%s: %v", workloadInfo.Kind, workloadInfo.Namespace, workloadInfo.Name, err)
			continue
		}

		for i := range pods.Items {
			pod := &pods.Items[i]

			hasModifiedMarker := pod.Annotations != nil && pod.Annotations[utils.AnnotationModified] == utils.TrueValue
			if !t.hasBlockingAnnotations(pod) && !hasModifiedMarker {
				continue
			}

			modified, err := t.reconcilePod(ctx, pod, &workloadInfo, state)
			if err != nil {
				logging.Errorf(ctx, "Failed to reconcile pod %s/%s: %v", pod.Namespace, pod.Name, err)
				continue
			}
			if modified {
				reconciledPods++
			}
		}
	}

	pdbs, err := t.kubeClient.PolicyV1().PodDisruptionBudgets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Failed to list PDBs: %v", err)
		return fmt.Errorf("failed to list PDBs: %w", err)
	}

	reconciledPDBs := 0
	for i := range pdbs.Items {
		pdb := &pdbs.Items[i]
		modified, err := t.reconcilePDB(ctx, pdb, state)
		if err != nil {
			logging.Errorf(ctx, "Failed to reconcile PDB %s/%s: %v", pdb.Namespace, pdb.Name, err)
			continue
		}
		if modified {
			reconciledPDBs++
		}
	}

	logging.Infof(ctx, "Disruption force task completed: reconciled %d pods, %d PDBs modified", reconciledPods, reconciledPDBs)
	return nil
}

func (t *DisruptionForceTask) getReconcileState(ctx context.Context, now time.Time) ReconcileState {
	duration, err := time.ParseDuration(t.config.Schedule)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse duration %q: %v", t.config.Schedule, err)
		return StateOut
	}

	nextRun := now.Add(duration)
	inNow := t.inEvictionWindow(ctx, now)
	inNext := t.inEvictionWindow(ctx, nextRun)

	if inNow {
		if inNext {
			return StateIn
		}
		return StateAboutToExit
	}
	return StateOut
}

func (t *DisruptionForceTask) inEvictionWindow(ctx context.Context, tm time.Time) bool {
	startCronExpr := t.appConfig.DisruptionSettings.WindowStartCron
	endCronExpr := t.appConfig.DisruptionSettings.WindowEndCron

	if startCronExpr == "" || endCronExpr == "" {
		logging.Warnf(ctx, "Disruption window start or end cron is empty, skipping disruption force task")
		return false
	}

	startSchedule, err := t.cronParser.Parse(startCronExpr)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse disruption window start cron %q: %v", startCronExpr, err)
		return false
	}

	endSchedule, err := t.cronParser.Parse(endCronExpr)
	if err != nil {
		logging.Errorf(ctx, "Failed to parse disruption window end cron %q: %v", endCronExpr, err)
		return false
	}

	nextStart := startSchedule.Next(tm)
	nextEnd := endSchedule.Next(tm)

	return nextEnd.Before(nextStart) || nextEnd.Equal(nextStart)
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
	}

	return modified, nil
}
