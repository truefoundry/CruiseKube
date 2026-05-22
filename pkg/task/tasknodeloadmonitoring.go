package task

import (
	"context"
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/truefoundry/cruisekube/pkg/adapters/metricsprovider/prometheus"
	"github.com/truefoundry/cruisekube/pkg/audit"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/task/utils"
	"github.com/truefoundry/cruisekube/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	NodeOverloadTaintKey    = "cruisekube.truefoundry.com/overloaded"
	NodeOverloadTaintValue  = "true"
	NodeOverloadTaintEffect = corev1.TaintEffectNoSchedule
	LoadThreshold           = 1.0
	NodeLoadLookback        = "2m"
)

type NodeLoadMonitoringTaskConfig struct {
	Name      string
	Enabled   bool
	Schedule  string
	ClusterID string
}

type NodeLoadMonitoringTask struct {
	config        *NodeLoadMonitoringTaskConfig
	kubeClient    *kubernetes.Clientset
	dynamicClient dynamic.Interface
	promClient    *prometheus.PrometheusProvider
}

func NewNodeLoadMonitoringTask(ctx context.Context, kubeClient *kubernetes.Clientset, dynamicClient dynamic.Interface, promClient *prometheus.PrometheusProvider, config *NodeLoadMonitoringTaskConfig) *NodeLoadMonitoringTask {
	return &NodeLoadMonitoringTask{
		config:        config,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		promClient:    promClient,
	}
}

func (n *NodeLoadMonitoringTask) GetCoreTask() any {
	return n
}

func (n *NodeLoadMonitoringTask) GetName() string {
	return n.config.Name
}

func (n *NodeLoadMonitoringTask) GetClusterID() string {
	return n.config.ClusterID
}

func (n *NodeLoadMonitoringTask) GetSchedule() string {
	return n.config.Schedule
}

func (n *NodeLoadMonitoringTask) IsEnabled() bool {
	return n.config.Enabled
}

func (n *NodeLoadMonitoringTask) Run(ctx context.Context) error {
	ctx = contextutils.WithTask(ctx, n.config.Name)
	ctx = contextutils.WithCluster(ctx, n.config.ClusterID)

	nodes, err := n.getAllNodes(ctx)
	if err != nil {
		logging.Errorf(ctx, "Error getting nodes: %v", err)
		return err
	}

	logging.Infof(ctx, "Found %d nodes to monitor", len(nodes.Items))

	nodeLoadData, err := n.getNodeLoadMetrics(ctx)
	if err != nil {
		logging.Errorf(ctx, "Error getting node load metrics: %v", err)
		return err
	}

	processedNodes := 0
	taintsAdded := 0
	taintsRemoved := 0

	for _, node := range nodes.Items {
		processedNodes++
		loadAvg, loadDataExists := nodeLoadData[node.Name]

		isOverloaded := loadDataExists && loadAvg > LoadThreshold
		hasOverloadTaint := n.nodeHasOverloadTaint(&node)
		details := map[string]interface{}{}
		if loadDataExists {
			details["loadRatio"] = loadAvg
		}

		if isOverloaded && !hasOverloadTaint {
			if changeDone, err := n.addOverloadTaint(ctx, node.Name); err != nil {
				logging.Errorf(ctx, "Error adding taint to node %s: %v", node.Name, err)
			} else if changeDone {
				taintsAdded++
				logging.Infof(ctx, "Added overload taint to node %s (load: %.2f%%)", node.Name, loadAvg*100)
				if audit.Recorder != nil {
					audit.Recorder.Record(ctx, n.config.ClusterID, types.AuditEvent{
						Type:     types.EventTypeNormal,
						Category: types.EventCategoryNodeOverloadTaintAdded,
						Payload: types.AuditPayload{
							Message: fmt.Sprintf("Overload taint added to node %s", node.Name),
							Target:  map[string]interface{}{"kind": "Node", "name": node.Name},
							Details: details,
						},
					})
				}
			}
		} else if !isOverloaded && hasOverloadTaint {
			if changeDone, err := n.removeOverloadTaint(ctx, node.Name); err != nil {
				logging.Errorf(ctx, "Error removing taint from node %s: %v", node.Name, err)
			} else if changeDone {
				taintsRemoved++
				if loadDataExists {
					logging.Infof(ctx, "Removed overload taint from node %s (load: %.2f%%)", node.Name, loadAvg*100)
				} else {
					logging.Infof(ctx, "Removed overload taint from node %s (no load data)", node.Name)
				}
				if audit.Recorder != nil {
					audit.Recorder.Record(ctx, n.config.ClusterID, types.AuditEvent{
						Type:     types.EventTypeNormal,
						Category: types.EventCategoryNodeOverloadTaintRemoved,
						Payload: types.AuditPayload{
							Message: fmt.Sprintf("Overload taint removed from node %s", node.Name),
							Target:  map[string]interface{}{"kind": "Node", "name": node.Name},
							Details: details,
						},
					})
				}
			}
		}
	}

	return nil
}

func (n *NodeLoadMonitoringTask) getAllNodes(ctx context.Context) (*corev1.NodeList, error) {
	nodes, err := n.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	return nodes, nil
}

func (n *NodeLoadMonitoringTask) getNodeLoadMetrics(ctx context.Context) (map[string]float64, error) {
	query := `
	    min_over_time(
			(
					max by (node) (max by (node) (node_load1{job="node-exporter"}))
				/
					(max by (node) (kube_node_status_capacity{job="kube-state-metrics",resource="cpu"}))
			)[%s:]
		)
	`

	query = fmt.Sprintf(query, NodeLoadLookback)
	logging.Infof(ctx, "Using query: %s", utils.CompressQueryForLogging(query))

	result, warnings, err := n.promClient.ExecuteQueryWithRetry(ctx, n.config.ClusterID, query, "node-load-monitoring")
	if err != nil {
		return nil, fmt.Errorf("error querying prometheus for node load metrics: %w", err)
	}
	if len(warnings) > 0 {
		logging.Infof(ctx, "Warnings from Prometheus query: %v", warnings)
	}

	return n.parseNodeLoadResults(ctx, result)
}

func (n *NodeLoadMonitoringTask) parseNodeLoadResults(ctx context.Context, result model.Value) (map[string]float64, error) {
	nodeLoadData := make(map[string]float64)

	if result.Type() != model.ValVector {
		return nil, fmt.Errorf("expected vector result, got %s", result.Type())
	}

	vector := result.(model.Vector)
	for _, sample := range vector {
		nodeName := string(sample.Metric["node"])
		if nodeName != "" {
			loadValue := float64(sample.Value)
			nodeLoadData[nodeName] = loadValue
			logging.Infof(ctx, "Node %s load: %.2f%%", nodeName, loadValue*100)
		}
	}

	return nodeLoadData, nil
}

func (n *NodeLoadMonitoringTask) nodeHasOverloadTaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == NodeOverloadTaintKey &&
			taint.Value == NodeOverloadTaintValue &&
			taint.Effect == NodeOverloadTaintEffect {
			return true
		}
	}
	return false
}

func (n *NodeLoadMonitoringTask) addOverloadTaint(ctx context.Context, nodeName string) (bool, error) {
	var updated bool
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := n.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get node %s: %w", nodeName, err)
		}
		if n.nodeHasOverloadTaint(node) {
			return nil
		}
		nodeCopy := node.DeepCopy()
		nodeCopy.Spec.Taints = append(nodeCopy.Spec.Taints, corev1.Taint{
			Key:    NodeOverloadTaintKey,
			Value:  NodeOverloadTaintValue,
			Effect: NodeOverloadTaintEffect,
		})
		if _, err := n.kubeClient.CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update node %s: %w", nodeName, err)
		}
		updated = true
		return nil
	}); err != nil {
		return false, fmt.Errorf("add overload taint to node %s: %w", nodeName, err)
	}
	return updated, nil
}

func (n *NodeLoadMonitoringTask) removeOverloadTaint(ctx context.Context, nodeName string) (bool, error) {
	var updated bool
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := n.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get node %s: %w", nodeName, err)
		}
		if !n.nodeHasOverloadTaint(node) {
			return nil
		}
		nodeCopy := node.DeepCopy()
		var newTaints []corev1.Taint
		for _, taint := range nodeCopy.Spec.Taints {
			if taint.Key != NodeOverloadTaintKey ||
				taint.Value != NodeOverloadTaintValue ||
				taint.Effect != NodeOverloadTaintEffect {
				newTaints = append(newTaints, taint)
			}
		}
		nodeCopy.Spec.Taints = newTaints
		if _, err := n.kubeClient.CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update node %s: %w", nodeName, err)
		}
		updated = true
		return nil
	}); err != nil {
		return false, fmt.Errorf("remove overload taint from node %s: %w", nodeName, err)
	}
	return updated, nil
}
