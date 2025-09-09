package task_util

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UniqueNameParams struct {
	PVC      *corev1.PersistentVolumeClaim
	TaskType v1.TaskType
	Config   v1.Config
}

// Format: {type}-{shortUUID} - Keep it simple and clean
// The short UUID ensures uniqueness even in high-frequency scenarios
func GenerateUniqueName(params UniqueNameParams) (string, string) {
	shortUUID := uuid.NewUUID()[:6] // Use first 6 characters for cleaner names

	businessName := string(params.TaskType)
	uniqueName := fmt.Sprintf("%s-%s", businessName, shortUUID)

	return uniqueName, businessName
}

func GetJobName(task *v1.Task) string {
	return fmt.Sprintf("%s-job", task.Name)
}

func GetJob(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	err := deps.Get(ctx, client.ObjectKey{Namespace: task.Status.JobRef.Namespace, Name: task.Status.JobRef.Name}, job)
	if err != nil {
		return nil, err
	}
	return job, nil
}
