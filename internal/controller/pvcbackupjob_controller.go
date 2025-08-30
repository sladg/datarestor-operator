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

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

// PVCBackupJobReconciler reconciles a PVCBackupJob object
type PVCBackupJobReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	resticJob   *ResticJob
	typedClient kubernetes.Interface
}

// NewPVCBackupJobReconciler creates a new PVCBackupJobReconciler
func NewPVCBackupJobReconciler(client client.Client, scheme *runtime.Scheme, typedClient kubernetes.Interface) *PVCBackupJobReconciler {
	return &PVCBackupJobReconciler{
		Client:      client,
		Scheme:      scheme,
		resticJob:   NewResticJob(client, typedClient),
		typedClient: typedClient,
	}
}

// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackupjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackupjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackupjobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *PVCBackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "pvcbackupjob").
		WithValues("name", req.Name, "namespace", req.Namespace)
	logger.Starting("reconcile")

	// Fetch the PVCBackupJob instance
	pvcBackupJob := &storagev1alpha1.PVCBackupJob{}
	err := r.Get(ctx, req.NamespacedName, pvcBackupJob)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PVCBackupJob is being deleted
	if !pvcBackupJob.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, pvcBackupJob)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(pvcBackupJob, "pvcbackupjob.storage.cheap-man-ha-store.com/finalizer") {
		logger.Starting("add finalizer")
		controllerutil.AddFinalizer(pvcBackupJob, "pvcbackupjob.storage.cheap-man-ha-store.com/finalizer")
		if err := r.Update(ctx, pvcBackupJob); err != nil {
			logger.Failed("add finalizer", err)
			return ctrl.Result{}, err
		}
		logger.Completed("add finalizer")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Handle different phases
	switch pvcBackupJob.Status.Phase {
	case "":
		return r.handlePending(ctx, pvcBackupJob)
	case "Pending":
		return r.handlePending(ctx, pvcBackupJob)
	case "Running":
		return r.handleRunning(ctx, pvcBackupJob)
	case "Completed", "Failed":
		// Job is done, no further action needed
		return ctrl.Result{}, nil
	default:
		return r.handlePending(ctx, pvcBackupJob)
	}
}

// handlePending starts the backup job
func (r *PVCBackupJobReconciler) handlePending(ctx context.Context, pvcBackupJob *storagev1alpha1.PVCBackupJob) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "pvcbackupjob").
		WithValues("name", pvcBackupJob.Name, "namespace", pvcBackupJob.Namespace)

	logger.Starting("start backup job")

	// Get the PVC
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, client.ObjectKey{
		Name:      pvcBackupJob.Spec.PVCRef.Name,
		Namespace: pvcBackupJob.Namespace,
	}, &pvc); err != nil {
		logger.Failed("get pvc", err)
		return r.updateStatusWithError(ctx, pvcBackupJob, "Failed", fmt.Sprintf("PVC not found: %v", err))
	}

	// Generate backup ID
	backupID := fmt.Sprintf("backup-%s-%s", pvc.Name, time.Now().Format("20060102-150405"))

	// Create Restic backup job
	job, err := r.resticJob.createResticJob(ctx, pvc, nil, "backup", []string{
		"backup", "/data",
		"--tag", fmt.Sprintf("id=%s", backupID),
		"--tag", fmt.Sprintf("pvc=%s", pvc.Name),
		"--tag", fmt.Sprintf("namespace=%s", pvc.Namespace),
		"--tag", fmt.Sprintf("job=%s", pvcBackupJob.Name),
	})
	if err != nil {
		logger.Failed("create restic job", err)
		return r.updateStatusWithError(ctx, pvcBackupJob, "Failed", fmt.Sprintf("Failed to create backup job: %v", err))
	}

	// Update status
	now := metav1.Now()
	pvcBackupJob.Status.Phase = "Running"
	pvcBackupJob.Status.StartTime = &now
	pvcBackupJob.Status.ResticID = backupID
	pvcBackupJob.Status.Repository = pvcBackupJob.Spec.BackupTarget.Restic.Repository
	pvcBackupJob.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}

	if err := r.Status().Update(ctx, pvcBackupJob); err != nil {
		logger.Failed("update status", err)
		return ctrl.Result{}, err
	}

	logger.Completed("start backup job")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleRunning monitors the running backup job
func (r *PVCBackupJobReconciler) handleRunning(ctx context.Context, pvcBackupJob *storagev1alpha1.PVCBackupJob) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "pvcbackupjob").
		WithValues("name", pvcBackupJob.Name, "namespace", pvcBackupJob.Namespace)

	logger.Starting("check backup job status")

	if pvcBackupJob.Status.JobRef == nil {
		return r.updateStatusWithError(ctx, pvcBackupJob, "Failed", "Missing job reference")
	}

	// Get the Kubernetes job
	var job batchv1.Job
	if err := r.Get(ctx, client.ObjectKey{
		Name:      pvcBackupJob.Status.JobRef.Name,
		Namespace: pvcBackupJob.Namespace,
	}, &job); err != nil {
		logger.Failed("get kubernetes job", err)
		return r.updateStatusWithError(ctx, pvcBackupJob, "Failed", fmt.Sprintf("Job not found: %v", err))
	}

	// Check job status
	if job.Status.Succeeded > 0 {
		// Job completed successfully
		now := metav1.Now()
		pvcBackupJob.Status.Phase = "Completed"
		pvcBackupJob.Status.CompletionTime = &now

		// Try to get backup size from Restic (optional)
		// This could be enhanced to parse logs for size information

		if err := r.Status().Update(ctx, pvcBackupJob); err != nil {
			logger.Failed("update status", err)
			return ctrl.Result{}, err
		}

		logger.Completed("backup job succeeded")
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		// Job failed
		return r.updateStatusWithError(ctx, pvcBackupJob, "Failed", "Backup job failed")
	}

	// Job is still running
	logger.Debug("Backup job still running")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// updateStatusWithError updates the status with error information
func (r *PVCBackupJobReconciler) updateStatusWithError(ctx context.Context, pvcBackupJob *storagev1alpha1.PVCBackupJob, phase, errorMsg string) (ctrl.Result, error) {
	now := metav1.Now()
	pvcBackupJob.Status.Phase = phase
	pvcBackupJob.Status.Error = errorMsg
	if pvcBackupJob.Status.CompletionTime == nil {
		pvcBackupJob.Status.CompletionTime = &now
	}

	if err := r.Status().Update(ctx, pvcBackupJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup when PVCBackupJob is being deleted
func (r *PVCBackupJobReconciler) handleDeletion(ctx context.Context, pvcBackupJob *storagev1alpha1.PVCBackupJob) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "pvcbackupjob").
		WithValues("name", pvcBackupJob.Name, "namespace", pvcBackupJob.Namespace)

	logger.Starting("handle deletion")

	// Clean up the associated Kubernetes job if it exists
	if pvcBackupJob.Status.JobRef != nil {
		var job batchv1.Job
		err := r.Get(ctx, client.ObjectKey{
			Name:      pvcBackupJob.Status.JobRef.Name,
			Namespace: pvcBackupJob.Namespace,
		}, &job)
		if err == nil {
			if err := r.Delete(ctx, &job); err != nil {
				logger.Failed("delete kubernetes job", err)
			} else {
				logger.Debug("Deleted associated Kubernetes job")
			}
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(pvcBackupJob, "pvcbackupjob.storage.cheap-man-ha-store.com/finalizer")
	if err := r.Update(ctx, pvcBackupJob); err != nil {
		logger.Failed("remove finalizer", err)
		return ctrl.Result{}, err
	}

	logger.Completed("handle deletion")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *PVCBackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.PVCBackupJob{}).
		Complete(r)
}
