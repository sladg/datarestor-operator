package task_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Init(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[InitTask]").With(
		"task", task.Name,
		"namespace", task.Namespace,
		"job", task.Name+"-job",
	)

	if !task.Status.InitializedAt.IsZero() {
		return false, -1, nil
	}

	logger.Info("Task not initialized yet, initializing...")

	task.Status.State = v1.TaskStatePending
	task.Status.InitializedAt = metav1.Now()

	return true, constants.ImmediateRequeueInterval, nil
}
