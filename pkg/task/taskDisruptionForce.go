package task

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	AnnotationPDBMaxUnavailable = "cruisekube.truefoundry.com/pdb/maxUnavailable"
	AnnotationPDBMinAvailable   = "cruisekube.truefoundry.com/pdb/minAvailable"
	AnnotationModified          = "cruisekube.truefoundry.com/modified"
)

var DoNotDisruptAnnotations = []string{
	"cluster-autoscaler.kubernetes.io/safe-to-evict",
	"karpenter.sh/do-not-evict",
	"karpenter.sh/do-not-disrupt",
}

type ReconcileState int

const (
	StateFullyIn ReconcileState = iota + 1
	StateLastIn                 // about to be out in the next run
	StateOut
)

type DisruptionForceTaskConfig struct {
	Name            string
	Enabled         bool
	Schedule        string
	ClusterID       string
	TargetNamespace string
}

type DisruptionForceTask struct {
	kubeClient *kubernetes.Clientset
	config     *DisruptionForceTaskConfig
	appConfig  *config.Config
	cronParser cron.Parser
}

func NewDisruptionForceTask(_ context.Context, kubeClient *kubernetes.Clientset, taskConfig *DisruptionForceTaskConfig, appConfig *config.Config) *DisruptionForceTask {
	return &DisruptionForceTask{
		kubeClient: kubeClient,
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

	logging.Infof(ctx, "Running disruption force task")

	now := time.Now()
	state := t.getReconcileState(now)
	logging.Infof(ctx, "Reconcile state: %v", state)

	namespace := t.config.TargetNamespace
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	pods, err := t.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Failed to list pods: %v", err)
		return fmt.Errorf("failed to list pods: %w", err)
	}

	reconciledPods := 0
	for i := range pods.Items {
		pod := &pods.Items[i]
		workloadInfo := utils.GetWorkloadInfoFromPod(pod)
		if workloadInfo == nil {
			continue
		}
		if err := t.reconcilePod(ctx, pod, workloadInfo, state); err != nil {
			logging.Errorf(ctx, "Failed to reconcile pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}
		reconciledPods++
	}

	pdbs, err := t.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Errorf(ctx, "Failed to list PDBs: %v", err)
		return fmt.Errorf("failed to list PDBs: %w", err)
	}

	reconciledPDBs := 0
	for i := range pdbs.Items {
		pdb := &pdbs.Items[i]
		if err := t.reconcilePDB(ctx, pdb, state); err != nil {
			logging.Errorf(ctx, "Failed to reconcile PDB %s/%s: %v", pdb.Namespace, pdb.Name, err)
			continue
		}
		reconciledPDBs++
	}

	logging.Infof(ctx, "Disruption force task completed: reconciled %d pods, %d PDBs", reconciledPods, reconciledPDBs)
	return nil
}

func (t *DisruptionForceTask) getReconcileState(now time.Time) ReconcileState {
	schedule, err := t.cronParser.Parse(t.config.Schedule)
	if err != nil {
		logging.Errorf(context.Background(), "Failed to parse schedule: %v", err)
		return StateOut
	}

	nextRun := schedule.Next(now)
	inNow := t.inEvictionWindow(now)
	inNext := t.inEvictionWindow(nextRun)

	if inNow {
		if inNext {
			return StateFullyIn
		}
		return StateLastIn
	}
	return StateOut
}

func (t *DisruptionForceTask) inEvictionWindow(tm time.Time) bool {
	cronExpr := t.appConfig.DisruptionSettings.WindowCron
	durationMinutes := t.appConfig.DisruptionSettings.WindowDurationMinutes

	if cronExpr == "" || durationMinutes <= 0 {
		return false
	}

	schedule, err := t.cronParser.Parse(cronExpr)
	if err != nil {
		return false
	}

	prevTime := tm.Add(-time.Duration(durationMinutes) * time.Minute)
	nextCron := schedule.Next(prevTime)
	return nextCron.Before(tm)
}

func (t *DisruptionForceTask) reconcilePod(ctx context.Context, pod *corev1.Pod, workloadInfo *utils.WorkloadInfo, state ReconcileState) error {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	modified := false

	if state == StateFullyIn {
		if pod.Annotations[AnnotationModified] != "true" {
			for _, key := range DoNotDisruptAnnotations {
				if _, exists := pod.Annotations[key]; exists {
					delete(pod.Annotations, key)
					modified = true
				}
			}
			if modified {
				pod.Annotations[AnnotationModified] = "true"
			}
		}
	} else if state == StateLastIn || state == StateOut {
		if pod.Annotations[AnnotationModified] == "true" {
			workloadSpec, err := utils.GetWorkloadPodSpec(ctx, t.kubeClient, workloadInfo)
			if err != nil {
				logging.Errorf(ctx, "Failed to get workload spec for pod %s: %v", pod.Name, err)
				return err
			}

			if workloadSpec != nil && workloadSpec.Annotations != nil {
				for _, key := range DoNotDisruptAnnotations {
					if val, exists := workloadSpec.Annotations[key]; exists {
						pod.Annotations[key] = val
						modified = true
					}
				}
			}

			delete(pod.Annotations, AnnotationModified)
			modified = true
		}
	}

	if modified {
		_, err := t.kubeClient.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update pod: %w", err)
		}
		logging.Infof(ctx, "Updated pod %s/%s", pod.Namespace, pod.Name)
	}

	return nil
}

func (t *DisruptionForceTask) reconcilePDB(ctx context.Context, pdb *policyv1.PodDisruptionBudget, state ReconcileState) error {
	if pdb.Annotations == nil {
		pdb.Annotations = make(map[string]string)
	}

	modified := false

	if state == StateFullyIn {
		if pdb.Annotations[AnnotationModified] != "true" {
			if pdb.Spec.MaxUnavailable != nil {
				pdb.Annotations[AnnotationPDBMaxUnavailable] = pdb.Spec.MaxUnavailable.String()
			}
			if pdb.Spec.MinAvailable != nil {
				pdb.Annotations[AnnotationPDBMinAvailable] = pdb.Spec.MinAvailable.String()
			}

			maxUnavailable := intstr.FromString("100%")
			minAvailable := intstr.FromInt32(0)
			pdb.Spec.MaxUnavailable = &maxUnavailable
			pdb.Spec.MinAvailable = &minAvailable
			pdb.Annotations[AnnotationModified] = "true"
			modified = true
		}
	} else if state == StateLastIn || state == StateOut {
		if pdb.Annotations[AnnotationModified] == "true" {
			if val, exists := pdb.Annotations[AnnotationPDBMaxUnavailable]; exists {
				maxUnavailable := intstr.Parse(val)
				pdb.Spec.MaxUnavailable = &maxUnavailable
			}
			if val, exists := pdb.Annotations[AnnotationPDBMinAvailable]; exists {
				minAvailable := intstr.Parse(val)
				pdb.Spec.MinAvailable = &minAvailable
			}

			delete(pdb.Annotations, AnnotationPDBMaxUnavailable)
			delete(pdb.Annotations, AnnotationPDBMinAvailable)
			delete(pdb.Annotations, AnnotationModified)
			modified = true
		}
	}

	if modified {
		_, err := t.kubeClient.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Update(ctx, pdb, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update PDB: %w", err)
		}
		logging.Infof(ctx, "Updated PDB %s/%s", pdb.Namespace, pdb.Name)
	}

	return nil
}
