package controller

import (
	context "context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *BackupConfigReconciler) isPodHealthy(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	if pod.Status.StartTime == nil || time.Since(pod.Status.StartTime.Time) < 30*time.Second {
		return false
	}
	return true
}

// startPodsAfterBackup starts pods that were stopped for backup
func (r *BackupConfigReconciler) startPodsAfterBackup(ctx context.Context, pvc corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "pod").
		WithPVC(pvc)

	logger.Starting("start pods")

	// Find all pods using this PVC - try field index first, fallback to manual search
	var pods corev1.PodList
	err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name})
	if err != nil {
		// Fallback: manually search for pods in the same namespace
		logger.Debug("Field index not available, using manual search")
		var allPods corev1.PodList
		if err := r.List(ctx, &allPods, client.InNamespace(pvc.Namespace)); err != nil {
			logger.Failed("list pods", err)
			return fmt.Errorf("failed to list pods in namespace: %w", err)
		}

		// Filter pods that use this PVC
		for _, pod := range allPods.Items {
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
					pods.Items = append(pods.Items, pod)
					break
				}
			}
		}
	}

	logger.WithValues("count", len(pods.Items)).Debug("Found pods")

	for _, pod := range pods.Items {
		podLogger := logger.WithValues("pod", pod.Name)

		// Find the owner reference
		ownerRef := r.getOwnerReference(&pod)
		if ownerRef == nil {
			podLogger.Debug("No owner reference")
			continue
		}

		// Scale up the owner resource
		podLogger.WithValues("owner", ownerRef.Kind).Debug("Scaling up resource")
		if err := r.scaleUpResource(ctx, ownerRef, &pod); err != nil {
			podLogger.Failed("scale up", err)
			continue
		}
	}

	logger.Completed("start pods")

	return nil
}

// stopPodsForBackup stops pods using the PVC for data integrity during backup
func (r *BackupConfigReconciler) stopPodsForBackup(ctx context.Context, pvc corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "pod").
		WithPVC(pvc)

	logger.Starting("stop pods")

	// Find all pods using this PVC - try field index first, fallback to manual search
	var pods corev1.PodList
	err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name})
	if err != nil {
		// Fallback: manually search for pods in the same namespace
		logger.Debug("Field index not available, using manual search")
		var allPods corev1.PodList
		if err := r.List(ctx, &allPods, client.InNamespace(pvc.Namespace)); err != nil {
			logger.Failed("list pods", err)
			return fmt.Errorf("failed to list pods in namespace: %w", err)
		}

		// Filter pods that use this PVC
		for _, pod := range allPods.Items {
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
					pods.Items = append(pods.Items, pod)
					break
				}
			}
		}
	}

	logger.WithValues("count", len(pods.Items)).Debug("Found pods")

	for _, pod := range pods.Items {
		podLogger := logger.WithValues("pod", pod.Name)

		// Find the owner reference (Deployment, StatefulSet, etc.)
		ownerRef := r.getOwnerReference(&pod)
		if ownerRef == nil {
			podLogger.Debug("No owner reference")
			continue
		}

		// Scale down the owner resource
		podLogger.WithValues("owner", ownerRef.Kind).Debug("Scaling down resource")
		if err := r.scaleDownResource(ctx, ownerRef, &pod); err != nil {
			podLogger.Failed("scale down", err)
			continue
		}
	}

	logger.Completed("stop pods")

	return nil
}

func (r *BackupConfigReconciler) cleanupBackupPod(ctx context.Context, pod *corev1.Pod) {
	logger := LoggerFrom(ctx, "pod").
		WithValues("pod", pod.Name)

	logger.Starting("cleanup")
	if err := r.Delete(ctx, pod); err != nil {
		logger.Failed("delete pod", err)
	} else {
		logger.Completed("cleanup")
	}
}

func (r *BackupConfigReconciler) waitForPodReady(ctx context.Context, pod *corev1.Pod) error {
	logger := LoggerFrom(ctx, "pod").
		WithValues("pod", pod.Name)

	logger.Starting("wait for ready")

	// Wait up to 5 minutes for pod to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			err := fmt.Errorf("timeout waiting for pod to be ready: %s", pod.Name)
			logger.Failed("wait for ready", err)
			return err
		case <-ticker.C:
			// Get the latest pod status
			var currentPod corev1.Pod
			if err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &currentPod); err != nil {
				logger.Failed("get pod status", err)
				continue
			}

			if currentPod.Status.Phase == corev1.PodRunning {
				logger.Completed("wait for ready")
				return nil
			}

			if currentPod.Status.Phase == corev1.PodFailed || currentPod.Status.Phase == corev1.PodUnknown {
				err := fmt.Errorf("pod failed or unknown: %s", currentPod.Status.Message)
				logger.Failed("wait for ready", err)
				return err
			}

			logger.WithValues("phase", currentPod.Status.Phase).Debug("Pod not ready yet")
		}
	}
}

// findObjectsForPod finds BackupConfig objects for a given Pod
func (r *BackupConfigReconciler) findObjectsForPod(ctx context.Context, obj client.Object) []reconcile.Request {
	pod := obj.(*corev1.Pod)

	// Find PVCs used by the pod
	var pvcNames []string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}

	if len(pvcNames) == 0 {
		return nil
	}

	var pvcBackups backupv1alpha1.BackupConfigList
	if err := r.List(ctx, &pvcBackups); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, pvcBackup := range pvcBackups.Items {
		matchedPVCs, err := r.findMatchingPVCs(ctx, &pvcBackup)
		if err != nil {
			continue
		}

		for _, matchedPVC := range matchedPVCs {
			for _, pvcName := range pvcNames {
				if matchedPVC.Name == pvcName && matchedPVC.Namespace == pod.Namespace {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      pvcBackup.Name,
							Namespace: pvcBackup.Namespace,
						},
					})
					break
				}
			}
		}
	}

	return requests
}
