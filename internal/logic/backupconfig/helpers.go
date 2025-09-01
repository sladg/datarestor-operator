package logic

import (
	"context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// HandleSchedule processes scheduled backups for all PVCs managed by the BackupConfig.
// For each target with a backup schedule:
//  1. Finds all managed PVCs
//  2. Checks the last successful backup for each PVC
//  3. Creates new backups when needed based on the schedule
//
// The function ensures each PVC is backed up according to its target's schedule,
// tracking the last successful backup to prevent unnecessary operations.
func HandleSchedule(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, managedPVCs []string) error {
	log := deps.Logger.Named("handle-schedule")

	// Find all managed PVCs once since we'll need them for all targets
	selector := backupv1alpha1.Selector{
		Names:      managedPVCs,
		Namespaces: []string{backupConfig.Namespace},
	}

	pvcs, err := utils.FindMatchingPVCs(ctx, deps, &selector)
	if err != nil {
		log.Errorw("Failed to find managed PVCs", "error", err)
		return err
	}

	// Get existing ResticBackups to check their status using selector
	backupSelector := backupv1alpha1.Selector{
		Namespaces: []string{backupConfig.Namespace},
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"backup-config": backupConfig.Name,
			},
		},
	}

	backupList := &backupv1alpha1.ResticBackupList{}
	existingBackups, err := utils.FindMatchingResources[*backupv1alpha1.ResticBackup](ctx, deps, []backupv1alpha1.Selector{backupSelector}, backupList)
	if err != nil {
		log.Errorw("Failed to find existing backups", "error", err)
		return err
	}

	// Process each backup target that has a schedule
	for i := range backupConfig.Spec.BackupTargets {
		target := &backupConfig.Spec.BackupTargets[i]
		if target.BackupSchedule == "" {
			continue // Skip targets without schedule
		}

		// Process each PVC for this target
		for _, pvc := range pvcs {
			// Find the latest successful backup for this PVC and target
			var lastBackupTime *time.Time
			for _, backup := range existingBackups {
				if backup.Labels["pvc"] == pvc.Name && backup.Labels["backup-target"] == target.Name {
					if backup.Status.CompletionTime != nil && backup.Status.Phase == backupv1alpha1.PhaseCompleted {
						t := backup.Status.CompletionTime.Time
						if lastBackupTime == nil || t.After(*lastBackupTime) {
							lastBackupTime = &t
						}
					}
				}
			}

			// Create new backup if needed based on schedule
			if utils.ShouldPerformBackup(target.BackupSchedule, lastBackupTime) {
				log.Infow("Creating scheduled backup",
					"target", target.Name,
					"pvc", pvc.Name,
					"lastBackup", lastBackupTime)

				backupName := getScheduledBackupName(backupConfig.Name, target.Name, pvc.Name)
				if err := createResticBackup(ctx, deps, backupConfig, &pvc, backupName, *target); err != nil {
					log.Errorw("Failed to create scheduled backup",
						"pvc", pvc.Name,
						"target", target.Name,
						"error", err)
					continue
				}
			}
		}
	}

	return nil
}
