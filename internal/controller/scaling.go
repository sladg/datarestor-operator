package controller

import (
	context "context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

	var backupConfigs backupv1alpha1.BackupConfigList
	if err := r.List(ctx, &backupConfigs); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupConfig := range backupConfigs.Items {
		matchedPVCs, err := r.findMatchingPVCs(ctx, &backupConfig)
		if err != nil {
			continue
		}

		for _, matchedPVC := range matchedPVCs {
			for _, pvcName := range pvcNames {
				if matchedPVC.Name == pvcName && matchedPVC.Namespace == pod.Namespace {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      backupConfig.Name,
							Namespace: backupConfig.Namespace,
						},
					})
					break
				}
			}
		}
	}

	return requests
}
