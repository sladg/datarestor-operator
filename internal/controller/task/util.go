package task_util

import (
	"context"
	"fmt"
	"io"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UniqueNameParams struct {
	PVC      *corev1.PersistentVolumeClaim
	TaskType v1.TaskType
	Config   v1.Config
}

// Format: {type}-YYYY-MM-DD-HH-mm-{shortUUID}
// The timestamp helps sorting by age; the short UUID ensures uniqueness
func GenerateUniqueName(params UniqueNameParams) (string, string) {
	shortUUID := uuid.NewUUID()[:6] // Use first 6 characters for cleaner names
	businessName := string(params.TaskType)
	timestamp := time.Now().Format("2006-01-02-15-04")
	uniqueName := fmt.Sprintf("%s-%s-%s", businessName, timestamp, shortUUID)

	return uniqueName, businessName
}

func GetJobName(task *v1.Task) string {
	return fmt.Sprintf("%s-job", task.Name)
}

func GetJobFromTask(ctx context.Context, deps *utils.Dependencies, task *v1.Task, ignoreNotFound bool) (*batchv1.Job, error) {
	name := task.Status.JobRef.Name
	ns := task.Status.JobRef.Namespace
	if name == "" {
		name = GetJobName(task)
	}
	if ns == "" {
		ns = task.Namespace
	}

	job := &batchv1.Job{}
	err := deps.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, job)
	if err != nil {
		if ignoreNotFound && client.IgnoreNotFound(err) == nil {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func JobToTaskStatus(ctx context.Context, deps *utils.Dependencies, task *v1.Task, job *batchv1.Job) {
	if job.Status.Succeeded > 0 {
		task.Status.State = v1.TaskStateScalingUp

		// @TODO: Maybe unnecessary?
		// Capture logs when job succeeds
		if task.Status.Output == "" {
			if logs, err := GetJobLogs(ctx, deps, job); err != nil {
				deps.Logger.Warnw("Failed to get job logs", "error", err, "job", job.Name)
			} else {
				task.Status.Output = logs
			}
		}
	} else if job.Status.Failed > 0 {
		task.Status.State = v1.TaskStateFailed
		// Capture logs when job fails
		if task.Status.Output == "" {
			if logs, err := GetJobLogs(ctx, deps, job); err != nil {
				deps.Logger.Warnw("Failed to get job logs", "error", err, "job", job.Name)
			} else {
				task.Status.Output = logs
			}
		}
	} else if job.Status.Active > 0 {
		task.Status.State = v1.TaskStateRunning
	}
}

func GetMaybeExistingJob(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (exists bool, jobRef corev1.ObjectReference, job *batchv1.Job, err error) {
	job, err = GetJobFromTask(ctx, deps, task, true)
	if err != nil {
		return false, corev1.ObjectReference{}, nil, err
	}
	if job == nil {
		return false, corev1.ObjectReference{}, nil, nil
	}

	jobRef = utils.JobToRef(job)

	return true, jobRef, job, nil
}

func CheckJobExistsForTask(ctx context.Context, deps *utils.Dependencies, task *v1.Task) (exists bool, err error) {
	job, err := GetJobFromTask(ctx, deps, task, true)
	if err != nil {
		return false, err
	}
	return job != nil, nil
}

// GetJobLogs retrieves logs from the Job's pods
func GetJobLogs(ctx context.Context, deps *utils.Dependencies, job *batchv1.Job) (string, error) {
	// Create a clientset from the REST config
	clientset, err := kubernetes.NewForConfig(deps.Config)
	if err != nil {
		return "", fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Get pods belonging to this job
	podList, err := clientset.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"job-name": job.Name,
			},
		}),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job %s: %w", job.Name, err)
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", job.Name)
	}

	var allLogs []string
	for _, pod := range podList.Items {
		// Get logs for each container in the pod
		for _, container := range pod.Spec.Containers {
			logs, err := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: container.Name,
			}).Stream(ctx)
			if err != nil {
				// Log the error but continue with other containers
				allLogs = append(allLogs, fmt.Sprintf("Error getting logs for pod %s container %s: %v", pod.Name, container.Name, err))
				continue
			}

			logContent, err := io.ReadAll(logs)
			_ = logs.Close()
			if err != nil {
				allLogs = append(allLogs, fmt.Sprintf("Error reading logs for pod %s container %s: %v", pod.Name, container.Name, err))
				continue
			}

			if len(logContent) > 0 {
				allLogs = append(allLogs, fmt.Sprintf("=== Pod: %s, Container: %s ===\n%s", pod.Name, container.Name, string(logContent)))
			}
		}
	}

	return fmt.Sprintf("%s", allLogs), nil
}
