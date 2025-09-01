package utils

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// LoggerFrom creates a new logger with a given name.
// This is a utility function to provide consistent logging across controllers.
func LoggerFrom(ctx context.Context, name string) *zap.SugaredLogger {
	// Create a named logger from the global zap instance
	return zap.S().Named(name)
}

// NewLogger creates a new named logger instance.
// This is used by controllers to get their specific logger.
func NewLogger(config interface{}) *zap.SugaredLogger {
	// For now, return the global sugared logger. In a real implementation,
	// you might want to configure the logger based on the config parameter.
	return zap.S()
}

// GetPodLogs retrieves logs from the first container of a pod belonging to a job.
func GetPodLogs(ctx context.Context, deps *Dependencies, namespace, jobName string) (string, error) {
	// Find the pod created by the job
	podList := &corev1.PodList{}
	if err := deps.Client.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{
		"job-name": jobName,
	}); err != nil {
		return "", fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", jobName)
	}

	// Get logs from the first pod
	pod := &podList.Items[0]
	return FetchPodLogs(ctx, deps.Config, pod, false)
}
