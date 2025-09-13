package task_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
)

func UpdateTaskStatus(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[UpdateTaskStatus]").With("task", task.Name, "namespace", task.Namespace)

	// Only process if in Running state
	if task.Status.State != v1.TaskStateRunning {
		return false, -1, nil
	}

	job, err := GetJobFromTask(ctx, deps, task, true)
	if err != nil {
		// We shouldn't be here unless job has started and was found
		logger.Errorw("Failed to get job", err)
		// @TODO: Improve this. We should ignore without err?
		return false, -1, err
	}

	if job == nil {
		logger.Warn("Job not found, assuming fail ...")
		task.Status.State = v1.TaskStateFailed
		return true, constants.ImmediateRequeueInterval, nil
	}

	task.Status.JobStatus = job.Status
	JobToTaskStatus(ctx, deps, task, job)

	return true, constants.DefaultRequeueInterval, nil
}
