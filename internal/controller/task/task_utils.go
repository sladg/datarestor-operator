package task_util

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// RemoveTasksByLabels deletes all Task resources matching the given labels.
func RemoveTasksByConfig(ctx context.Context, deps *utils.Dependencies, config *v1.Config) error {
	labels := map[string]string{
		constants.LabelTaskParentName:      config.Name,
		constants.LabelTaskParentNamespace: config.Namespace,
	}

	tasks := &v1.TaskList{}
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
func GetTasksStatus(tasks *v1.TaskList) v1.ConfigStatistics {
	stats := v1.ConfigStatistics{}

	for _, task := range tasks.Items {
		// Only count backup tasks for statistics
		if task.Spec.Type != v1.TaskTypeBackupScheduled && task.Spec.Type != v1.TaskTypeBackupManual {
			continue
		}

		jobStatus := task.Status.State

		switch jobStatus {
		case v1.TaskStateCompleted:
			stats.SuccessfulBackups++
		case v1.TaskStateFailed:
			stats.FailedBackups++
		case v1.TaskStateRunning:
			stats.RunningBackups++
		}

		// Note: Jobs with 0 Active, 0 Succeeded, 0 Failed are considered pending/not started
	}

	return stats
}

func CheckTaskScaleDown(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, error) {
	logger := deps.Logger.Named("task-utils/scale-down").With("task", task.Name, "namespace", task.Namespace)

	shouldScaleDown := task.Spec.StopPods && task.Status.InitializedAt.IsZero()

	if shouldScaleDown && task.Status.State != v1.TaskStateScalingDown {
		scaling, err := utils.ScaleDownWorkloadsForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to scale down workloads", err, "pvc", task.Spec.PVCRef.Name)
			return true, err
		}
		if !scaling {
			return false, nil
		}

		controllerutil.AddFinalizer(task, constants.TaskFinalizer)
		task.Status.State = v1.TaskStateScalingDown
		return true, nil
	}

	if task.Status.State == v1.TaskStateScalingDown {
		done, err := utils.IsScaleDownCompleteForPVC(ctx, deps, task.Spec.PVCRef)
		if err != nil {
			logger.Warnw("Failed to check if workloads are scaled down", err, "pvc", task.Spec.PVCRef.Name)
			return true, err
		}
		if !done {
			logger.Infow("Workloads are not scaled down yet", "pvc", task.Spec.PVCRef.Name)
			return true, nil
		}

		task.Status.State = v1.TaskStatePending
		return true, nil
	}

	return false, nil
}

func CheckTaskScaleUp(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, error) {
	logger := deps.Logger.Named("task-utils/scale-up").With("task", task.Name, "namespace", task.Namespace)

	shouldScaleUp := task.Spec.StopPods && utils.Contains([]v1.TaskState{v1.TaskStateFailed, v1.TaskStateCompleted}, task.Status.State)

	// Scale up workloads if job is completed
	if shouldScaleUp && task.Status.State != v1.TaskStateScalingUp {
		scaling, err := utils.ScaleUpWorkloadsForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to scale up workloads", err, "pvc", task.Spec.PVCRef.Name)
			return true, err
		}
		if !scaling {
			return false, nil
		}

		task.Status.State = v1.TaskStateScalingUp

		return true, nil
	}

	// Remove finalizer if job is completed and workloads are scaled up
	if task.Status.State == v1.TaskStateScalingUp {
		done, err := utils.IsScaleUpCompleteForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to check if workloads are scaled down", err, "pvc", task.Spec.PVCRef.Name)
			return true, err
		}
		if !done {
			logger.Infow("Workloads are not scaled down yet", "pvc", task.Spec.PVCRef.Name)
			return true, nil
		}

		controllerutil.RemoveFinalizer(task, constants.TaskFinalizer)
		return true, nil
	}

	return false, nil
}
