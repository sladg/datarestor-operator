/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
)

// RestoreJobReconciler reconciles a RestoreJob object
type RestoreJobReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	resticJob   *ResticJob
	typedClient kubernetes.Interface
}

// NewRestoreJobReconciler creates a new RestoreJobReconciler
func NewRestoreJobReconciler(client client.Client, scheme *runtime.Scheme, typedClient kubernetes.Interface) *RestoreJobReconciler {
	return &RestoreJobReconciler{
		Client:      client,
		Scheme:      scheme,
		resticJob:   NewResticJob(client, typedClient),
		typedClient: typedClient,
	}
}

// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=restorejobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=restorejobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=restorejobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RestoreJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "restore-job").
		WithValues("name", req.Name, "namespace", req.Namespace)

	logger.Starting("reconcile")

	// Fetch the RestoreJob instance
	restoreJob := &backupv1alpha1.RestoreJob{}
	if err := r.Get(ctx, req.NamespacedName, restoreJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("RestoreJob not found")
			return ctrl.Result{}, nil
		}
		logger.Failed("get restore job", err)
		return ctrl.Result{}, err
	}

	// Handle finalizer
	if restoreJob.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(restoreJob, "restorejob.backup.autorestore-backup-operator.com/finalizer") {
			if err := r.handleCleanup(ctx, restoreJob); err != nil {
				logger.Failed("cleanup", err)
				return ctrl.Result{RequeueAfter: time.Minute}, err
			}
			controllerutil.RemoveFinalizer(restoreJob, "restorejob.backup.autorestore-backup-operator.com/finalizer")
			if err := r.Update(ctx, restoreJob); err != nil {
				logger.Failed("remove finalizer", err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(restoreJob, "restorejob.backup.autorestore-backup-operator.com/finalizer") {
		controllerutil.AddFinalizer(restoreJob, "restorejob.backup.autorestore-backup-operator.com/finalizer")
		if err := r.Update(ctx, restoreJob); err != nil {
			logger.Failed("add finalizer", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle the restore job based on its current phase
	switch restoreJob.Status.Phase {
	case "", "Pending":
		return r.handlePendingRestore(ctx, restoreJob)
	case "Running":
		return r.handleRunningRestore(ctx, restoreJob)
	case "Completed", "Failed":
		// Nothing to do for terminal states
		return ctrl.Result{}, nil
	default:
		logger.WithValues("phase", restoreJob.Status.Phase).Debug("Unknown phase")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
}

// handlePendingRestore starts the restore process
func (r *RestoreJobReconciler) handlePendingRestore(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "restore-job").
		WithValues("name", restoreJob.Name)

	logger.Starting("start restore")

	// Get the PVC to restore into
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      restoreJob.Spec.PVCRef.Name,
		Namespace: restoreJob.Namespace,
	}, pvc); err != nil {
		logger.Failed("get pvc", err)
		return r.updateStatusWithError(ctx, restoreJob, "Failed to get target PVC", err)
	}

	// Determine backup ID to restore
	backupID := restoreJob.Spec.BackupID
	if backupID == "" && restoreJob.Spec.BackupJobRef != nil {
		// Get backup ID from referenced BackupJob
		backupJob := &backupv1alpha1.BackupJob{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      restoreJob.Spec.BackupJobRef.Name,
			Namespace: restoreJob.Namespace,
		}, backupJob); err != nil {
			logger.Failed("get backup job", err)
			return r.updateStatusWithError(ctx, restoreJob, "Failed to get referenced backup job", err)
		}
		backupID = backupJob.Status.ResticID
	}

	if backupID == "" {
		// Find latest backup automatically using disaster recovery approach
		var err error

		// Try to find a BackupConfig resource for this restore job
		var pvcBackup *backupv1alpha1.BackupConfig
		if restoreJob.Spec.BackupConfigRef.Name != "" {
			pvcBackup = &backupv1alpha1.BackupConfig{}
			if getErr := r.Get(ctx, client.ObjectKey{
				Name:      restoreJob.Spec.BackupConfigRef.Name,
				Namespace: restoreJob.Namespace,
			}, pvcBackup); getErr != nil {
				logger.WithValues("error", getErr).Debug("BackupConfig not found, using disaster recovery mode")
				pvcBackup = nil
			}
		}

		if pvcBackup != nil {
			// Normal mode - use existing BackupConfig
			backupID, err = r.resticJob.FindLatestBackup(ctx, pvcBackup, *pvc)
		} else {
			// Disaster recovery mode - use backup target from restore job spec
			backupID, err = r.resticJob.FindLatestBackupForDisasterRecovery(ctx, *pvc, []backupv1alpha1.BackupTarget{restoreJob.Spec.BackupTarget})
		}

		if err != nil {
			logger.Failed("find latest backup", err)
			return r.updateStatusWithError(ctx, restoreJob, "Failed to find backup to restore", err)
		}

		if backupID == "" {
			// No backup found for restore job
			logger.Debug("No backup found for explicit restore request")
			var errorMsg string
			if restoreJob.Spec.RestoreType == "automated" {
				errorMsg = fmt.Sprintf("no backup found for PVC %s/%s - this is normal for newly created PVCs", pvc.Namespace, pvc.Name)
			} else {
				errorMsg = fmt.Sprintf("no backup found for PVC %s/%s - manual restore requested but no backup available", pvc.Namespace, pvc.Name)
			}
			return r.updateStatusWithErrorAndRestoreStatus(ctx, restoreJob, "No backup available for restore", fmt.Errorf("%s", errorMsg), "NotFound")
		}

		logger.WithValues(
			"backup_id", backupID,
			"mode", func() string {
				if pvcBackup != nil {
					return "normal"
				}
				return "disaster-recovery"
			}(),
		).Debug("Found backup for restore")
	}

	// Create Kubernetes Job for restore
	job, err := r.createRestoreJob(ctx, restoreJob, *pvc, backupID)
	if err != nil {
		logger.Failed("create restore job", err)
		return r.updateStatusWithError(ctx, restoreJob, "Failed to create restore job", err)
	}

	// Update status to Running and set restore status based on type
	restoreJob.Status.Phase = "Running"
	now := metav1.Now()
	restoreJob.Status.StartTime = &now
	restoreJob.Status.RestoredBackupID = backupID
	restoreJob.Status.Repository = restoreJob.Spec.BackupTarget.Restic.Repository
	restoreJob.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}

	// Set restore status based on restore type
	switch restoreJob.Spec.RestoreType {
	case "manual":
		restoreJob.Status.RestoreStatus = "Manual"
	case "automated":
		restoreJob.Status.RestoreStatus = "Automated"
	default:
		restoreJob.Status.RestoreStatus = "Manual" // Default fallback
	}

	if err := r.Status().Update(ctx, restoreJob); err != nil {
		logger.Failed("update status", err)
		return ctrl.Result{}, err
	}

	logger.Completed("start restore")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleRunningRestore monitors the running restore job
func (r *RestoreJobReconciler) handleRunningRestore(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "restore-job").
		WithValues("name", restoreJob.Name)

	if restoreJob.Status.JobRef == nil {
		return r.updateStatusWithError(ctx, restoreJob, "No job reference found", fmt.Errorf("missing job reference"))
	}

	// Get the restore job
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      restoreJob.Status.JobRef.Name,
		Namespace: restoreJob.Namespace,
	}, job); err != nil {
		logger.Failed("get job", err)
		return r.updateStatusWithError(ctx, restoreJob, "Failed to get restore job", err)
	}

	// Check job status
	if job.Status.Succeeded > 0 {
		// Job completed successfully
		now := metav1.Now()
		restoreJob.Status.Phase = "Completed"
		restoreJob.Status.CompletionTime = &now
		restoreJob.Status.Error = ""

		// TODO: Parse job logs to get data restored size
		restoreJob.Status.DataRestored = 0 // Placeholder

		if err := r.Status().Update(ctx, restoreJob); err != nil {
			logger.Failed("update status", err)
			return ctrl.Result{}, err
		}

		logger.Completed("restore")
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		// Job failed
		return r.updateStatusWithError(ctx, restoreJob, "Restore job failed", fmt.Errorf("kubernetes job failed"))
	}

	// Job still running
	logger.Debug("Restore job still running")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// createRestoreJob creates a Kubernetes Job to perform the restore
func (r *RestoreJobReconciler) createRestoreJob(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob, pvc corev1.PersistentVolumeClaim, backupID string) (*batchv1.Job, error) {
	// Build restic restore command
	args := []string{
		"restore", backupID,
		"--target", "/data",
	}

	// Use the shared job creation logic
	return r.resticJob.createResticJob(ctx, pvc, nil, "restore", args)
}

// updateStatusWithError updates the status with error information
func (r *RestoreJobReconciler) updateStatusWithError(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob, message string, err error) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "restore-job").
		WithValues("name", restoreJob.Name)

	now := metav1.Now()
	restoreJob.Status.Phase = "Failed"
	restoreJob.Status.CompletionTime = &now
	restoreJob.Status.Error = fmt.Sprintf("%s: %v", message, err)

	if updateErr := r.Status().Update(ctx, restoreJob); updateErr != nil {
		logger.Failed("update error status", updateErr)
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{}, err
}

// updateStatusWithErrorAndRestoreStatus updates the status with error information and restore status
func (r *RestoreJobReconciler) updateStatusWithErrorAndRestoreStatus(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob, message string, err error, restoreStatus string) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "restore-job").
		WithValues("name", restoreJob.Name)

	now := metav1.Now()
	restoreJob.Status.Phase = "Failed"
	restoreJob.Status.RestoreStatus = restoreStatus
	restoreJob.Status.CompletionTime = &now
	restoreJob.Status.Error = fmt.Sprintf("%s: %v", message, err)

	if updateErr := r.Status().Update(ctx, restoreJob); updateErr != nil {
		logger.Failed("update error status", updateErr)
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{}, err
}

// handleCleanup cleans up resources when the restore job is deleted
func (r *RestoreJobReconciler) handleCleanup(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob) error {
	logger := LoggerFrom(ctx, "restore-job").
		WithValues("name", restoreJob.Name)

	logger.Starting("cleanup")

	// Clean up associated Kubernetes Job if it exists
	if restoreJob.Status.JobRef != nil {
		job := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKey{
			Name:      restoreJob.Status.JobRef.Name,
			Namespace: restoreJob.Namespace,
		}, job)
		if err == nil {
			if err := r.Delete(ctx, job); err != nil && !errors.IsNotFound(err) {
				logger.Failed("delete job", err)
				return err
			}
		} else if !errors.IsNotFound(err) {
			logger.Failed("get job for cleanup", err)
			return err
		}
	}

	logger.Completed("cleanup")
	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *RestoreJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.RestoreJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
