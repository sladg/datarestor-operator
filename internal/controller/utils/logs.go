package utils

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// FetchPodLogs retrieves logs from a pod's container
func FetchPodLogs(ctx context.Context, config *rest.Config, pod *corev1.Pod, previous bool) (string, error) {
	// Create a Kubernetes client
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get logs using typed client
	req := k8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Previous: previous,
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

// FetchPodLogsByName is a convenience wrapper for getting logs by pod name and namespace
func FetchPodLogsByName(ctx context.Context, config *rest.Config, podName, namespace string, previous bool) ([]byte, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}

	logs, err := FetchPodLogs(ctx, config, pod, previous)
	if err != nil {
		return nil, err
	}
	return []byte(logs), nil
}
