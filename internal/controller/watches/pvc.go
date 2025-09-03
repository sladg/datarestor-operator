package watches

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// pvcToRequests converts a PVC to a list of reconcile requests
func pvcToRequests(ctx context.Context, deps *utils.Dependencies, pvc *corev1.PersistentVolumeClaim) []reconcile.Request {
	// List all BackupConfigs in the namespace
	backupConfigList := &v1.BackupConfigList{}
	selector := utils.SelectorInNamespace(pvc.Namespace)
	backupConfigs, err := utils.FindMatchingResources[*v1.BackupConfig](ctx, deps, []v1.Selector{selector}, backupConfigList)
	if err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupConfig := range backupConfigs {
		// Check if PVC matches any selector
		if utils.MatchesAnySelector(pvc, backupConfig.Spec.Selectors) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      backupConfig.Name,
					Namespace: backupConfig.Namespace,
				},
			})
		}
	}

	return requests
}

// FindObjectsForPVC returns BackupConfig reconcile requests for a PVC
func FindObjectsForPVC(deps *utils.Dependencies) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		pvc, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			return nil
		}
		return pvcToRequests(ctx, deps, pvc)
	}
}

// FindObjectsForPod returns BackupConfig reconcile requests for a Pod
func FindObjectsForPod(deps *utils.Dependencies) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return nil
		}

		// Get all PVCs used by the pod
		var pvcNames []string
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
			}
		}

		// No PVCs used by this pod
		if len(pvcNames) == 0 {
			return nil
		}

		// Get all PVCs
		var requests []reconcile.Request
		for _, pvcName := range pvcNames {
			pvc := &corev1.PersistentVolumeClaim{}
			if err := deps.Get(ctx, client.ObjectKey{
				Name:      pvcName,
				Namespace: pod.Namespace,
			}, pvc); err != nil {
				continue
			}

			// Find BackupConfigs for this PVC
			pvcRequests := pvcToRequests(ctx, deps, pvc)
			requests = append(requests, pvcRequests...)
		}

		return requests
	}
}
