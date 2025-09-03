package utils

import (
	"context"
	"fmt"
	"io"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// fetchPodLogs retrieves logs from a pod's container
func fetchPodLogs(ctx context.Context, k8sClient *kubernetes.Clientset, pod *corev1.Pod, containerName string, previous bool) (string, error) {
	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("no containers found in pod %s", pod.Name)
	}

	// Default to the first container if no name is specified
	if containerName == "" {
		containerName = pod.Spec.Containers[0].Name
	}

	// Get logs using typed client
	req := k8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: containerName,
		Previous:  previous,
	})

	logStream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get log stream: %w", err)
	}
	defer logStream.Close()

	// Read all logs
	logBytes, err := io.ReadAll(logStream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(logBytes), nil
}

// GetJobLogs retrieves logs from the most recent pod of a job.
func GetJobLogs(ctx context.Context, deps *Dependencies, job *batchv1.Job) (string, error) {
	// Find pods created by the job
	podList, err := FindPodsForJob(ctx, deps, job)
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job %s: %w", job.Name, err)
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", job.Name)
	}

	// Sort pods by creation time to find the most recent one
	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].CreationTimestamp.Time.After(podList.Items[j].CreationTimestamp.Time)
	})

	pod := &podList.Items[0]

	// Create a Kubernetes client
	k8sClient, err := kubernetes.NewForConfig(deps.Config)
	if err != nil {
		return "", fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get logs from the pod
	return fetchPodLogs(ctx, k8sClient, pod, "", false)
}
