package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HandleAnnotations processes backup and restore annotations on both BackupConfig and its PVCs.
// It supports two types of manual operations:
//  1. BackupConfig-level backup: Triggers backup for all managed PVCs when BackupConfig is annotated
//  2. PVC-level operations:
//     - Manual backup: Triggers backup for a specific PVC when it's annotated
//     - Manual restore: Creates a restore operation for a specific PVC
//
// The function ensures idempotency by removing processed annotations after successful operations.
// For backups, all configured targets in the BackupConfig are used.
func HandleAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("handle-annotations")

	// Log managedPVCs input
	log.Infow("HandleAnnotations called", "backupConfig", backupConfig.Name)

	// Process BackupConfig annotations first - these affect all managed PVCs
	if annotations := backupConfig.GetAnnotations(); annotations != nil {
		if backupName, ok := annotations[constants.AnnotationManualBackup]; ok && backupName != "" {
			log.Infow("Manual backup annotation found on BackupConfig", "name", backupConfig.Name, "value", backupName)

			// Create backups for each PVC using all configured targets
			for _, pvc := range pvcs {
				for i := range backupConfig.Spec.BackupTargets {
					target := &backupConfig.Spec.BackupTargets[i]
					repo, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, backupConfig.Namespace, target.Repository.Name)
					if err != nil {
						log.Errorw("Failed to get ResticRepository for manual backup", "repo", target.Repository.Name, "error", err)
						continue
					}

					if err := createResticBackup(ctx, deps, backupConfig, pvc, backupName, repo); err != nil {
						log.Errorw("Failed to create manual ResticBackup from BackupConfig annotation",
							"pvc", pvc.Name, "target", target.Name, "error", err)
						continue
					}
				}
			}

			// Clear the processed annotations to prevent re-triggering
			if err := utils.AnnotateResources(ctx, deps, []client.Object{backupConfig}, map[string]*string{
				constants.AnnotationManualBackup: nil,
			}); err != nil {
				log.Errorw("Failed to remove manual backup annotations from BackupConfig", "error", err)
				return err
			}
		}
	}

	// Process individual PVC annotations - allows per-PVC operations
	for _, pvc := range pvcs {
		annotations := pvc.GetAnnotations()
		if annotations == nil {
			continue
		}

		// Handle PVC-level manual backup request
		if backupName, ok := annotations[constants.AnnotationManualBackup]; ok && backupName != "" {
			log.Infow("Manual backup annotation found on PVC", "pvc", pvc.Name, "value", backupName)

			// Create backups using all configured targets
			for i := range backupConfig.Spec.BackupTargets {
				target := &backupConfig.Spec.BackupTargets[i]
				repo, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, backupConfig.Namespace, target.Repository.Name)
				if err != nil {
					log.Errorw("Failed to get ResticRepository for manual backup", "repo", target.Repository.Name, "error", err)
					continue
				}
				if err := createResticBackup(ctx, deps, backupConfig, pvc, backupName, repo); err != nil {
					log.Errorw("Failed to create manual ResticBackup from PVC annotation",
						"pvc", pvc.Name, "target", target.Name, "error", err)
					continue
				}
			}

			// Clear the processed backup annotations
			if err := utils.AnnotateResources(ctx, deps, []client.Object{pvc}, map[string]*string{
				constants.AnnotationManualBackup: nil,
			}); err != nil {
				log.Errorw("Failed to remove manual backup annotations from PVC", "error", err)
				continue
			}
		}

		// Handle PVC-level manual restore request
		if restoreRequest, ok := annotations[constants.AnnotationManualRestore]; ok && restoreRequest != "" {
			log.Infow("Manual restore annotation found on PVC", "pvc", pvc.Name, "value", restoreRequest)

			snapshotID := utils.GetSnapshotIDFromAnnotation(ctx, deps, backupConfig, restoreRequest)

			// Get the ResticRepository
			// This logic assumes the first target is the one to restore from, which might need refinement.
			if len(backupConfig.Spec.BackupTargets) > 0 {
				target := backupConfig.Spec.BackupTargets[0]
				repo, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, backupConfig.Namespace, target.Repository.Name)
				if err != nil {
					log.Errorw("Failed to get ResticRepository for manual restore", "repo", target.Repository.Name, "error", err)
					continue
				}

				// Create a ResticRestore CR to handle the restore operation
				if err := createResticRestore(ctx, deps, backupConfig, pvc, repo, snapshotID); err != nil {
					log.Errorw("Failed to create ResticRestore", "pvc", pvc.Name, "error", err)
					continue
				}
			} else {
				log.Warnw("No backup targets defined for manual restore", "backupconfig", backupConfig.Name)
			}

			// Clear the processed restore annotations
			if err := utils.AnnotateResources(ctx, deps, []client.Object{pvc}, map[string]*string{
				constants.AnnotationManualRestore: nil,
			}); err != nil {
				log.Errorw("Failed to remove manual restore annotations from PVC", "error", err)
				continue
			}
		}
	}

	return nil
}
