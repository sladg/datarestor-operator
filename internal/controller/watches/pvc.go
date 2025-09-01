package watches

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// pvcToRequests converts a PVC to a list of reconcile requests
func pvcToRequests(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) []reconcile.Request {
	// List all BackupConfigs in the namespace
	var backupConfigs backupv1alpha1.BackupConfigList
	if err := c.List(ctx, &backupConfigs, client.InNamespace(pvc.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupConfig := range backupConfigs.Items {
		// Check if PVC matches any selector
		for _, selector := range backupConfig.Spec.Selectors {
			// Check if PVC matches the label selector
			if selector.LabelSelector != nil {
				labelSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
				if err != nil {
					continue
				}
				if !labelSelector.Matches(labels.Set(pvc.Labels)) {
					continue
				}
			}

			// Check if PVC is in the namespaces list
			if len(selector.Namespaces) > 0 {
				found := false
				for _, ns := range selector.Namespaces {
					if ns == pvc.Namespace {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Check if PVC is in the names list
			if len(selector.Names) > 0 {
				found := false
				for _, name := range selector.Names {
					if name == pvc.Name {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// If we got here, the PVC matches this selector
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      backupConfig.Name,
					Namespace: backupConfig.Namespace,
				},
			})
			break // No need to check other selectors for this BackupConfig
		}
	}

	return requests
}

// FindObjectsForPVC returns BackupConfig reconcile requests for a PVC
func FindObjectsForPVC(c client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		pvc, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			return nil
		}
		return pvcToRequests(ctx, c, pvc)
	}
}

// FindObjectsForPod returns BackupConfig reconcile requests for a Pod
func FindObjectsForPod(c client.Client) handler.MapFunc {
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
			if err := c.Get(ctx, client.ObjectKey{
				Name:      pvcName,
				Namespace: pod.Namespace,
			}, pvc); err != nil {
				continue
			}

			// Find BackupConfigs for this PVC
			pvcRequests := pvcToRequests(ctx, c, pvc)
			requests = append(requests, pvcRequests...)
		}

		return requests
	}
}
