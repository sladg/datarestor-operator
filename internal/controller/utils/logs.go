package utils

import (
	"context"
	"fmt"
	"io"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fetchPodLogs retrieves logs from a pod's container
func fetchPodLogs(ctx context.Context, k8sClient *kubernetes.Clientset, pod *corev1.Pod, containerName string, previous bool) (string, error) {

	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("no containers found in pod %s in namespace %s", pod.Name, pod.Namespace)
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
		return "", fmt.Errorf("failed to get log stream from pod %s in namespace %s: %w", pod.Name, pod.Namespace, err)
	}
	defer func() {
		if cerr := logStream.Close(); cerr != nil {
			err = fmt.Errorf("failed to close log stream for pod %s in namespace %s: %w", pod.Name, pod.Namespace, cerr)
		}
	}()

	// Read all logs
	logBytes, err := io.ReadAll(logStream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs from pod %s in namespace %s: %w", pod.Name, pod.Namespace, err)
	}

	return string(logBytes), nil
}

// GetJobLogs retrieves logs from the most recent pod of a job.
// It follows a similar pattern to CreateResticJobWithOutput for consistency.
func GetJobLogs(ctx context.Context, deps *Dependencies, job corev1.ObjectReference) (string, error) {
	var jobObj *batchv1.Job
	err := deps.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, jobObj)
	if err != nil {
		return "", fmt.Errorf("failed to get job %s in namespace %s: %w", job.Name, job.Namespace, err)
	}

	// Find pods created by the job
	podList, err := FindPodsForJob(ctx, deps, jobObj)
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job %s in namespace %s: %w", job.Name, job.Namespace, err)
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s in namespace %s", job.Name, job.Namespace)
	}

	// Sort pods by creation time to find the most recent one
	sort.Slice(podList.Items, func(i, j int) bool {
		t1 := podList.Items[i].CreationTimestamp.Time
		t2 := podList.Items[j].CreationTimestamp.Time
		return t1.After(t2)
	})

	pod := &podList.Items[0]

	// Create a Kubernetes client
	k8sClient, err := kubernetes.NewForConfig(deps.Config)
	if err != nil {
		return "", fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get logs from the pod - use the first container by default
	containerName := ""
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	// Get logs from the pod
	return fetchPodLogs(ctx, k8sClient, pod, containerName, false)
}
