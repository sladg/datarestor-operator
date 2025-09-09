package task_util

import (
	"context"

	"github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RemoveTasksByLabels deletes all Task resources matching the given labels.
func RemoveTasksByConfig(ctx context.Context, deps *utils.Dependencies, config *v1alpha1.Config) error {
	labels := map[string]string{
		constants.LabelTaskParentName:      config.Name,
		constants.LabelTaskParentNamespace: config.Namespace,
	}

	tasks := &v1alpha1.TaskList{}
	err := deps.List(ctx, tasks, client.MatchingLabels(labels))
	if err != nil {
		return err
	}
	for _, task := range tasks.Items {
		if err := deps.Delete(ctx, &task); err != nil {
			return err
		}
	}

	return nil
}

// GetTasksStatus aggregates statistics from a list of tasks
func GetTasksStatus(tasks *v1alpha1.TaskList) v1alpha1.ConfigStatistics {
	stats := v1alpha1.ConfigStatistics{}

	for _, task := range tasks.Items {
		// Only count backup tasks for statistics
		if task.Spec.Type != v1alpha1.TaskTypeBackupScheduled && task.Spec.Type != v1alpha1.TaskTypeBackupManual {
			continue
		}

		jobStatus := task.Status.JobStatus

		// Check job status based on Kubernetes JobStatus fields
		if jobStatus.Succeeded > 0 {
			stats.SuccessfulBackups++
		} else if jobStatus.Failed > 0 {
			stats.FailedBackups++
		} else if jobStatus.Active > 0 {
			stats.RunningBackups++
		}
		// Note: Jobs with 0 Active, 0 Succeeded, 0 Failed are considered pending/not started
	}

	return stats
}
