package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HandleBackupPending handles the logic when a ResticBackup is in the Pending phase.
func HandleBackupPending(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup, stopPods bool) (ctrl.Result, error) {
	log := deps.Logger.Named("backup")
	log.Info("Handling pending backup")

	// 1. Add finalizer
	if err := utils.AddFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		log.Errorw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Stop pods if requested
	if stopPods {
		pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, backup.Namespace, backup.Spec.SourcePVC.Name, log)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, backup, true); err != nil {
			log.Errorw("Failed to scale down workloads", "error", err)
			return ctrl.Result{}, err
		}
	}

	// Get repository
	repo, err := GetRepositoryForBackup(ctx, deps.Client, backup)
	if err != nil {
		log.Errorw("Failed to get repository for backup", "error", err)
		return ctrl.Result{}, err
	}

	// Check repository is ready
	if repo.Status.Phase != v1.PhaseCompleted {
		log.Debug("Repository not ready, requeueing")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Create the backup job
	job, err := createBackupJob(ctx, deps, backup, repo)
	if err != nil {
		log.Errorw("Failed to create backup job", "error", err)
		return ctrl.Result{}, err
	}

	// 5. Update status to Running
	return updateBackupStatus(ctx, deps, backup, v1.PhaseRunning, metav1.Now(), job.Name)
}

// HandleBackupRunning handles the logic when a ResticBackup is in the Running phase.
func HandleBackupRunning(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-running")
	log.Info("Handling running backup")

	// 1. Get the Kubernetes Job
	jobName := backup.Status.Job.JobRef.Name
	job, err := utils.GetResource[batchv1.Job](ctx, deps.Client, backup.Namespace, jobName, log)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Errorw("Backup job not found, assuming failure and moving to Failed phase", "jobName", jobName, "error", err)
			return updateBackupStatus(ctx, deps, backup, v1.PhaseFailed, metav1.Now(), jobName)
		}
		return ctrl.Result{}, err
	}

	// 2. Check job status and update backup status accordingly
	return processBackupJobStatus(ctx, deps, backup, job)
}

// HandleBackupCompleted handles the logic when a ResticBackup is in the Completed phase.
func HandleBackupCompleted(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-completed")
	log.Info("Handling completed backup")

	// Clean up the job since it's completed successfully
	jobName := backup.Status.Job.JobRef.Name
	job, err := utils.GetResource[batchv1.Job](ctx, deps.Client, backup.Spec.SourcePVC.Namespace, jobName, log)
	if err == nil { // Job exists, so delete it
		if err := deps.Client.Delete(ctx, job); err != nil {
			log.Errorw("Failed to clean up completed backup job", "error", err)
			// Don't fail the backup just because we couldn't clean up
		} else {
			log.Info("Successfully cleaned up completed backup job")
		}
	} else if !errors.IsNotFound(err) {
		// Error already logged by GetResource
	}

	// Restore workloads if they were scaled down
	pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, backup.Namespace, backup.Spec.SourcePVC.Name, log)
	if err != nil {
		// Error logged by GetResource, nothing more to do
	} else {
		if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, backup, false); err != nil {
			log.Errorw("Failed to scale up workloads on completion", "error", err)
			// Don't requeue, just log. The backup is complete.
		}
	}

	return ctrl.Result{}, nil
}

// HandleBackupFailed handles the logic when a ResticBackup is in the Failed phase.
func HandleBackupFailed(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-failed")
	log.Info("Handling failed backup")

	// Get the failed job to extract logs
	jobName := backup.Status.Job.JobRef.Name
	job, err := utils.GetResource[batchv1.Job](ctx, deps.Client, backup.Spec.SourcePVC.Namespace, jobName, log)
	if err == nil { // Job exists
		// Attempt to get logs from the failed pod for better error reporting
		podLogs, logErr := utils.GetJobLogs(ctx, deps, job)
		if logErr != nil {
			log.Errorw("Failed to get logs from failed backup job pod", "error", logErr)
		}
		log.Errorw("Backup job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message, "logs", podLogs)
		backup.Status.Error = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", job.Status.Conditions[0].Reason, job.Status.Conditions[0].Message, podLogs)

		// Update status with the log message
		if err := deps.Client.Status().Update(ctx, backup); err != nil {
			log.Errorw("Failed to update backup status with failure logs", "error", err)
		}

		// Clean up the failed job
		if err := deps.Client.Delete(ctx, job); err != nil {
			log.Errorw("Failed to clean up failed backup job", "error", err)
		} else {
			log.Info("Successfully cleaned up failed backup job")
		}
	} else if !errors.IsNotFound(err) {
		// Error already logged by GetResource
	}

	// Restore workloads if they were scaled down
	pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, backup.Namespace, backup.Spec.SourcePVC.Name, log)
	if err != nil {
		// Error logged by GetResource, nothing more to do
	} else {
		if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, backup, false); err != nil {
			log.Errorw("Failed to scale up workloads on failure", "error", err)
			// Don't requeue, just log. The backup has failed.
		}
	}

	return ctrl.Result{}, nil
}

// GetRepositoryForBackup retrieves the ResticRepository for a given ResticBackup.
func GetRepositoryForBackup(ctx context.Context, c client.Client, backup *v1.ResticBackup) (*v1.ResticRepository, error) {
	repo, err := utils.GetResource[v1.ResticRepository](ctx, c, backup.Namespace, backup.Spec.Repository.Name)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// createBackupJob creates a new backup job.
func createBackupJob(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup, repo *v1.ResticRepository) (*batchv1.Job, error) {
	log := deps.Logger.Named("create-backup-job")

	jobSpec, err := utils.BuildBackupJobSpec(backup)
	if err != nil {
		return nil, err
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, backup)
	if err != nil {
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	log.Infow("Backup job created", "jobName", job.Name)
	return job, nil
}

// processBackupJobStatus checks the status of the Kubernetes Job and updates the ResticBackup status.
func processBackupJobStatus(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup, job *batchv1.Job) (ctrl.Result, error) {
	log := deps.Logger.Named("process-backup-job-status")
	finished, succeeded := utils.IsJobFinished(job)

	if !finished {
		log.Debug("Backup job is still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Backup job succeeded. Moving to Completed phase.")
		_, err := updateBackupStatus(ctx, deps, backup, v1.PhaseCompleted, metav1.Now(), job.Name)
		return ctrl.Result{}, err
	}

	// Job failed
	log.Errorw("Backup job failed. Moving to Failed phase.")
	_, err := updateBackupStatus(ctx, deps, backup, v1.PhaseFailed, metav1.Now(), job.Name)
	return ctrl.Result{}, err
}

// updateBackupStatus updates the status of the ResticBackup resource.
func updateBackupStatus(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup, phase v1.Phase, transitionTime metav1.Time, jobName string) (ctrl.Result, error) {
	backup.Status.Phase = phase
	// Message is set in processBackupJobStatus for failures
	if phase != v1.PhaseFailed {
		backup.Status.Error = "" // Clear previous messages
	}

	if jobName != "" {
		backup.Status.Job.JobRef = &corev1.LocalObjectReference{Name: jobName}
	}
	if backup.Status.CompletionTime == nil && (phase == v1.PhaseCompleted || phase == v1.PhaseFailed) {
		backup.Status.CompletionTime = &transitionTime
	}

	if err := deps.Client.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}

	if phase == v1.PhaseFailed {
		return ctrl.Result{RequeueAfter: constants.FailedRequeueInterval}, nil
	}
	if phase == v1.PhaseCompleted {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// getParentBackupConfig retrieves the parent BackupConfig for a ResticBackup
func getParentBackupConfig(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (*v1.BackupConfig, error) {
	backupConfigName := backup.Labels["backup-config"]
	if backupConfigName == "" {
		return nil, nil // No error, just no BackupConfig found
	}

	backupConfig, err := utils.GetResource[v1.BackupConfig](ctx, deps.Client, backup.Namespace, backupConfigName)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent BackupConfig: %w", err)
	}

	return backupConfig, nil
}

// shouldStopPodsForBackup determines if workloads should be scaled down for a given backup.
func shouldStopPodsForBackup(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (bool, error) {
	log := deps.Logger.With("name", backup.Name, "namespace", backup.Namespace)

	backupConfig, err := getParentBackupConfig(ctx, deps, backup)
	if err != nil {
		log.Errorw("Failed to get parent BackupConfig", "error", err)
		// Continue without stopping pods if we can't get the BackupConfig.
		return false, nil
	}
	if backupConfig == nil {
		return false, nil // No config, so no instruction to stop pods.
	}

	pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, backup.Spec.SourcePVC.Namespace, backup.Spec.SourcePVC.Name, log)
	if err != nil {
		// If we can't get the PVC, we can't check selectors, so don't stop pods.
		return false, nil
	}

	matchingSelectors := utils.FindMatchingSelectors(pvc, backupConfig.Spec.Selectors)
	for _, selector := range matchingSelectors {
		if selector.StopPods {
			return true, nil
		}
	}

	return false, nil
}

// HandlePendingPhase handles the pending phase of a backup
func HandlePendingPhase(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.With("name", backup.Name, "namespace", backup.Namespace)

	stopPods, err := shouldStopPodsForBackup(ctx, deps, backup)
	if err != nil {
		// This path is currently not hit as shouldStopPodsForBackup returns nil errors,
		// but it's good practice for future development.
		log.Errorw("Failed to determine if pods should be stopped", "error", err)
		return HandleBackupPending(ctx, deps, backup, false)
	}

	return HandleBackupPending(ctx, deps, backup, stopPods)
}
