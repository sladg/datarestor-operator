package logic

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
)

// HandleScheduledBackups handles scheduled backups on a per-target basis.
func HandleScheduledBackups(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, managedPVCs []*corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("scheduled-backups")

	if len(managedPVCs) == 0 {
		log.Debug("No PVCs to backup")
		return nil
	}

	var targetsProcessed, backupsCreated, targetsSkipped int32

	// Process each backup target
	for i, target := range backupConfig.Spec.BackupTargets {
		processed, created, skipped := processScheduledTarget(ctx, deps, backupConfig, i, target, managedPVCs)
		targetsProcessed += processed
		backupsCreated += created
		targetsSkipped += skipped
	}

	if targetsProcessed > 0 {
		log.Infow("Scheduled backup cycle completed", "targetsProcessed", targetsProcessed, "backupsCreated", backupsCreated, "targetsSkipped", targetsSkipped, "totalPVCs", len(managedPVCs))
	}

	return nil
}

// processScheduledTarget handles scheduling for a single backup target
func processScheduledTarget(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, targetIndex int, target v1.BackupTarget, managedPVCs []*corev1.PersistentVolumeClaim) (processed, created, skipped int32) {
	log := deps.Logger.Named("target-scheduler")

	// Skip targets without a schedule
	if target.BackupSchedule == "" {
		return 0, 0, 1
	}

	// Get last backup time for scheduling decision
	lastBackupTime := getLastBackupTime(target)

	// Check if backup should be performed
	if !utils.ShouldPerformBackup(target.BackupSchedule, lastBackupTime) {
		log.Debugw("Skipping target - not scheduled", "target", target.Name)
		return 0, 0, 1
	}

	// Verify repository is ready
	if err := validateRepositoryForBackup(ctx, deps, target); err != nil {
		log.Warnw("Skipping target - repository issue", "target", target.Name, "error", err)
		return 0, 0, 1
	}

	// Mark target as running and create backups
	backupsCreated := createScheduledBackupsForTarget(ctx, deps, backupConfig, targetIndex, target, managedPVCs)

	if backupsCreated > 0 {
		log.Infow("Scheduled backups started", "target", target.Name, "backups", backupsCreated)
	}

	return 1, backupsCreated, 0
}

// getLastBackupTime extracts the last backup completion time from target status
func getLastBackupTime(target v1.BackupTarget) *time.Time {
	if target.LastRun != nil && !target.LastRun.CompletionTime.IsZero() {
		return &target.LastRun.CompletionTime.Time
	}
	return nil
}

// validateRepositoryForBackup checks if repository is ready for backup operations
func validateRepositoryForBackup(ctx context.Context, deps *utils.Dependencies, target v1.BackupTarget) error {
	_, err := utils.GetRepositoryForOperation(ctx, deps, target.Repository.Namespace, target.Repository.Name)
	return err
}

// createScheduledBackupsForTarget creates scheduled backups for all PVCs in a target
func createScheduledBackupsForTarget(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, targetIndex int, target v1.BackupTarget, managedPVCs []*corev1.PersistentVolumeClaim) int32 {
	log := deps.Logger.Named("backup-creator")

	// Mark target as running in the status
	if err := updateBackupTargetLastRun(ctx, deps, backupConfig, targetIndex, v1.BackupTargetStatusRunning, 0, 0, ""); err != nil {
		log.Errorw("Failed to mark target as running", "target", target.Name, "error", err)
	}

	var createdBackups int32
	var failedBackups int32

	// Create backup for each PVC
	for _, pvc := range managedPVCs {
		req := BackupRequest{
			PVC:        pvc,
			Repository: target.Repository,
			BackupType: v1.BackupTypeScheduled,
			SnapshotID: "", // Scheduled backups don't specify snapshot ID
		}

		if err := CreateBackupForPVC(ctx, deps, backupConfig, req); err != nil {
			failedBackups++
			log.Debugw("Backup creation failed", "target", target.Name, "pvc", pvc.Name, "error", err)
		} else {
			createdBackups++
		}
	}

	// Log summary for this target if there were failures
	if failedBackups > 0 {
		log.Warnw("Some backups failed to create", "target", target.Name, "failed", failedBackups, "total", len(managedPVCs))
	}

	return createdBackups
}
