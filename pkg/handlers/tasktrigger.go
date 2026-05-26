package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type TaskTriggerResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}

func (deps HandlerDependencies) HandleTaskTrigger(c *gin.Context) {
	ctx := c.Request.Context()
	clusterID := c.Param("clusterID")
	taskName := c.Param("taskName")

	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("cluster.id", clusterID),
		attribute.String("task.name", taskName),
	)

	logging.Infof(ctx, "Manual task trigger for task '%s' in cluster '%s' by %s", taskName, clusterID, c.ClientIP())

	task, err := deps.ClusterManager.GetTask(taskName)
	if err != nil {
		logging.Errorf(ctx, "Task '%s' not found: %v", taskName, err)
		c.JSON(http.StatusNotFound, TaskTriggerResponse{
			Status: metrics.StatusError,
			Error:  fmt.Sprintf("Task '%s' not found", taskName),
		})
		return
	}
	taskClusterID := task.GetClusterID()
	if taskClusterID != clusterID {
		logging.Errorf(ctx, "Task '%s' belongs to cluster '%s', not requested cluster '%s'", taskName, taskClusterID, clusterID)
		c.JSON(http.StatusNotFound, TaskTriggerResponse{
			Status: metrics.StatusError,
			Error:  fmt.Sprintf("Task '%s' not found in cluster '%s'", taskName, clusterID),
		})
		return
	}

	logging.Infof(ctx, "Manual trigger - executing task '%s'", taskName)

	startedAt := time.Now()
	defer func() {
		if r := recover(); r != nil {
			metrics.RecordTaskRun(taskClusterID, taskName, metrics.StatusPanic, metrics.TaskSourceManual, time.Since(startedAt))
			panic(r)
		}
	}()

	logging.Infof(ctx, "Starting synchronous execution of task '%s'", taskName)
	if err := task.Run(ctx); err != nil {
		duration := time.Since(startedAt)
		logging.Errorf(ctx, "Task '%s' failed after %v: %v", taskName, duration, err)
		metrics.RecordTaskRun(taskClusterID, taskName, metrics.StatusError, metrics.TaskSourceManual, duration)
		c.JSON(http.StatusInternalServerError, TaskTriggerResponse{
			Status:   metrics.StatusError,
			Error:    err.Error(),
			Duration: duration.String(),
		})
		return
	}

	duration := time.Since(startedAt)
	logging.Infof(ctx, "Task '%s' completed successfully in %v", taskName, duration)
	metrics.RecordTaskRun(taskClusterID, taskName, metrics.StatusSuccess, metrics.TaskSourceManual, duration)
	c.JSON(http.StatusOK, TaskTriggerResponse{
		Status:   metrics.StatusSuccess,
		Message:  fmt.Sprintf("Task '%s' completed successfully", taskName),
		Duration: duration.String(),
	})
}
