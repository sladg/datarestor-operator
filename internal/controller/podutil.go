package controller

import (
	context "context"
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
