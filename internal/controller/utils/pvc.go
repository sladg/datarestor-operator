package utils

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// For each selector, find a PVCs that match and return a request for each PVC
func GetPVCsForConfig(ctx context.Context, deps *Dependencies, config *v1.Config) []*corev1.PersistentVolumeClaim {
	requests := []*corev1.PersistentVolumeClaim{}

	seen := make(map[types.UID]bool)

	for _, selector := range config.Spec.Selectors {
		opts := []client.ListOption{}
		if selector.MatchLabels != nil {
			opts = append(opts, client.MatchingLabels(selector.MatchLabels))
		}
		if selector.MatchNamespaces != nil {
			for _, namespace := range selector.MatchNamespaces {
				opts = append(opts, client.InNamespace(namespace))
			}
		}

		pvcs := &corev1.PersistentVolumeClaimList{}
		err := deps.List(ctx, pvcs, opts...)
		if err != nil {
			return nil
		}

		for _, pvc := range pvcs.Items {
			uid := pvc.GetUID()
			if seen[uid] {
				continue
			}

			// Skip PVCs that are being deleted
			if IsPVCBeingDeleted(&pvc) {
				continue
			}

			seen[uid] = true
			requests = append(requests, &pvc)
		}
	}

	return requests
}

func FilterNewPVCs(managedPVCs []*corev1.PersistentVolumeClaim, Logger *zap.SugaredLogger) []*corev1.PersistentVolumeClaim {
	log := Logger.Named("[FilterNewPVCs]")
	// Pre-allocate with length 0 and capacity based on expected filtered size
	newPVCs := make([]*corev1.PersistentVolumeClaim, 0, len(managedPVCs)/10)

	for _, pvc := range managedPVCs {
		// Check if PVC is too old to be considered "new"
		pvcAge := time.Since(pvc.CreationTimestamp.Time)
		if pvcAge > constants.MaxAgeForNewPVC {
			log.Debugw("PVC is too old for auto-restore", "pvc", pvc.Name, "age", pvcAge)
			continue
		}

		log.Debugw("PVC qualifies as new", "pvc", pvc.Name, "age", pvcAge)
		newPVCs = append(newPVCs, pvc)
	}

	return newPVCs
}

func FilterUnclaimedPVCs(managedPVCs []*corev1.PersistentVolumeClaim, Logger *zap.SugaredLogger) []*corev1.PersistentVolumeClaim {
	log := Logger.Named("[FilterUnclaimedPVCs]")
	// Pre-allocate with length 0 and capacity based on expected filtered size
	newPVCs := make([]*corev1.PersistentVolumeClaim, 0, len(managedPVCs)/10)

	for _, pvc := range managedPVCs {
		// Check if we've already processed this PVC for auto-restore
		if pvc.Annotations != nil && pvc.Annotations[constants.AnnAutoRestored] == "true" {
			log.Debugw("PVC already processed for auto-restore", "pvc", pvc.Name)
			continue
		}

		if pvc.Annotations != nil && pvc.Annotations[constants.AnnRestore] != "" {
			log.Debugw("PVC has manual restore annotation", "pvc", pvc.Name)
			continue
			// @FIXME: We continue here, but we process it later on in next loop when AnnRestore is removed because it was picked up by Manaul. Fix this.
		}

		log.Debugw("PVC qualifies as unclaimed", "pvc", pvc.Name)
		newPVCs = append(newPVCs, pvc)
	}

	return newPVCs
}

// IsPVCBeingDeleted checks if a PVC is being deleted (has DeletionTimestamp set)
func IsPVCBeingDeleted(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.DeletionTimestamp != nil
}

// IsPVCDeletedOrBeingDeleted checks if a PVC is deleted (not found) or being deleted
func IsPVCDeletedOrBeingDeleted(ctx context.Context, deps *Dependencies, pvcRef corev1.ObjectReference) bool {
	pvc := &corev1.PersistentVolumeClaim{}
	err := deps.Get(ctx, client.ObjectKey{Name: pvcRef.Name, Namespace: pvcRef.Namespace}, pvc)
	if err != nil {
		// PVC not found - consider it deleted
		return true
	}
	// PVC found - check if it's being deleted
	return pvc.DeletionTimestamp != nil
}

// @TODO: Check creation time of PVC vs. creation time of config. If PVC is older, skip it.
