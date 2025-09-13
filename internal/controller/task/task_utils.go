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
		"managed-by":                       v1.OperatorDomain,
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

func CheckTaskScaleDown(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[CheckTaskScaleDown]").With(
		"task", task.Name,
		"namespace", task.Namespace,
		"pvc", task.Spec.PVCRef.Name,
	)

	// Only process if in ScalingDown state
	if task.Status.State != v1.TaskStateScalingDown {
		return false, -1, nil
	}

	// If not stopping pods, proceed to start immediately
	if !task.Spec.StopPods {
		task.Status.State = v1.TaskStateStarting
		return true, constants.ImmediateRequeueInterval, nil
	}

	// We're in ScalingDown state - check if we need to start scaling
	if task.Status.ScaledDownAt.IsZero() {
		scaling, err := utils.ScaleDownWorkloadsForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to scale down workloads, continuing ...", err)
			task.Status.State = v1.TaskStateFailed
			return true, constants.ImmediateRequeueInterval, err
		}
		if !scaling {
			logger.Info("No workloads found for PVC, proceeding ...")
			task.Status.State = v1.TaskStateStarting
			return true, constants.ImmediateRequeueInterval, nil
		}

		task.Status.ScaledDownAt = metav1.Now()
		return true, constants.QuickRequeueInterval, nil
	}

	// Check if workloads are scaled down already
	done, err := utils.IsScaleDownCompleteForPVC(ctx, deps, task.Spec.PVCRef)
	if err != nil {
		logger.Warnw("Failed to check if workloads are scaled down, continuing ...", err)
		task.Status.State = v1.TaskStateFailed
		return true, constants.ImmediateRequeueInterval, err
	}
	if !done {
		logger.Info("Workloads are not scaled down yet, requeuing ...")
		return true, constants.QuickRequeueInterval, nil
	}

	logger.Info("Workloads are scaled down, starting task ...")

	task.Status.State = v1.TaskStateStarting
	return true, constants.ImmediateRequeueInterval, nil
}

func CheckTaskScaleUp(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[CheckTaskScaleUp]").With(
		"task", task.Name,
		"namespace", task.Namespace,
		"pvc", task.Spec.PVCRef.Name,
	)

	// Only process if in ScalingUp state
	if task.Status.State != v1.TaskStateScalingUp {
		return false, -1, nil
	}

	// If pods were not stopped, there is nothing to scale up
	if !task.Spec.StopPods {
		task.Status.State = v1.TaskStateCompleted
		return true, constants.ImmediateRequeueInterval, nil
	}

	// Initialize the scaling up of workloads
	if task.Status.ScaledUpAt.IsZero() {
		scaling, err := utils.ScaleUpWorkloadsForPVC(ctx, deps, task.Spec.PVCRef, task)
		if err != nil {
			logger.Warnw("Failed to scale up workloads, continuing ...", err)
			task.Status.State = v1.TaskStateFailed
			return true, constants.ImmediateRequeueInterval, err
		}
		if !scaling {
			logger.Info("No workloads found for PVC, proceeding ...")
			task.Status.State = v1.TaskStateCompleted
			return true, constants.ImmediateRequeueInterval, nil
		}

		task.Status.ScaledUpAt = metav1.Now()
		return true, constants.ImmediateRequeueInterval, nil
	}

	// Check if workloads are scaled up already
	done, err := utils.IsScaleUpCompleteForPVC(ctx, deps, task.Spec.PVCRef, task)
	if err != nil {
		logger.Warnw("Failed to check if workloads are scaled up, continuing ...", err)
		task.Status.State = v1.TaskStateFailed
		return true, constants.ImmediateRequeueInterval, err
	}
	if !done {
		logger.Info("Workloads are not scaled up yet, requeuing ...")
		return true, constants.QuickRequeueInterval, nil
	}

	logger.Info("Workloads are scaled up, task completed ...")

	task.Status.State = v1.TaskStateCompleted
	return true, constants.ImmediateRequeueInterval, nil
}
