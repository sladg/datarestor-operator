package logic

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
