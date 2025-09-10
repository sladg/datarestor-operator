package task_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
)

func UpdateTaskStatus(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[UpdateTaskStatus]").With("task", task.Name, "namespace", task.Namespace)

	job, err := GetJob(ctx, deps, task, true)
	if err != nil {
		logger.Errorw("Failed to get job", err)
		return false, -1, err
	}

	if job == nil {
		logger.Info("Job not found, nuking itself")
		return true, constants.ImmediateRequeueInterval, deps.Delete(ctx, task)
	}

	task.Status.JobStatus = job.Status

	// Derive high-level state from JobStatus
	if job.Status.Succeeded > 0 {
		task.Status.State = v1.TaskStateCompleted
	} else if job.Status.Failed > 0 {
		task.Status.State = v1.TaskStateFailed
	} else if job.Status.Active > 0 {
		task.Status.State = v1.TaskStateRunning
	} else {
		task.Status.State = v1.TaskStatePending
	}

	return true, constants.DefaultRequeueInterval, nil
}
