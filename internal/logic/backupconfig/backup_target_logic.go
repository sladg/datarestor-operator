package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// updateBackupTargetLastRun updates the LastRun information for a backup target
func updateBackupTargetLastRun(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, targetIndex int, status v1.BackupTargetStatus, successfulBackups, failedBackups int32, errorMessage string) error {
	log := deps.Logger.Named("update-backup-target-lastrun")

	if targetIndex >= len(backupConfig.Spec.BackupTargets) {
		return fmt.Errorf("target index %d out of range", targetIndex)
	}

	target := &backupConfig.Spec.BackupTargets[targetIndex]

	// Initialize LastRun if it doesn't exist
	if target.LastRun == nil {
		target.LastRun = &v1.BackupTargetLastRun{}
	}

	now := time.Now()

	// Update based on status
	switch status {
	case v1.BackupTargetStatusRunning:
		target.LastRun.StartTime = metav1.Time{Time: now}
		target.LastRun.Status = status
		target.LastRun.SuccessfulBackups = 0
		target.LastRun.FailedBackups = 0
		target.LastRun.ErrorMessage = ""
	case v1.BackupTargetStatusCompleted, v1.BackupTargetStatusFailed, v1.BackupTargetStatusPartial:
		target.LastRun.CompletionTime = metav1.Time{Time: now}
		target.LastRun.Status = status
		target.LastRun.SuccessfulBackups = successfulBackups
		target.LastRun.FailedBackups = failedBackups
		target.LastRun.ErrorMessage = errorMessage
	}

	log.Debugw("Updated backup target last run", "target", target.Name, "status", status, "successful", successfulBackups, "failed", failedBackups)

	// Update the BackupConfig status
	return deps.Status().Update(ctx, backupConfig)
}

// MonitorScheduledBackups monitors the status of scheduled backups and updates BackupTarget LastRun information
func MonitorScheduledBackups(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) error {
	log := deps.Logger.Named("monitor-scheduled-backups")

	// Get scheduled backups grouped by target
	backupsByTarget, err := getScheduledBackupsByTarget(ctx, deps, backupConfig)
	if err != nil {
		return err
	}

	// Update each target's LastRun based on backup statuses
	for i, target := range backupConfig.Spec.BackupTargets {
		if err := monitorTargetBackups(ctx, deps, backupConfig, i, target, backupsByTarget); err != nil {
			log.Errorw("Failed to monitor target backups", "target", target.Name, "error", err)
		}
	}

	return nil
}

// getScheduledBackupsByTarget gets all scheduled backups owned by the BackupConfig and groups them by target
func getScheduledBackupsByTarget(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) (map[string][]*v1.ResticBackup, error) {
	log := deps.Logger.Named("get-scheduled-backups")

	// Find all ResticBackup resources owned by this BackupConfig
	backupList := &v1.ResticBackupList{}
	if err := deps.List(ctx, backupList, &client.MatchingFields{
		"metadata.ownerReferences.uid": string(backupConfig.UID),
	}); err != nil {
		log.Errorw("Failed to list ResticBackup resources", "error", err)
		return nil, err
	}

	// Group backups by target repository
	backupsByTarget := make(map[string][]*v1.ResticBackup)
	for i := range backupList.Items {
		backup := &backupList.Items[i]
		if backup.Spec.Type == v1.BackupTypeScheduled {
			targetKey := fmt.Sprintf("%s/%s", backup.Spec.Repository.Namespace, backup.Spec.Repository.Name)
			backupsByTarget[targetKey] = append(backupsByTarget[targetKey], backup)
		}
	}

	return backupsByTarget, nil
}

// monitorTargetBackups monitors and updates the status for a specific backup target
func monitorTargetBackups(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, targetIndex int, target v1.BackupTarget, backupsByTarget map[string][]*v1.ResticBackup) error {
	log := deps.Logger.Named("monitor-target-backups")

	targetKey := fmt.Sprintf("%s/%s", target.Repository.Namespace, target.Repository.Name)
	targetBackups := backupsByTarget[targetKey]

	if len(targetBackups) == 0 {
		// No scheduled backups for this target, skip
		return nil
	}

	// Check if we need to update the target's status
	if !shouldUpdateTargetStatus(target, targetBackups) {
		return nil
	}

	// Calculate backup statistics
	successful, failed, latestCompletion, errorMessages := calculateBackupStatistics(targetBackups)

	// Update target status if we have completion data
	if latestCompletion != nil {
		status, errorMessage := determineTargetStatus(successful, failed, errorMessages)

		if needsStatusUpdate(target, status) {
			if err := updateBackupTargetLastRun(ctx, deps, backupConfig, targetIndex, status, successful, failed, errorMessage); err != nil {
				log.Errorw("Failed to update target status", "target", target.Name, "error", err)
				return err
			}
			log.Infow("Updated backup target status", "target", target.Name, "status", status, "successful", successful, "failed", failed)
		}
	}

	return nil
}

// shouldUpdateTargetStatus checks if a target needs status update based on current state and recent backups
func shouldUpdateTargetStatus(target v1.BackupTarget, targetBackups []*v1.ResticBackup) bool {
	// If target is running, always check for updates
	if target.LastRun != nil && target.LastRun.Status == v1.BackupTargetStatusRunning {
		return true
	}

	// Target is not currently marked as running, check if there are recent backups
	for _, backup := range targetBackups {
		if backup.Status.CompletionTime != nil &&
			time.Since(backup.Status.CompletionTime.Time) < 5*time.Minute {
			return true
		}
	}

	return false
}

// calculateBackupStatistics calculates success/failure counts and error messages for target backups
func calculateBackupStatistics(targetBackups []*v1.ResticBackup) (successful, failed int32, latestCompletion *metav1.Time, errorMessages []string) {
	var hasErrors bool

	for _, backup := range targetBackups {
		if backup.Status.CompletionTime != nil {
			if latestCompletion == nil || backup.Status.CompletionTime.After(latestCompletion.Time) {
				latestCompletion = backup.Status.CompletionTime
			}
		}

		switch backup.Status.Phase {
		case v1.PhaseCompleted:
			successful++
		case v1.PhaseFailed:
			failed++
			if backup.Status.Error != "" {
				hasErrors = true
				errorMessages = append(errorMessages, fmt.Sprintf("PVC %s: %s", backup.Spec.SourcePVC.Name, backup.Status.Error))
			}
		}
	}

	// Only return error messages if there are errors
	if !hasErrors {
		errorMessages = nil
	}

	return successful, failed, latestCompletion, errorMessages
}

// determineTargetStatus determines the final status and error message based on backup statistics
func determineTargetStatus(successful, failed int32, errorMessages []string) (v1.BackupTargetStatus, string) {
	var status v1.BackupTargetStatus
	var errorMessage string

	if failed == 0 {
		status = v1.BackupTargetStatusCompleted
	} else if successful == 0 {
		status = v1.BackupTargetStatusFailed
		errorMessage = "All scheduled backups failed"
	} else {
		status = v1.BackupTargetStatusPartial
		errorMessage = fmt.Sprintf("%d PVCs failed out of %d total", failed, successful+failed)
	}

	if len(errorMessages) > 0 {
		if errorMessage != "" {
			errorMessage += "; "
		}
		errorMessage += strings.Join(errorMessages[:min(3, len(errorMessages))], "; ")
		if len(errorMessages) > 3 {
			errorMessage += fmt.Sprintf(" (and %d more)", len(errorMessages)-3)
		}
	}

	return status, errorMessage
}

// needsStatusUpdate checks if a status update is necessary
func needsStatusUpdate(target v1.BackupTarget, newStatus v1.BackupTargetStatus) bool {
	// Always update if no last run exists
	if target.LastRun == nil {
		return true
	}

	// Update if status has changed
	if target.LastRun.Status != newStatus {
		return true
	}

	// Update if it's been more than a minute since last completion
	if target.LastRun.CompletionTime.IsZero() ||
		time.Since(target.LastRun.CompletionTime.Time) > time.Minute {
		return true
	}

	return false
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
