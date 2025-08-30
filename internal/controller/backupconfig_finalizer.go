package controller

import (
	"context"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// cleanupBackupConfigResources cleans up resources created by the BackupConfig
func (r *BackupConfigReconciler) cleanupBackupConfigResources(ctx context.Context, backupConfig *backupv1alpha1.BackupConfig) error {
	// Note: BackupJobs and RestoreJobs are automatically cleaned up by Kubernetes
	// due to owner references, and their finalizers handle their own cleanup
	//
	// Kubernetes Jobs cleanup is handled by individual BackupJob/RestoreJob finalizers
	// Restic cleanup is also handled by individual BackupJob finalizers
	// This ensures each backup and its associated resources are properly cleaned up

	return nil
}

// handleBackupConfigDeletion handles cleanup when BackupConfig is being deleted
func (r *BackupConfigReconciler) handleBackupConfigDeletion(ctx context.Context, backupConfig *backupv1alpha1.BackupConfig) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "deletion").
		WithValues("name", backupConfig.Name)

	logger.Starting("deletion")

	// Clean up any resources created by this BackupConfig
	if err := r.cleanupBackupConfigResources(ctx, backupConfig); err != nil {
		logger.Failed("cleanup resources", err)
		// Don't return error to avoid blocking deletion
	}

	// Remove finalizer
	logger.Debug("Removing finalizer")
	controllerutil.RemoveFinalizer(backupConfig, BackupConfigFinalizer)
	if err := r.Update(ctx, backupConfig); err != nil {
		logger.Failed("remove finalizer", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Completed("deletion")
	return ctrl.Result{}, nil
}
