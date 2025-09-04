package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleAutoRestore handles the auto-restore logic for newly claimed PVCs.
func HandleAutoRestore(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, managedPVCs []*corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("handle-auto-restore")
	if !backupConfig.Spec.AutoRestore {
		return nil
	}

	for _, pvc := range managedPVCs {
		// If the PVC already has the restore finalizer, it's either being restored or has been restored.
		if controllerutil.ContainsFinalizer(pvc, constants.ResticRestoreFinalizer) {
			log.Debugw("PVC already has restore finalizer, skipping auto-restore", "pvc", pvc.Name)
			continue
		}

		log.Infow("New PVC detected, creating ResticRestore for auto-restore", "pvc", pvc.Name)

		// Create auto-restore for each backup target
		for _, target := range backupConfig.Spec.BackupTargets {
			req := RestoreRequest{
				PVC:         pvc,
				Repository:  target.Repository,
				RestoreType: v1.RestoreTypeAutomated,
				SnapshotID:  "", // Auto-restore uses latest/empty
			}
			if err := CreateRestoreForPVC(ctx, deps, backupConfig, req); err != nil {
				continue // Continue with other targets even if one fails
			}
		}
	}
	return nil
}
