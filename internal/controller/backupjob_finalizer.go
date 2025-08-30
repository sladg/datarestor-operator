package controller

import (
	"context"
	"fmt"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// handleBackupJobDeletion handles cleanup when BackupJob is being deleted
func (r *BackupJobReconciler) handleBackupJobDeletion(ctx context.Context, backupJob *backupv1alpha1.BackupJob) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "backupjob-deletion").
		WithValues("name", backupJob.Name, "namespace", backupJob.Namespace)

	logger.Starting("deletion")

	// Clean up the restic backup data if this backup was completed
	if backupJob.Status.Phase == "Completed" && backupJob.Status.ResticID != "" {
		if err := r.cleanupResticBackup(ctx, backupJob, logger); err != nil {
			logger.Failed("cleanup restic backup", err)
			// Don't fail deletion due to restic cleanup issues
		}
	}

	// Clean up the associated Job if it exists
	if err := r.cleanupJob(ctx, backupJob, logger); err != nil {
		logger.Failed("cleanup job", err)
		// Continue with finalizer removal
	}

	// Remove finalizer
	logger.Debug("Removing finalizer")
	controllerutil.RemoveFinalizer(backupJob, BackupJobFinalizer)
	if err := r.Update(ctx, backupJob); err != nil {
		logger.Failed("remove finalizer", err)
		return ctrl.Result{}, err
	}

	logger.Completed("deletion")
	return ctrl.Result{}, nil
}

// cleanupResticBackup removes the specific backup from restic repository
func (r *BackupJobReconciler) cleanupResticBackup(ctx context.Context, backupJob *backupv1alpha1.BackupJob, logger *Logger) error {
	logger.Starting("cleanup restic backup")

	// Get the PVC for this backup job
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, client.ObjectKey{
		Name:      backupJob.Spec.PVCRef.Name,
		Namespace: backupJob.Namespace,
	}, &pvc); err != nil {
		logger.Failed("get pvc for restic cleanup", err)
		return err
	}

	// Create ResticJob instance for cleanup operations
	resticJob := &ResticJob{
		client:      r.Client,
		typedClient: r.typedClient,
	}

	// Build restic forget command for this specific backup
	args := []string{
		"forget", backupJob.Status.ResticID,
		"--prune", // Also prune to actually remove the data
	}

	logger.WithValues("restic_id", backupJob.Status.ResticID).Debug("Forgetting restic backup")

	// Create temporary BackupConfig with just the target we need
	tempBackupConfig := &backupv1alpha1.BackupConfig{
		Spec: backupv1alpha1.BackupConfigSpec{
			BackupTargets: []backupv1alpha1.BackupTarget{backupJob.Spec.BackupTarget},
		},
	}

	// Create and run the cleanup job
	job, err := resticJob.createResticJob(ctx, pvc, tempBackupConfig, "forget", args)
	if err != nil {
		return fmt.Errorf("failed to create forget job: %w", err)
	}

	// Ensure job cleanup
	defer func() {
		if deleteErr := resticJob.client.Delete(ctx, job); deleteErr != nil {
			logger.WithValues("job", job.Name).Failed("cleanup forget job", deleteErr)
		}
	}()

	// Wait for job completion with timeout
	if err := resticJob.waitForJobCompletion(ctx, job); err != nil {
		return fmt.Errorf("forget job failed: %w", err)
	}

	logger.Completed("cleanup restic backup")
	return nil
}

// cleanupJob cleans up the associated Job
func (r *BackupJobReconciler) cleanupJob(ctx context.Context, backupJob *backupv1alpha1.BackupJob, logger *Logger) error {
	logger.Starting("cleanup job")

	if backupJob.Status.JobRef == nil {
		logger.Debug("No Job reference found")
		return nil
	}

	var job batchv1.Job
	err := r.Get(ctx, client.ObjectKey{
		Name:      backupJob.Status.JobRef.Name,
		Namespace: backupJob.Namespace,
	}, &job)

	if err == nil {
		logger.WithValues("job", job.Name).Debug("Deleting Job")
		if err := r.Delete(ctx, &job); err != nil {
			logger.Failed("delete job", err)
			return err
		} else {
			logger.Debug("Job deleted")
		}
	} else {
		// Job might already be deleted, which is fine
		logger.WithValues("error", err).Debug("Job not found (likely already deleted)")
	}

	logger.Completed("cleanup job")
	return nil
}
