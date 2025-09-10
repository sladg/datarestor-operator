package task_util

import (
	"context"
	"encoding/json"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func Start(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[StartTask]").With("task", task.Name, "namespace", task.Namespace)

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		Spec: batchv1.JobSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      task.Name + "-job",
			Namespace: task.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: task.APIVersion,
					Kind:       task.Kind,
					Name:       task.Name,
					UID:        task.UID,
					Controller: ptr.To(true),
				},
			},
		},
	}

	if err := json.Unmarshal(task.Spec.JobTemplate.Raw, &job.Spec); err != nil {
		logger.Errorw("Failed to unmarshal jobTemplate", err)

		task.Status.State = v1.TaskStateFailed
		return true, 0, err
	}

	// Debug: Log the job spec to see what's missing
	logger.Info("Creating job")

	if err := deps.Create(ctx, &job); err != nil {
		logger.Errorw("Failed to create job", err)

		task.Status.State = v1.TaskStateFailed
		return true, constants.ImmediateRequeueInterval, err
	}

	task.Status.State = v1.TaskStateRunning
	task.Status.JobRef = corev1.ObjectReference{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       job.Name,
		Namespace:  job.Namespace,
		UID:        job.UID,
	}

	logger.Info("Job created")

	// Will be requeued by the job changes
	return true, 0, nil
}
