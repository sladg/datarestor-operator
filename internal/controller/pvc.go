package controller

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// findMatchingPVCs finds PVCs that match the BackupConfig selector
func (r *BackupConfigReconciler) findMatchingPVCs(ctx context.Context, backupConfig *backupv1alpha1.BackupConfig) ([]corev1.PersistentVolumeClaim, error) {
	logger := LoggerFrom(ctx, "pvc").
		WithValues("name", backupConfig.Name)

	logger.Starting("find matching PVCs")
	var pvcs []corev1.PersistentVolumeClaim

	// Handle namespace-specific selection
	namespaces := backupConfig.Spec.PVCSelector.Namespaces
	if len(namespaces) == 0 {
		// If no namespaces specified, search all namespaces
		namespaces = []string{""}
		logger.Debug("No namespaces specified, searching all")
	} else {
		logger.WithValues("namespaces", namespaces).Debug("Searching specific namespaces")
	}

	for _, namespace := range namespaces {
		nsLogger := logger.WithValues("namespace", namespace)
		var namespacePVCs corev1.PersistentVolumeClaimList

		if namespace == "" {
			// Search all namespaces
			if err := r.List(ctx, &namespacePVCs); err != nil {
				nsLogger.Failed("list PVCs across all namespaces", err)
				return nil, err
			}
		} else {
			// Search specific namespace
			if err := r.List(ctx, &namespacePVCs, client.InNamespace(namespace)); err != nil {
				nsLogger.Failed("list PVCs in namespace", err)
				return nil, err
			}
		}

		// Filter by label selector
		if backupConfig.Spec.PVCSelector.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(backupConfig.Spec.PVCSelector.LabelSelector)
			if err != nil {
				nsLogger.Failed("parse label selector", err)
				return nil, err
			}

			nsLogger.Debug("Filtering by label selector")
			for _, pvc := range namespacePVCs.Items {
				if selector.Matches(labels.Set(pvc.Labels)) {
					pvcs = append(pvcs, pvc)
				}
			}
		} else {
			// If no label selector, include all PVCs from the namespace
			nsLogger.Debug("No label selector, including all PVCs")
			pvcs = append(pvcs, namespacePVCs.Items...)
		}
	}

	// Filter by specific names if provided
	if len(backupConfig.Spec.PVCSelector.Names) > 0 {
		logger.WithValues("names", backupConfig.Spec.PVCSelector.Names).Debug("Filtering by specific names")
		var filteredPVCs []corev1.PersistentVolumeClaim
		nameSet := make(map[string]bool)
		for _, name := range backupConfig.Spec.PVCSelector.Names {
			nameSet[name] = true
		}

		for _, pvc := range pvcs {
			if nameSet[pvc.Name] {
				filteredPVCs = append(filteredPVCs, pvc)
			}
		}
		pvcs = filteredPVCs
	}

	logger.WithValues("count", len(pvcs)).Completed("find matching PVCs")
	return pvcs, nil
}

// findObjectsForPVC finds BackupConfig objects for a given PVC
func (r *BackupConfigReconciler) findObjectsForPVC(ctx context.Context, obj client.Object) []reconcile.Request {
	pvc := obj.(*corev1.PersistentVolumeClaim)

	var backupConfigs backupv1alpha1.BackupConfigList
	if err := r.List(ctx, &backupConfigs); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, backupConfig := range backupConfigs.Items {
		// Check if PVC matches the selector
		matchedPVCs, err := r.findMatchingPVCs(ctx, &backupConfig)
		if err != nil {
			continue
		}

		for _, matchedPVC := range matchedPVCs {
			if matchedPVC.Name == pvc.Name && matchedPVC.Namespace == pvc.Namespace {
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

	return requests
}
