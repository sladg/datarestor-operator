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
		if controllerutil.ContainsFinalizer(restoreJob, RestoreJobFinalizer) {
			if err := r.handleCleanup(ctx, restoreJob); err != nil {
				logger.Failed("cleanup", err)
				return ctrl.Result{RequeueAfter: time.Minute}, err
			}
			controllerutil.RemoveFinalizer(restoreJob, RestoreJobFinalizer)
			if err := r.Update(ctx, restoreJob); err != nil {
				logger.Failed("remove finalizer", err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(restoreJob, RestoreJobFinalizer) {
		controllerutil.AddFinalizer(restoreJob, RestoreJobFinalizer)
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
		return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
			Phase:             "Failed",
			Error:             fmt.Sprintf("Failed to get target PVC: %v", err),
			SetCompletionTime: true,
		})
	}

	// Handle different restore strategies based on restore type
	if restoreJob.Spec.RestoreType == "manual" {
		// Manual restore: Disable resources using the PVC for data integrity
		logger.Debug("Manual restore: disabling deployments using PVC")
		if err := enableDeploymentsUsingPVC(ctx, r.Client, *pvc, false); err != nil {
			logger.Failed("disable deployments", err)
			return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
				Phase:             "Failed",
				Error:             fmt.Sprintf("Failed to disable resources: %v", err),
				SetCompletionTime: true,
			})
		}
	} else {
		// Automated restore: Add finalizer to PVC to block pod startup
		logger.Debug("Automated restore: adding finalizer to PVC")
		if err := r.addPVCFinalizer(ctx, pvc); err != nil {
			logger.Failed("add PVC finalizer", err)
			return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
				Phase:             "Failed",
				Error:             fmt.Sprintf("Failed to add PVC finalizer: %v", err),
				SetCompletionTime: true,
			})
		}
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
			return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
				Phase:             "Failed",
				Error:             fmt.Sprintf("Failed to get referenced backup job: %v", err),
				SetCompletionTime: true,
			})
		}
		backupID = backupJob.Status.ResticID
	}

	if backupID == "" {
		// Find latest backup - will automatically try BackupJobs first, then fall back to backup target
		result, err := r.resticJob.FindLatestBackup(ctx, *pvc, restoreJob.Spec.BackupTarget)

		if err != nil {
			logger.Failed("find latest backup", err)
			return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
				Phase:             "Failed",
				Error:             fmt.Sprintf("Failed to find backup to restore: %v", err),
				SetCompletionTime: true,
			})
		}

		if result == "" {
			// No backup found - this is expected for new PVCs, complete successfully
			logger.Debug("No backup found for restore request - completing as NotFound")

			if restoreJob.Spec.RestoreType == "automated" {
				// Automated restore for newly created PVC - this is expected and normal
				return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
					Phase:             "Completed",
					RestoreStatus:     "NotFound",
					SetCompletionTime: true,
				})
			} else {
				// Manual restore requested but no backup available - this should fail
				var errorMsg string
				if len([]backupv1alpha1.BackupTarget{restoreJob.Spec.BackupTarget}) > 0 {
					errorMsg = fmt.Sprintf("no backup found for PVC %s/%s in disaster recovery mode - backup may not exist or targets may be incorrect", pvc.Namespace, pvc.Name)
				} else {
					errorMsg = fmt.Sprintf("no backup found for PVC %s/%s - manual restore requested but no backup available", pvc.Namespace, pvc.Name)
				}
				logger.Debug("Manual restore failed - no backup available")
				return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
					Phase:             "Failed",
					Error:             fmt.Sprintf("No backup available for restore: %s", errorMsg),
					RestoreStatus:     "BackupNotFound",
					SetCompletionTime: true,
				})
			}
		}

		backupID = result
		logger.WithValues("backup_id", backupID).Debug("Found backup for restore")
	}

	// Create Kubernetes Job for restore
	job, err := r.createRestoreJob(ctx, restoreJob, *pvc, backupID)
	if err != nil {
		logger.Failed("create restore job", err)
		return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
			Phase:             "Failed",
			Error:             fmt.Sprintf("Failed to create restore job: %v", err),
			SetCompletionTime: true,
		})
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
		return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
			Phase:             "Failed",
			Error:             "No job reference found: missing job reference",
			SetCompletionTime: true,
		})
	}

	// Get the restore job
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      restoreJob.Status.JobRef.Name,
		Namespace: restoreJob.Namespace,
	}, job); err != nil {
		logger.Failed("get job", err)
		return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
			Phase:             "Failed",
			Error:             fmt.Sprintf("Failed to get restore job: %v", err),
			SetCompletionTime: true,
		})
	}

	// Check job status
	if job.Status.Succeeded > 0 {
		// Job completed successfully
		logger.Debug("Restore job completed successfully")

		// Handle post-restore cleanup based on restore type
		if err := r.handleRestoreCompletion(ctx, restoreJob); err != nil {
			logger.Failed("handle restore completion", err)
			return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
				Phase:             "Failed",
				Error:             fmt.Sprintf("Failed to handle restore completion: %v", err),
				SetCompletionTime: true,
			})
		}

		// Update status to completed
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
		// Job failed - handle cleanup based on restore type
		logger.Debug("Restore job failed")

		// Handle post-restore cleanup even on failure
		if err := r.handleRestoreCompletion(ctx, restoreJob); err != nil {
			logger.Failed("handle restore failure cleanup", err)
			// Continue with failure reporting even if cleanup fails
		}

		return UpdateJobStatus(ctx, r.Client, restoreJob, JobStatusOptions{
			Phase:             "Failed",
			Error:             "Restore job failed: kubernetes job failed",
			SetCompletionTime: true,
		})
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

// addPVCFinalizer adds a finalizer to the PVC to block pod startup during automated restore
func (r *RestoreJobReconciler) addPVCFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "pvc-finalizer").
		WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)

	finalizer := PVCRestoreFinalizer

	// Check if finalizer already exists
	for _, f := range pvc.Finalizers {
		if f == finalizer {
			logger.Debug("Finalizer already exists")
			return nil
		}
	}

	logger.Starting("add PVC finalizer")

	// Add the finalizer
	pvc.Finalizers = append(pvc.Finalizers, finalizer)
	if err := r.Update(ctx, pvc); err != nil {
		logger.Failed("add finalizer", err)
		return err
	}

	logger.Completed("add PVC finalizer")
	return nil
}

// removePVCFinalizer removes the restore finalizer from the PVC
func (r *RestoreJobReconciler) removePVCFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "pvc-finalizer").
		WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)

	finalizer := PVCRestoreFinalizer

	logger.Starting("remove PVC finalizer")

	// Remove the finalizer
	var newFinalizers []string
	found := false
	for _, f := range pvc.Finalizers {
		if f != finalizer {
			newFinalizers = append(newFinalizers, f)
		} else {
			found = true
		}
	}

	if !found {
		logger.Debug("Finalizer not found")
		return nil
	}

	pvc.Finalizers = newFinalizers
	if err := r.Update(ctx, pvc); err != nil {
		logger.Failed("remove finalizer", err)
		return err
	}

	logger.Completed("remove PVC finalizer")
	return nil
}

// handleRestoreCompletion handles post-restore cleanup based on restore type
func (r *RestoreJobReconciler) handleRestoreCompletion(ctx context.Context, restoreJob *backupv1alpha1.RestoreJob) error {
	logger := LoggerFrom(ctx, "restore-completion").
		WithValues("name", restoreJob.Name, "type", restoreJob.Spec.RestoreType)

	logger.Starting("handle restore completion")

	// Get the PVC
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      restoreJob.Spec.PVCRef.Name,
		Namespace: restoreJob.Namespace,
	}, pvc); err != nil {
		logger.Failed("get pvc", err)
		return err
	}

	if restoreJob.Spec.RestoreType == "manual" {
		// Manual restore: Enable resources that were disabled
		logger.Debug("Manual restore completion: enabling deployments")
		if err := enableDeploymentsUsingPVC(ctx, r.Client, *pvc, true); err != nil {
			logger.Failed("enable deployments", err)
			return err
		}
	} else {
		// Automated restore: Remove PVC finalizer to allow pod startup
		logger.Debug("Automated restore completion: removing PVC finalizer")
		if err := r.removePVCFinalizer(ctx, pvc); err != nil {
			logger.Failed("remove PVC finalizer", err)
			return err
		}
	}

	logger.Completed("handle restore completion")
	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *RestoreJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.RestoreJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
