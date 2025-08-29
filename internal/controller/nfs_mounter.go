package controller

import (
	"context"
	"fmt"
	"time"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NFSMounter handles NFS mounting using Kubernetes volumes
type NFSMounter struct {
	client client.Client
}

// NewNFSMounter creates a new NFS mounter
func NewNFSMounter(client client.Client) *NFSMounter {
	return &NFSMounter{
		client: client,
	}
}

// MountNFS creates a temporary pod to mount NFS and waits for it to be ready
func (n *NFSMounter) MountNFS(ctx context.Context, target storagev1alpha1.BackupTarget) (string, error) {
	logger := log.FromContext(ctx)

	if target.NFS == nil {
		return "", fmt.Errorf("no NFS configuration found for target %s", target.Name)
	}

	// Create a temporary pod to mount the NFS share
	podName := fmt.Sprintf("nfs-mount-%s-%s", target.Name, target.NFS.Server)
	mountPath := fmt.Sprintf("/mnt/nfs-%s", target.Name)

	// Create the temporary pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default", // TODO: Make this configurable
			Labels: map[string]string{
				"app":    "nfs-mount",
				"target": target.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "nfs-mount",
					Image: "busybox:1.35",
					Command: []string{
						"/bin/sh",
						"-c",
						fmt.Sprintf("mkdir -p %s && mount -t nfs %s:%s %s && echo 'NFS mounted successfully' && sleep 3600",
							mountPath, target.NFS.Server, target.NFS.Path, mountPath),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "nfs-mount",
							MountPath: mountPath,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: func() *bool { b := true; return &b }(),
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "nfs-mount",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	// Create the pod
	if err := n.client.Create(ctx, pod); err != nil {
		return "", fmt.Errorf("failed to create NFS mount pod: %w", err)
	}

	logger.Info("Created NFS mount pod",
		"target", target.Name,
		"pod", podName,
		"server", target.NFS.Server,
		"path", target.NFS.Path)

	// Wait for pod to be ready and NFS mounted
	if err := n.waitForPodReady(ctx, podName, "default"); err != nil {
		// Clean up the pod if it failed
		n.cleanupPod(ctx, podName, "default")
		return "", fmt.Errorf("NFS mount pod failed to become ready: %w", err)
	}

	logger.Info("NFS mount pod is ready", "target", target.Name, "pod", podName)
	return mountPath, nil
}

// waitForPodReady waits for a pod to be ready
func (n *NFSMounter) waitForPodReady(ctx context.Context, podName, namespace string) error {
	return wait.PollImmediate(2*time.Second, 60*time.Second, func() (bool, error) {
		pod := &corev1.Pod{}
		if err := n.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: namespace}, pod); err != nil {
			return false, err
		}

		// Check if pod is running
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}

		// Check if all containers are ready
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready {
				return false, nil
			}
		}

		return true, nil
	})
}

// cleanupPod deletes a pod and waits for it to be gone
func (n *NFSMounter) cleanupPod(ctx context.Context, podName, namespace string) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}

	if err := n.client.Delete(ctx, pod); err != nil {
		logger := log.FromContext(ctx)
		logger.Error(err, "Failed to delete NFS mount pod", "pod", podName)
	}
}

// UnmountNFS deletes the temporary NFS mount pod
func (n *NFSMounter) UnmountNFS(ctx context.Context, target storagev1alpha1.BackupTarget) error {
	logger := log.FromContext(ctx)

	if target.NFS == nil {
		return nil
	}

	podName := fmt.Sprintf("nfs-mount-%s-%s", target.Name, target.NFS.Server)
	namespace := "default" // TODO: Make this configurable

	// Delete the temporary pod
	n.cleanupPod(ctx, podName, namespace)

	logger.Info("Deleted NFS mount pod", "target", target.Name, "pod", podName)
	return nil
}

// GetMountPath returns the mount path for a target
func (n *NFSMounter) GetMountPath(targetName string) (string, bool) {
	// For now, return a predictable path
	// In a real implementation, you'd track the actual mounted paths
	mountPath := fmt.Sprintf("/mnt/nfs-%s", targetName)
	return mountPath, true
}

// CleanupAll deletes all NFS mount pods
func (n *NFSMounter) CleanupAll(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// List and delete all NFS mount pods
	var pods corev1.PodList
	if err := n.client.List(ctx, &pods, client.MatchingLabels{"app": "nfs-mount"}); err != nil {
		return fmt.Errorf("failed to list NFS mount pods: %w", err)
	}

	for _, pod := range pods.Items {
		if err := n.client.Delete(ctx, &pod); err != nil {
			logger.Error(err, "Failed to delete NFS mount pod", "pod", pod.Name)
		} else {
			logger.Info("Deleted NFS mount pod", "pod", pod.Name)
		}
	}

	return nil
}
