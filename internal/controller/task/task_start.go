package task_util

import (
	"context"
	"encoding/json"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Start(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[StartTask]").With("task", task.Name, "namespace", task.Namespace)

	// Only start when transitioning from scaling-down logic
	if task.Status.State != v1.TaskStateStarting {
		return false, -1, nil
	}

	// If job already exists, adopt it
	if exists, err := CheckJobExistsForTask(ctx, deps, task); err == nil && exists {
		logger.Infow("Job already exists, adopting", "job", task.Status.JobRef.Name)
		task.Status.State = v1.TaskStateRunning
		return true, constants.ImmediateRequeueInterval, nil
	}

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		Spec: batchv1.JobSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      task.Name + "-job",
			Namespace: task.Namespace,
			// Do not set managed-by label, it has to be empty in order for k8s to manage it automatically
			OwnerReferences: []metav1.OwnerReference{
				// While job is active, task should not be deleted
				utils.TaskToOwnerReference(task),
				// Job owns PVC during active operations to prevent PVC deletion
				// This prevents PVC deletion while backup/restore is running
				utils.PVCToOwnerReference(task.Spec.PVCRef),
			},
		},
	}

	if err := json.Unmarshal(task.Spec.JobTemplate.Raw, &job.Spec); err != nil {
		logger.Errorw("Failed to unmarshal jobTemplate", err)

		task.Status.State = v1.TaskStateFailed
		return true, constants.ImmediateRequeueInterval, err
	}

	// Debug: Log the job spec to see what's missing
	logger.Info("Creating job")

	if err := deps.Create(ctx, &job); err != nil {
		if client.IgnoreAlreadyExists(err) == nil {
			logger.Infow("Job already exists, adopting", "job", job.Name)
			task.Status.State = v1.TaskStateRunning

			return true, constants.ImmediateRequeueInterval, nil
		}

		task.Status.State = v1.TaskStateFailed
		logger.Errorw("Failed to create job", err)
		return true, constants.ImmediateRequeueInterval, err
	}

	task.Status.State = v1.TaskStateRunning
	task.Status.JobRef = utils.JobToRef(&job)

	logger.Info("Job created")

	// Will be requeued by the job changes
	return true, constants.ImmediateRequeueInterval, nil
}
