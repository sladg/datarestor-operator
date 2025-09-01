package logic

import (
	"context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleResticRestoreDeletion handles the deletion logic for a ResticRestore.
func HandleResticRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Starting deletion logic for ResticRestore")

	if controllerutil.ContainsFinalizer(restore, constants.ResticRestoreFinalizer) {
		// Our finalizer doesn't do anything specific on deletion,
		// as the restore job is owned by the CR and will be garbage collected.
		// Workload scaling is handled by the workload finalizer, not this one.

		// Remove the finalizer
		log.Debug("Removing finalizer")
		controllerutil.RemoveFinalizer(restore, constants.ResticRestoreFinalizer)
		if err := deps.Client.Update(ctx, restore); err != nil {
			log.Errorw("Failed to remove finalizer", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	log.Info("Completed deletion logic for ResticRestore")
	return ctrl.Result{}, nil
}

// HandleRestorePending handles the logic when a ResticRestore is in the Pending phase.
func HandleRestorePending(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Handling pending restore")

	// 1. Add finalizer
	if !controllerutil.ContainsFinalizer(restore, constants.ResticRestoreFinalizer) {
		controllerutil.AddFinalizer(restore, constants.ResticRestoreFinalizer)
		if err := deps.Client.Update(ctx, restore); err != nil {
			log.Errorw("Failed to add finalizer", "error", err)
			return ctrl.Result{}, err
		}
	}

	// 2. Add workload finalizers if this is an auto-restore
	if restore.Labels[constants.LabelRestoreType] == "auto" {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := deps.Client.Get(ctx, types.NamespacedName{Name: restore.Spec.TargetPVC, Namespace: restore.Namespace}, pvc); err != nil {
			log.Errorw("Failed to get PVC for adding workload finalizers", "pvc", restore.Spec.TargetPVC, "error", err)
			return ctrl.Result{}, err
		}
		if err := addWorkloadFinalizers(ctx, deps, pvc); err != nil {
			log.Errorw("Failed to add workload finalizers", "error", err)
			return ctrl.Result{}, err
		}
	}

	// 3. Get repository
	repo, err := getRepositoryForRestore(ctx, deps, restore)
	if err != nil {
		log.Errorw("Failed to get repository for restore", "error", err)
		return ctrl.Result{}, err
	}

	// Check if repository is ready
	if !repo.Status.Initialized {
		log.Debug("Repository not ready, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 4. Create the restore job
	job, err := createRestoreJob(ctx, deps, restore, repo)
	if err != nil {
		log.Errorw("Failed to create restore job", "error", err)
		return ctrl.Result{}, err
	}

	// 5. Update status to Running
	return updateResticRestoreStatus(ctx, deps, restore, backupv1alpha1.PhaseRunning, metav1.Now(), job.Name)
}

// HandleRestoreRunning handles the logic when a ResticRestore is in the Running phase.
func HandleRestoreRunning(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Handling running restore")

	// 1. Get the Kubernetes Job
	job := &batchv1.Job{}
	if err := deps.Client.Get(ctx, types.NamespacedName{Name: restore.Status.Job.JobRef.Name, Namespace: restore.Namespace}, job); err != nil {
		if errors.IsNotFound(err) {
			log.Errorw("Restore job not found, assuming failure and moving to Failed phase", "jobName", restore.Status.Job.JobRef.Name, "error", err)
			return updateResticRestoreStatus(ctx, deps, restore, backupv1alpha1.PhaseFailed, metav1.Now(), restore.Status.Job.JobRef.Name)
		}
		log.Errorw("Failed to get restore job", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Check job status and update restore status accordingly
	return processRestoreJobStatus(ctx, deps, restore, job)
}

func processIfRestoreJobCompleted(ctx context.Context, deps *utils.Dependencies, resticRestore *backupv1alpha1.ResticRestore, jobOutput *utils.JobOutput) (ctrl.Result, error) {

	// Before marking as complete, remove finalizers from associated workloads
	// No finalizer cleanup required; gating is handled via workload scaling in backup controller paths

	return UpdateResticRestoreStatus(ctx, deps, resticRestore, RestoreJobStatusOptions{
		Phase:         string(backupv1alpha1.PhaseCompleted),
		IsJobFinished: true,
	})
}

func processIfRestoreJobFailed(ctx context.Context, deps *utils.Dependencies, resticRestore *backupv1alpha1.ResticRestore, jobOutput *utils.JobOutput) (ctrl.Result, error) {
	// No finalizer cleanup on failure; nothing to do here
	return UpdateResticRestoreStatus(ctx, deps, resticRestore, RestoreJobStatusOptions{
		Phase:         string(backupv1alpha1.PhaseFailed),
		Message:       fmt.Sprintf("Restore job failed: %s", jobOutput.Error),
		IsJobFinished: true,
	})
}

func processIfRestoreJobRunning(ctx context.Context, deps *utils.Dependencies, resticRestore *backupv1alpha1.ResticRestore, jobOutput *utils.JobOutput) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// RestoreJobStatusOptions holds all possible options for updating a ResticRestore's status.
type RestoreJobStatusOptions struct {
	Phase         string
	Message       string
	JobName       string
	IsJobFinished bool
}

// UpdateResticRestoreStatus updates the status of a ResticRestore.
func UpdateResticRestoreStatus(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore, options RestoreJobStatusOptions) (ctrl.Result, error) {
	restore.Status.Phase = options.Phase
	restore.Status.Error = options.Message

	if options.JobName != "" {
		restore.Status.JobRef = &corev1.LocalObjectReference{Name: options.JobName}
	}
	if options.IsJobFinished && restore.Status.CompletionTime == nil {
		now := metav1.Now()
		restore.Status.CompletionTime = &now
	}

	if err := deps.Client.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update restore status: %w", err)
	}

	requeueTime := constants.DefaultRequeueInterval
	if options.Phase == "Failed" {
		requeueTime = 5 * time.Minute
	}
	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

// getRepositoryForRestore retrieves the ResticRepository for a given ResticRestore.
func getRepositoryForRestore(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (*backupv1alpha1.ResticRepository, error) {
	// First get the backup to find its repository
	backup := &backupv1alpha1.ResticBackup{}
	err := deps.Client.Get(ctx, types.NamespacedName{Name: restore.Spec.BackupName, Namespace: restore.Namespace}, backup)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup %s: %w", restore.Spec.BackupName, err)
	}

	// Now get the repository using the backup's repository reference
	repo := &backupv1alpha1.ResticRepository{}
	err = deps.Client.Get(ctx, types.NamespacedName{Name: backup.Spec.RepositoryRef.Name, Namespace: restore.Namespace}, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository %s: %w", backup.Spec.RepositoryRef.Name, err)
	}
	return repo, nil
}

// createRestoreJob creates a new restore job.
func createRestoreJob(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore, repo *backupv1alpha1.ResticRepository) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("restore-%s", restore.Name)
	pvcName := restore.Spec.TargetPVC

	// Define the restore command
	command, args, err := utils.BuildRestoreJobCommand(restore.Spec.BackupName, restore.Spec.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to build restore command: %w", err)
	}

	// Prepare volume mounts for PVC
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
		},
	}

	// Prepare volumes
	volumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}

	// Build job spec
	jobSpec := utils.ResticJobSpec{
		JobName:      jobName,
		Namespace:    restore.Namespace,
		JobType:      "restore",
		Command:      command,
		Args:         args,
		Repository:   repo.Spec.Repository,
		Password:     repo.Spec.Password,
		Image:        repo.Spec.Image,
		Env:          repo.Spec.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        restore,
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, restore)
	if err != nil {
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	deps.Logger.Infow("Restore job created", "jobName", job.Name)
	return job, nil
}

// processRestoreJobStatus checks the status of the Kubernetes Job and updates the ResticRestore status.
func processRestoreJobStatus(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore, job *batchv1.Job) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-job-status")
	if job.Status.Succeeded > 0 {
		log.Info("Restore job succeeded")
		// On success, remove workload finalizers
		if restore.Labels[constants.LabelRestoreType] == "auto" {
			pvc := &corev1.PersistentVolumeClaim{}
			if err := deps.Client.Get(ctx, types.NamespacedName{Name: restore.Spec.TargetPVC, Namespace: restore.Namespace}, pvc); err != nil {
				log.Errorw("Failed to get PVC for removing workload finalizers", "pvc", restore.Spec.TargetPVC, "error", err)
				// Don't fail the whole restore, but log the issue
			} else if err := removeWorkloadFinalizers(ctx, deps, pvc); err != nil {
				log.Errorw("Failed to remove workload finalizers after successful restore", "error", err)
				// Don't fail the whole restore
			}
		}
		_, err := updateResticRestoreStatus(ctx, deps, restore, backupv1alpha1.PhaseCompleted, metav1.Now(), job.Name)
		return ctrl.Result{}, err
	}

	if job.Status.Failed > 0 {
		podLogs, logErr := utils.GetPodLogs(ctx, deps.Client, job.Namespace, job.Name)
		if logErr != nil {
			log.Errorw("Failed to get logs from failed restore job pod", "error", logErr)
		}
		log.Errorw("Restore job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message, "logs", podLogs)
		restore.Status.Message = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", job.Status.Conditions[0].Reason, job.Status.Conditions[0].Message, podLogs)
		_, err := updateResticRestoreStatus(ctx, deps, restore, backupv1alpha1.PhaseFailed, metav1.Now(), job.Name)
		return ctrl.Result{}, err
	}

	log.Debug("Restore job is still running")
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// updateResticRestoreStatus updates the status of the ResticRestore resource.
func updateResticRestoreStatus(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore, phase backupv1alpha1.Phase, transitionTime metav1.Time, jobName string) (ctrl.Result, error) {
	restore.Status.Phase = string(phase)
	if phase != backupv1alpha1.PhaseFailed {
		restore.Status.Message = "" // Clear previous messages
	}

	if jobName != "" {
		restore.Status.Job.JobRef = &corev1.LocalObjectReference{Name: jobName}
	}
	if restore.Status.CompletionTime == nil && (phase == backupv1alpha1.PhaseCompleted || phase == backupv1alpha1.PhaseFailed) {
		restore.Status.CompletionTime = &transitionTime
	}

	if err := deps.Client.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update restore status: %w", err)
	}

	requeueTime := constants.DefaultRequeueInterval
	if phase == backupv1alpha1.PhaseFailed {
		requeueTime = 5 * time.Minute
	}
	if phase == backupv1alpha1.PhaseCompleted {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

func addWorkloadFinalizers(ctx context.Context, deps *utils.Dependencies, pvc *corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("workload-finalizers")
	workloads, err := utils.ScaleWorkloads(ctx, deps.Client, pvc, -1, log) // -1 finds without scaling
	if err != nil {
		return fmt.Errorf("failed to find workloads to add finalizers: %w", err)
	}

	for _, workload := range workloads {
		if !controllerutil.ContainsFinalizer(workload, constants.WorkloadFinalizer) {
			log.Infow("Adding finalizer to workload", "kind", workload.GetObjectKind().GroupVersionKind().Kind, "name", workload.GetName())
			controllerutil.AddFinalizer(workload, constants.WorkloadFinalizer)
			if err := deps.Client.Update(ctx, workload); err != nil {
				log.Errorw("Failed to add finalizer to workload", "kind", workload.GetObjectKind().GroupVersionKind().Kind, "name", workload.GetName(), "error", err)
				return err
			}
		}
	}
	return nil
}

func removeWorkloadFinalizers(ctx context.Context, deps *utils.Dependencies, pvc *corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("workload-finalizers")
	workloads, err := utils.ScaleWorkloads(ctx, deps.Client, pvc, -1, log) // -1 finds without scaling
	if err != nil {
		return fmt.Errorf("failed to find workloads to remove finalizers: %w", err)
	}

	for _, workload := range workloads {
		if controllerutil.ContainsFinalizer(workload, constants.WorkloadFinalizer) {
			log.Infow("Removing finalizer from workload", "kind", workload.GetObjectKind().GroupVersionKind().Kind, "name", workload.GetName())
			controllerutil.RemoveFinalizer(workload, constants.WorkloadFinalizer)
			if err := deps.Client.Update(ctx, workload); err != nil {
				log.Errorw("Failed to remove finalizer from workload", "kind", workload.GetObjectKind().GroupVersionKind().Kind, "name", workload.GetName(), "error", err)
				return err
			}
		}
	}
	return nil
}
