package logic

import (
	"context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getManualBackupName generates a name for a manual backup operation.
// If a custom name is provided via the AnnotationManualBackupName annotation, it will be used.
// Otherwise, a name is generated using the format: "{backupConfig}-{pvc}-manual-{timestamp}".
func getManualBackupName(annotations map[string]string, backupConfigName, pvcName string) string {
	if name, ok := annotations[constants.AnnotationManualBackupName]; ok && name != "" {
		return name
	}
	return fmt.Sprintf("%s-%s-manual-%d", backupConfigName, pvcName, time.Now().Unix())
}

// getScheduledBackupName generates a consistent name for a scheduled backup operation
// using the format: "{backupConfig}-{target}-{pvc}-{timestamp}".
func getScheduledBackupName(backupConfigName, targetName, pvcName string) string {
	return fmt.Sprintf("%s-%s-%s-%d", backupConfigName, targetName, pvcName, time.Now().Unix())
}

// HandleAnnotations processes backup and restore annotations on both BackupConfig and its PVCs.
// It supports two types of manual operations:
//  1. BackupConfig-level backup: Triggers backup for all managed PVCs when BackupConfig is annotated
//  2. PVC-level operations:
//     - Manual backup: Triggers backup for a specific PVC when it's annotated
//     - Manual restore: Creates a restore operation for a specific PVC
//
// The function ensures idempotency by removing processed annotations after successful operations.
// For backups, all configured targets in the BackupConfig are used.
func HandleAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, managedPVCs []string) error {
	log := deps.Logger.Named("handle-annotations")

	// Find all managed PVCs once since they're needed for both BackupConfig and PVC-level operations
	selector := backupv1alpha1.Selector{
		Names:      managedPVCs,
		Namespaces: []string{backupConfig.Namespace},
	}

	pvcs, err := utils.FindMatchingPVCs(ctx, deps, &selector)
	if err != nil {
		log.Errorw("Failed to find managed PVCs", "error", err)
		return err
	}

	// Process BackupConfig annotations first - these affect all managed PVCs
	if annotations := backupConfig.GetAnnotations(); annotations != nil {
		if backupRequest, ok := annotations[constants.AnnotationManualBackup]; ok && backupRequest == "true" {
			log.Infow("Manual backup annotation found on BackupConfig", "name", backupConfig.Name)

			// Create backups for each PVC using all configured targets
			for _, pvc := range pvcs {
				for i := range backupConfig.Spec.BackupTargets {
					target := &backupConfig.Spec.BackupTargets[i]
					backupName := getManualBackupName(annotations, backupConfig.Name, pvc.Name)
					if err := createResticBackup(ctx, deps, backupConfig, &pvc, backupName, *target); err != nil {
						log.Errorw("Failed to create manual ResticBackup from BackupConfig annotation",
							"pvc", pvc.Name, "target", target.Name, "error", err)
						continue
					}
				}
			}

			// Clear the processed annotations to prevent re-triggering
			if err := utils.AnnotateResources(ctx, deps, []client.Object{backupConfig}, map[string]string{
				constants.AnnotationManualBackup:     "", // Empty string removes the annotation
				constants.AnnotationManualBackupName: "", // Empty string removes the annotation
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
		if backupRequest, ok := annotations[constants.AnnotationManualBackup]; ok && backupRequest == "true" {
			log.Infow("Manual backup annotation found on PVC", "pvc", pvc.Name)

			// Create backups using all configured targets
			for i := range backupConfig.Spec.BackupTargets {
				target := &backupConfig.Spec.BackupTargets[i]
				backupName := getManualBackupName(annotations, backupConfig.Name, pvc.Name)
				if err := createResticBackup(ctx, deps, backupConfig, &pvc, backupName, *target); err != nil {
					log.Errorw("Failed to create manual ResticBackup from PVC annotation",
						"pvc", pvc.Name, "target", target.Name, "error", err)
					continue
				}
			}

			// Clear the processed backup annotations
			if err := utils.AnnotateResources(ctx, deps, []client.Object{&pvc}, map[string]string{
				constants.AnnotationManualBackup:     "", // Empty string removes the annotation
				constants.AnnotationManualBackupName: "", // Empty string removes the annotation
			}); err != nil {
				log.Errorw("Failed to remove manual backup annotations from PVC", "error", err)
				continue
			}
		}

		// Handle PVC-level manual restore request
		if restoreRequest, ok := annotations[constants.AnnotationManualRestore]; ok && restoreRequest != "" {
			log.Infow("Manual restore annotation found on PVC", "pvc", pvc.Name, "backup", restoreRequest)

			// Create a ResticRestore CR to handle the restore operation
			if err := createResticRestore(ctx, deps, backupConfig, &pvc); err != nil {
				log.Errorw("Failed to create ResticRestore", "pvc", pvc.Name, "error", err)
				continue
			}

			// Clear the processed restore annotations
			if err := utils.AnnotateResources(ctx, deps, []client.Object{&pvc}, map[string]string{
				constants.AnnotationManualRestore:     "", // Empty string removes the annotation
				constants.AnnotationManualRestoreName: "", // Empty string removes the annotation
			}); err != nil {
				log.Errorw("Failed to remove manual restore annotations from PVC", "error", err)
				continue
			}
		}
	}

	return nil
}

// DiscoverMatchingPVCs discovers all PVCs matching BackupConfig selectors. Returns the list of matching PVC names.
func DiscoverMatchingPVCs(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig) ([]string, error) {
	log := deps.Logger.Named("discover-pvcs")
	list := &corev1.PersistentVolumeClaimList{}
	pvcs, err := utils.FindMatchingResources[*corev1.PersistentVolumeClaim](ctx, deps, backupConfig.Spec.Selectors, list)
	if err != nil {
		log.Errorw("Failed to find matching PVCs", "error", err)
		return nil, err
	}
	var managedPVCs []string
	for _, pvc := range pvcs {
		managedPVCs = append(managedPVCs, pvc.Name)
	}
	return managedPVCs, nil
}

// UpdateStatusPVCsCount updates the BackupConfig status with the count of managed PVCs.
func UpdateStatusPVCsCount(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, managedPVCs []string) error {
	log := deps.Logger.Named("update-status-pvcs-count")
	backupConfig.Status.PVCsCount = int32(len(managedPVCs))

	if err := deps.Client.Status().Update(ctx, backupConfig); err != nil {
		log.Errorw("Failed to update BackupConfig status", "error", err)
		return err
	}

	return nil
}

// isOwnedByBackupConfig checks if a ResticRepository is owned by the given BackupConfig.
func isOwnedByBackupConfig(repo *backupv1alpha1.ResticRepository, backupConfig *backupv1alpha1.BackupConfig) bool {
	for _, ownerRef := range repo.OwnerReferences {
		if ownerRef.Kind == "BackupConfig" &&
			ownerRef.Name == backupConfig.Name &&
			ownerRef.UID == backupConfig.UID {
			return true
		}
	}
	return false
}
