package controller

import (
	"context"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// cleanupBackupConfigResources cleans up resources created by the BackupConfig
func (r *BackupConfigReconciler) cleanupBackupConfigResources(ctx context.Context, pvcBackup *backupv1alpha1.BackupConfig) error {
	logger := LoggerFrom(ctx, "cleanup").
		WithValues("name", pvcBackup.Name)

	// Clean up backup pods created by this BackupConfig
	var pods corev1.PodList
	logger.Starting("cleanup pods")
	if err := r.List(ctx, &pods, client.MatchingLabels(map[string]string{
		"backupconfig.autorestore-backup-operator.com/created-by": pvcBackup.Name,
	})); err != nil {
		logger.Failed("list pods", err)
	} else {
		for _, pod := range pods.Items {
			podLogger := logger.WithValues("pod", pod.Name)
			podLogger.Debug("Deleting pod")
			if err := r.Delete(ctx, &pod); err != nil {
				podLogger.Failed("delete pod", err)
			} else {
				podLogger.Debug("Pod deleted")
			}
		}
	}
	logger.Completed("cleanup pods")

	return nil
}

// handleBackupConfigDeletion handles cleanup when BackupConfig is being deleted
func (r *BackupConfigReconciler) handleBackupConfigDeletion(ctx context.Context, pvcBackup *backupv1alpha1.BackupConfig) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "deletion").
		WithValues("name", pvcBackup.Name)

	logger.Starting("deletion")

	// Clean up any resources created by this BackupConfig
	if err := r.cleanupBackupConfigResources(ctx, pvcBackup); err != nil {
		logger.Failed("cleanup resources", err)
		// Don't return error to avoid blocking deletion
	}

	// Remove finalizer
	logger.Debug("Removing finalizer")
	controllerutil.RemoveFinalizer(pvcBackup, BackupConfigFinalizer)
	if err := r.Update(ctx, pvcBackup); err != nil {
		logger.Failed("remove finalizer", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Completed("deletion")
	return ctrl.Result{}, nil
}

// shouldPerformBackup checks if it's time to perform a backup based on schedule
func (r *BackupConfigReconciler) shouldPerformBackup(pvcBackup *backupv1alpha1.BackupConfig) bool {
	// If no cron schedule, only perform manual backups
	if pvcBackup.Spec.Schedule.Cron == "" {
		return false
	}

	// Parse the cron schedule
	schedule, err := r.cronParser.Parse(pvcBackup.Spec.Schedule.Cron)
	if err != nil {
		return false
	}

	now := time.Now()

	// If there's no last backup time, perform backup now
	if pvcBackup.Status.LastBackup == nil {
		return true
	}

	// Check if it's time for the next backup
	nextBackup := schedule.Next(pvcBackup.Status.LastBackup.Time)
	return now.After(nextBackup) || now.Equal(nextBackup)
}
