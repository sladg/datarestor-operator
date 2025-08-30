package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

// NewPVCBackupReconciler creates a new PVCBackupReconciler
func NewPVCBackupReconciler(client client.Client, scheme *runtime.Scheme, config *rest.Config) (*PVCBackupReconciler, error) {
	typedClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &PVCBackupReconciler{
		Client:      client,
		Scheme:      scheme,
		typedClient: typedClient,
		cronParser:  cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}, nil
}

// PVCBackupReconciler reconciles a PVCBackup object
type PVCBackupReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	typedClient kubernetes.Interface
	cronParser  cron.Parser
}

// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PVCBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "controller").
		WithValues("name", req.Name, "namespace", req.Namespace)
	logger.Starting("reconcile")

	// Fetch the PVCBackup instance
	pvcBackup := &storagev1alpha1.PVCBackup{}
	err := r.Get(ctx, req.NamespacedName, pvcBackup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request with backoff.
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PVCBackup is being deleted
	if !pvcBackup.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, pvcBackup)
	}

	// Add finalizer if not present
	if !containsFinalizer(pvcBackup, "pvcbackup.storage.cheap-man-ha-store.com/finalizer") {
		logger.Starting("add finalizer")
		pvcBackup.Finalizers = append(pvcBackup.Finalizers, "pvcbackup.storage.cheap-man-ha-store.com/finalizer")
		if err := r.Update(ctx, pvcBackup); err != nil {
			logger.Failed("add finalizer", err)
			return ctrl.Result{}, err
		}
		logger.Completed("add finalizer")
		// Return early to allow the update to be processed
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Find PVCs that match the selector
	matchedPVCs, err := r.findMatchingPVCs(ctx, pvcBackup)
	if err != nil {
		logger.Failed("find matching PVCs", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Log the number of matched PVCs for debugging
	logger.WithValues(
		"count", len(matchedPVCs),
		"namespaces", pvcBackup.Spec.PVCSelector.Namespaces,
	).Debug("Found matching PVCs")

	// Update status with managed PVCs
	logger.Starting("update managed PVCs status")
	if err := r.updateManagedPVCsStatus(ctx, pvcBackup, matchedPVCs); err != nil {
		logger.Failed("update managed PVCs status", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	logger.Completed("update managed PVCs status")

	// Handle scheduling only - create job resources as needed for each PVC individually
	if len(matchedPVCs) > 0 {
		logger.WithValues("pvcCount", len(matchedPVCs)).Debug("Processing schedule")

		// Check for manual backup trigger
		if val, ok := pvcBackup.Annotations["backup.cheap-man-ha-store.com/manual-trigger"]; ok && val == "now" {
			logger.Starting("schedule manual backup")
			successCount := 0
			for _, pvc := range matchedPVCs {
				if r.shouldCreateBackupJob(ctx, pvcBackup, pvc, "manual") {
					if err := r.createPVCBackupJob(ctx, pvcBackup, pvc, "manual"); err != nil {
						logger.WithPVC(pvc).Failed("create manual backup job", err)
					} else {
						successCount++
						logger.WithPVC(pvc).Debug("Created manual backup job")
					}
				}
			}
			// Remove the annotation after scheduling
			delete(pvcBackup.Annotations, "backup.cheap-man-ha-store.com/manual-trigger")
			if err := r.Update(ctx, pvcBackup); err != nil {
				logger.Failed("remove manual-trigger annotation", err)
			}
			logger.WithValues("created", successCount).Completed("schedule manual backup")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// Check if it's time for scheduled backup
		if r.shouldPerformBackup(pvcBackup) {
			logger.Starting("schedule backup")
			successCount := 0
			for _, pvc := range matchedPVCs {
				if r.shouldCreateBackupJob(ctx, pvcBackup, pvc, "scheduled") {
					if err := r.createPVCBackupJob(ctx, pvcBackup, pvc, "scheduled"); err != nil {
						logger.WithPVC(pvc).Failed("create scheduled backup job", err)
					} else {
						successCount++
						logger.WithPVC(pvc).Debug("Created scheduled backup job")
					}
				}
			}
			if successCount > 0 {
				r.updateBackupStatus(ctx, pvcBackup, successCount)
			}
			logger.WithValues("created", successCount).Completed("schedule backup")
		}

		// Handle restore scheduling for new PVCs
		if pvcBackup.Spec.AutoRestore {
			logger.Starting("schedule auto restore")
			successCount := 0
			for _, pvc := range matchedPVCs {
				if r.needsAutoRestore(ctx, pvc) {
					if err := r.createPVCBackupRestoreJob(ctx, pvcBackup, pvc); err != nil {
						logger.WithPVC(pvc).Failed("create auto restore job", err)
					} else {
						successCount++
						logger.WithPVC(pvc).Debug("Created auto restore job")
					}
				}
			}
			logger.WithValues("created", successCount).Completed("schedule auto restore")
		}
	} else {
		logger.WithValues("namespaces", pvcBackup.Spec.PVCSelector.Namespaces).
			Debug("No matching PVCs found")
	}

	// Schedule next reconciliation based on backup schedule
	logger.Starting("calculate next reconcile")
	nextReconcile, err := r.calculateNextReconcile(pvcBackup)
	if err != nil {
		logger.Failed("calculate next reconcile", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	logger.WithValues("requeueAfter", nextReconcile).Debug("Next reconciliation scheduled")
	return ctrl.Result{RequeueAfter: nextReconcile}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PVCBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create field index for finding pods by PVC claim name
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.volumes.persistentVolumeClaim.claimName", func(obj client.Object) []string {
		pod := obj.(*corev1.Pod)
		var pvcNames []string
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
			}
		}
		return pvcNames
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.PVCBackup{}).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForPVC),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForPod),
		).
		Complete(r)
}

// createPVCBackupJob creates a new PVCBackupJob resource
func (r *PVCBackupReconciler) createPVCBackupJob(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim, backupType string) error {
	if len(pvcBackup.Spec.BackupTargets) == 0 {
		return fmt.Errorf("no backup targets configured")
	}

	jobName := fmt.Sprintf("%s-%s-%s", pvcBackup.Name, pvc.Name, time.Now().Format("20060102-150405"))
	target := pvcBackup.Spec.BackupTargets[0] // Use first target

	pvcBackupJob := &storagev1alpha1.PVCBackupJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				"pvcbackup.cheap-man-ha-store.com/pvcbackup": pvcBackup.Name,
				"pvcbackup.cheap-man-ha-store.com/pvc":       pvc.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: pvcBackup.APIVersion,
					Kind:       pvcBackup.Kind,
					Name:       pvcBackup.Name,
					UID:        pvcBackup.UID,
				},
			},
		},
		Spec: storagev1alpha1.PVCBackupJobSpec{
			PVCRef: corev1.LocalObjectReference{
				Name: pvc.Name,
			},
			BackupTarget: target,
			PVCBackupRef: corev1.LocalObjectReference{
				Name: pvcBackup.Name,
			},
			BackupType: backupType,
		},
	}

	return r.Create(ctx, pvcBackupJob)
}

// createPVCBackupRestoreJob creates a new PVCBackupRestoreJob resource
func (r *PVCBackupReconciler) createPVCBackupRestoreJob(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) error {
	if len(pvcBackup.Spec.BackupTargets) == 0 {
		return fmt.Errorf("no backup targets configured")
	}

	jobName := fmt.Sprintf("restore-%s-%s-%s", pvcBackup.Name, pvc.Name, time.Now().Format("20060102-150405"))
	target := pvcBackup.Spec.BackupTargets[0] // Use first target

	restoreJob := &storagev1alpha1.PVCBackupRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				"pvcbackup.cheap-man-ha-store.com/pvcbackup": pvcBackup.Name,
				"pvcbackup.cheap-man-ha-store.com/pvc":       pvc.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: pvcBackup.APIVersion,
					Kind:       pvcBackup.Kind,
					Name:       pvcBackup.Name,
					UID:        pvcBackup.UID,
				},
			},
		},
		Spec: storagev1alpha1.PVCBackupRestoreJobSpec{
			PVCRef: corev1.LocalObjectReference{
				Name: pvc.Name,
			},
			BackupTarget: target,
			PVCBackupRef: corev1.LocalObjectReference{
				Name: pvcBackup.Name,
			},
			RestoreType: "automated",
		},
	}

	return r.Create(ctx, restoreJob)
}

// needsAutoRestore checks if a PVC needs automatic restoration
func (r *PVCBackupReconciler) needsAutoRestore(ctx context.Context, pvc corev1.PersistentVolumeClaim) bool {
	// Check if PVC has the restore annotation
	if val, ok := pvc.Annotations["restore.cheap-man-ha-store.com/needed"]; ok && val == "true" {
		return true
	}

	// For auto-restore, we should only create restore jobs if we know backups exist
	// This prevents unnecessary restore job creation for new PVCs
	logger := LoggerFrom(ctx, "auto-restore").WithPVC(pvc)

	// Quick check if any PVCBackupJobs exist for this PVC
	var pvcBackupJobs storagev1alpha1.PVCBackupJobList
	if err := r.List(ctx, &pvcBackupJobs,
		client.InNamespace(pvc.Namespace),
		client.MatchingLabels{
			"pvcbackup.cheap-man-ha-store.com/pvc": pvc.Name,
		}); err != nil {
		logger.WithValues("error", err).Debug("Failed to check for existing backup jobs")
		return false
	}

	// Only auto-restore if there are completed backup jobs
	for _, job := range pvcBackupJobs.Items {
		if job.Status.Phase == "Completed" && job.Status.ResticID != "" {
			logger.Debug("Found existing backup, PVC needs auto-restore")
			return true
		}
	}

	logger.Debug("No completed backups found, skipping auto-restore")
	return false
}

// shouldCreateBackupJob checks if a backup job should be created for a specific PVC
func (r *PVCBackupReconciler) shouldCreateBackupJob(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim, backupType string) bool {
	logger := LoggerFrom(ctx, "scheduler").
		WithPVC(pvc).
		WithValues("type", backupType)

	// Check existing backup jobs for this PVC
	var pvcBackupJobs storagev1alpha1.PVCBackupJobList
	if err := r.List(ctx, &pvcBackupJobs,
		client.InNamespace(pvc.Namespace),
		client.MatchingLabels{
			"pvcbackup.cheap-man-ha-store.com/pvcbackup": pvcBackup.Name,
			"pvcbackup.cheap-man-ha-store.com/pvc":       pvc.Name,
		}); err != nil {
		logger.WithValues("error", err).Debug("Failed to check existing backup jobs")
		return true // Default to creating job if check fails
	}

	// Count only pending jobs - we don't care about running jobs for scheduling decisions
	var pendingJobs []storagev1alpha1.PVCBackupJob

	for _, job := range pvcBackupJobs.Items {
		switch job.Status.Phase {
		case "", "Pending":
			pendingJobs = append(pendingJobs, job)
		case "Running", "Completed", "Failed":
			// Don't count these for scheduling decisions
		default:
			// Unknown state, treat as pending to be safe
			pendingJobs = append(pendingJobs, job)
		}
	}

	logger.WithValues("pending_jobs", len(pendingJobs)).Debug("Current pending job count")

	// Limit pending jobs to prevent queue buildup - allow maximum 1 pending job
	if len(pendingJobs) >= 1 {
		logger.WithValues("pending", len(pendingJobs)).Debug("Maximum pending job limit reached, skipping backup to prevent queue buildup")
		return false
	}

	// For manual backups, allow if no pending jobs
	if backupType == "manual" {
		logger.Debug("Creating manual backup job - no pending jobs")
		return true
	}

	// Scheduled backup can be created - no pending jobs to cause queue buildup
	// Note: Running job concurrency should be limited by the PVCBackupJob controller
	logger.Debug("Creating scheduled backup job - no pending jobs")
	return true
}

// updateBackupStatus updates the PVCBackup status after creating backup jobs
func (r *PVCBackupReconciler) updateBackupStatus(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, successCount int) {
	logger := LoggerFrom(ctx, "scheduler").
		WithValues("name", pvcBackup.Name)

	now := time.Now()
	pvcBackup.Status.LastBackup = &metav1.Time{Time: now}

	// Calculate next backup time
	if pvcBackup.Spec.Schedule.Cron != "" {
		schedule, err := r.cronParser.Parse(pvcBackup.Spec.Schedule.Cron)
		if err == nil {
			nextBackup := schedule.Next(now)
			pvcBackup.Status.NextBackup = &metav1.Time{Time: nextBackup}
		}
	}

	pvcBackup.Status.SuccessfulBackups += int32(successCount)

	if err := r.Status().Update(ctx, pvcBackup); err != nil {
		logger.Failed("update status", err)
	}
}
