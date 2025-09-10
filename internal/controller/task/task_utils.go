package task_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func CheckTaskScaleDown(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[CheckTaskScaleDown]").With(
		"task", task.Name,
		"namespace", task.Namespace,
		"pvc", task.Spec.PVCRef.Name,
	)

	if task.Status.State != v1.TaskStatePending {
		return false, -1, nil
	}

	if task.Spec.StopPods {
		return false, -1, nil
	}

	if task.Status.ScaledDownAt.IsZero() {
		scaling, err := utils.ScaleDownWorkloadsForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to scale down workloads, continuing ...", err)
			return false, -1, err
		}
		if !scaling {
			logger.Info("Workloads are not scaled down yet, requeuing ...")
			return false, -1, nil
		}

		task.Status.ScaledDownAt = metav1.Now()
		return true, constants.QuickRequeueInterval, nil
	}

	// Check if workloads are scaled down already
	done, err := utils.IsScaleDownCompleteForPVC(ctx, deps, task.Spec.PVCRef)
	if err != nil {
		logger.Warnw("Failed to check if workloads are scaled down, continuing ...", err)
		return false, -1, err
	}
	if !done {
		logger.Info("Workloads are not scaled down yet, requeuing ...")
		return true, constants.QuickRequeueInterval, nil
	}

	logger.Info("Workloads are scaled down, starting task ...")

	task.Status.State = v1.TaskStateStarting
	return true, constants.ImmediateRequeueInterval, nil
}

func CheckTaskScaleUp(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[CheckTaskScaleUp]").With(
		"task", task.Name,
		"namespace", task.Namespace,
		"pvc", task.Spec.PVCRef.Name,
	)

	// Only scale up if task is completed or failed
	if task.Status.State != v1.TaskStateCompleted && task.Status.State != v1.TaskStateFailed {
		return false, -1, nil
	}

	if task.Spec.StopPods {
		return false, -1, nil
	}

	// Initialize the scaling up of workloads
	if task.Status.ScaledUpAt.IsZero() {
		scaling, err := utils.ScaleUpWorkloadsForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to scale up workloads, continuing ...", err)
			return false, -1, err
		}
		if !scaling {
			logger.Info("Workloads are not scaled up yet, requeuing ...")
			return false, -1, nil
		}

		task.Status.ScaledUpAt = metav1.Now()
		return true, constants.ImmediateRequeueInterval, nil
	}

	// Check if workloads are scaled up already
	done, err := utils.IsScaleUpCompleteForPVC(ctx, deps, task.Spec.PVCRef, task)
	if err != nil {
		logger.Warnw("Failed to check if workloads are scaled up, continuing ...", err)
		return false, -1, err
	}
	if !done {
		logger.Info("Workloads are not scaled up yet, requeuing ...")
		return true, constants.QuickRequeueInterval, nil
	}

	logger.Info("Workloads are scaled up, task completed ...")

	return true, constants.ImmediateRequeueInterval, nil
}
