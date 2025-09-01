package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// workloadInfo is used to store original replica counts for scaling.
type workloadInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Replicas int32  `json:"replicas"`
}

// HandleBackupPending handles the logic when a ResticBackup is in the Pending phase.
func HandleBackupPending(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup, stopPods bool) (ctrl.Result, error) {
	log := deps.Logger.Named("backup")
	log.Info("Handling pending backup")

	// 1. Add finalizer
	if !controllerutil.ContainsFinalizer(backup, constants.ResticBackupFinalizer) {
		controllerutil.AddFinalizer(backup, constants.ResticBackupFinalizer)
		if err := deps.Client.Update(ctx, backup); err != nil {
			log.Errorw("Failed to add finalizer", "error", err)
			return ctrl.Result{}, err
		}
	}

	// 2. Stop pods if requested
	if stopPods {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := deps.Client.Get(ctx, types.NamespacedName{Name: backup.Spec.PVCRef.Name, Namespace: backup.Namespace}, pvc); err != nil {
			log.Errorw("Failed to get PVC for stopping pods", "pvc", backup.Spec.PVCRef.Name, "error", err)
			return ctrl.Result{}, err
		}
		if err := storeOriginalReplicaCounts(ctx, deps, backup, pvc); err != nil {
			log.Errorw("Failed to store original replica counts", "error", err)
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
	if repo.Status.Phase != string(backupv1alpha1.PhaseReady) {
		log.Debug("Repository not ready, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Create the backup job
	job, err := createBackupJob(ctx, deps, backup, repo)
	if err != nil {
		log.Errorw("Failed to create backup job", "error", err)
		return ctrl.Result{}, err
	}

	// 5. Update status to Running
	return updateBackupStatus(ctx, deps, backup, backupv1alpha1.PhaseRunning, metav1.Now(), job.Name)
}

// HandleBackupRunning handles the logic when a ResticBackup is in the Running phase.
func HandleBackupRunning(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-running")
	log.Info("Handling running backup")

	// 1. Get the Kubernetes Job
	job := &batchv1.Job{}
	jobName := backup.Status.Job.JobRef.Name
	if err := deps.Client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: backup.Namespace}, job); err != nil {
		if errors.IsNotFound(err) {
			log.Errorw("Backup job not found, assuming failure and moving to Failed phase", "jobName", jobName, "error", err)
			return updateBackupStatus(ctx, deps, backup, backupv1alpha1.PhaseFailed, metav1.Now(), jobName)
		}
		log.Errorw("Failed to get backup job", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Check job status and update backup status accordingly
	return processBackupJobStatus(ctx, deps, backup, job)
}

// HandleBackupCompleted handles the logic when a ResticBackup is in the Completed phase.
func HandleBackupCompleted(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-completed")

	log.Info("Handling completed backup")

	// Restore workloads if they were scaled down
	if err := restoreOriginalReplicaCounts(ctx, deps, backup); err != nil {
		log.Errorw("Failed to restore original replica counts on completion", "error", err)
		// Don't requeue, just log. The backup is complete.
	}

	return ctrl.Result{}, nil
}

// HandleBackupFailed handles the logic when a ResticBackup is in the Failed phase.
func HandleBackupFailed(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-failed")
	log.Info("Handling failed backup")

	// Restore workloads if they were scaled down
	if err := restoreOriginalReplicaCounts(ctx, deps, backup); err != nil {
		log.Errorw("Failed to restore original replica counts on failure", "error", err)
		// Don't requeue, just log. The backup has failed.
	}

	return ctrl.Result{}, nil
}

// GetRepositoryForBackup retrieves the ResticRepository for a given ResticBackup.
func GetRepositoryForBackup(ctx context.Context, c client.Client, backup *backupv1alpha1.ResticBackup) (*backupv1alpha1.ResticRepository, error) {
	repo := &backupv1alpha1.ResticRepository{}
	err := c.Get(ctx, types.NamespacedName{Name: backup.Spec.RepositoryRef.Name, Namespace: backup.Namespace}, repo)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// createBackupJob creates a new backup job.
func createBackupJob(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup, repo *backupv1alpha1.ResticRepository) (*batchv1.Job, error) {
	log := deps.Logger.Named("create-backup-job")
	jobName := fmt.Sprintf("backup-%s", backup.Name)
	pvcName := backup.Spec.PVCRef.Name

	// Define the backup command
	command, args, err := utils.BuildBackupJobCommand(jobName, backup.Spec.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to build backup command: %w", err)
	}

	// Prepare volume mounts for PVC
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
			ReadOnly:  true, // Backup is read-only
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
		Namespace:    backup.Namespace,
		JobType:      "backup",
		Command:      command,
		Args:         args,
		Repository:   repo.Spec.Repository,
		Password:     repo.Spec.Password,
		Image:        repo.Spec.Image,
		Env:          repo.Spec.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        backup,
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	log.Infow("Backup job created", "jobName", job.Name)
	return job, nil
}

// processBackupJobStatus checks the status of the Kubernetes Job and updates the ResticBackup status.
func processBackupJobStatus(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup, job *batchv1.Job) (ctrl.Result, error) {
	log := deps.Logger.Named("process-backup-job-status")
	if job.Status.Succeeded > 0 {
		log.Info("Backup job succeeded")
		_, err := updateBackupStatus(ctx, deps, backup, backupv1alpha1.PhaseCompleted, metav1.Now(), job.Name)
		return ctrl.Result{}, err
	}

	if job.Status.Failed > 0 {
		// Attempt to get logs from the failed pod for better error reporting
		podLogs, logErr := utils.GetPodLogs(ctx, deps, job.Namespace, job.Name)
		if logErr != nil {
			log.Errorw("Failed to get logs from failed backup job pod", "error", logErr)
		}
		log.Errorw("Backup job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message, "logs", podLogs)
		backup.Status.Message = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", job.Status.Conditions[0].Reason, job.Status.Conditions[0].Message, podLogs)
		_, err := updateBackupStatus(ctx, deps, backup, backupv1alpha1.PhaseFailed, metav1.Now(), job.Name)
		return ctrl.Result{}, err
	}

	log.Debug("Backup job is still running")
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// updateBackupStatus updates the status of the ResticBackup resource.
func updateBackupStatus(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup, phase backupv1alpha1.Phase, transitionTime metav1.Time, jobName string) (ctrl.Result, error) {
	backup.Status.Phase = phase
	// Message is set in processBackupJobStatus for failures
	if phase != backupv1alpha1.PhaseFailed {
		backup.Status.Message = "" // Clear previous messages
	}

	if jobName != "" {
		backup.Status.Job.JobRef = &corev1.LocalObjectReference{Name: jobName}
	}
	if backup.Status.CompletionTime == nil && (phase == backupv1alpha1.PhaseCompleted || phase == backupv1alpha1.PhaseFailed) {
		backup.Status.CompletionTime = &transitionTime
	}

	if err := deps.Client.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
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

// storeOriginalReplicaCounts annotates the ResticBackup with the original replica counts of the scaled workloads.
func storeOriginalReplicaCounts(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup, pvc *corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("store-original-replicas")
	workloads, err := utils.ScaleWorkloads(ctx, deps.Client, pvc, 0, log)
	if err != nil {
		return fmt.Errorf("failed to scale down workloads: %w", err)
	}

	if len(workloads) == 0 {
		log.Info("No workloads found for PVC, skipping replica count storage")
		return nil
	}

	originalReplicas := make(map[string]workloadInfo)
	for _, workload := range workloads {
		switch w := workload.(type) {
		case *appsv1.Deployment:
			originalReplicas[string(w.UID)] = workloadInfo{Kind: "Deployment", Name: w.Name, Replicas: *w.Spec.Replicas}
		case *appsv1.StatefulSet:
			originalReplicas[string(w.UID)] = workloadInfo{Kind: "StatefulSet", Name: w.Name, Replicas: *w.Spec.Replicas}
		}
	}

	// Store in annotation
	jsonData, err := json.Marshal(originalReplicas)
	if err != nil {
		return fmt.Errorf("failed to marshal original replica counts: %w", err)
	}

	if backup.Annotations == nil {
		backup.Annotations = make(map[string]string)
	}
	backup.Annotations[constants.AnnotationOriginalReplicas] = string(jsonData)

	if err := deps.Client.Update(ctx, backup); err != nil {
		return fmt.Errorf("failed to update backup with original replica annotation: %w", err)
	}

	// Wait for pods to be terminated
	return checkPodsTerminated(ctx, deps, pvc)
}

// restoreOriginalReplicaCounts restores the original replica counts of workloads.
func restoreOriginalReplicaCounts(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup) error {
	log := deps.Logger.Named("restore-original-replicas")
	jsonData, ok := backup.Annotations[constants.AnnotationOriginalReplicas]
	if !ok {
		log.Info("No original replica count annotation found, nothing to restore")
		return nil
	}

	var originalReplicas map[string]workloadInfo
	if err := json.Unmarshal([]byte(jsonData), &originalReplicas); err != nil {
		return fmt.Errorf("failed to unmarshal original replica counts: %w", err)
	}

	for _, info := range originalReplicas {
		var obj client.Object
		var patchData []byte
		switch info.Kind {
		case "Deployment":
			obj = &appsv1.Deployment{}
			patchData = []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, info.Replicas))
		case "StatefulSet":
			obj = &appsv1.StatefulSet{}
			patchData = []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, info.Replicas))
		default:
			continue
		}

		if err := deps.Client.Get(ctx, types.NamespacedName{Name: info.Name, Namespace: backup.Namespace}, obj); err != nil {
			log.Errorw("Failed to get workload for scaling up", "kind", info.Kind, "name", info.Name, "error", err)
			continue
		}

		patch := client.RawPatch(types.MergePatchType, patchData)

		if err := deps.Client.Patch(ctx, obj, patch); err != nil {
			log.Errorw("Failed to restore replica count for workload", "kind", info.Kind, "name", info.Name, "error", err)
			// Continue trying to restore others
		} else {
			log.Infow("Successfully restored replica count for workload", "kind", info.Kind, "name", info.Name, "replicas", info.Replicas)
		}
	}

	return nil
}

// checkPodsTerminated waits for all pods using a PVC to be terminated.
func checkPodsTerminated(ctx context.Context, deps *utils.Dependencies, pvc *corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("check-pods-terminated")
	// This is a simplified check. A more robust implementation might use a watcher
	// or a more sophisticated polling mechanism with backoff.
	for i := 0; i < 30; i++ { // Poll for a max of 5 minutes (30 * 10s)
		workloads, err := utils.ScaleWorkloads(ctx, deps.Client, pvc, -1, log) // -1 means don't scale, just find
		if err != nil {
			return err
		}

		allTerminated := true
		for _, workload := range workloads {
			switch w := workload.(type) {
			case *appsv1.Deployment:
				if w.Status.ReadyReplicas > 0 {
					allTerminated = false
					break
				}
			case *appsv1.StatefulSet:
				if w.Status.ReadyReplicas > 0 {
					allTerminated = false
					break
				}
			}
		}

		if allTerminated {
			log.Info("All pods using PVC have been terminated")
			return nil
		}

		log.Debug("Waiting for pods to terminate...")
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("timed out waiting for pods to terminate for PVC %s", pvc.Name)
}

// getParentBackupConfig retrieves the parent BackupConfig for a ResticBackup
func getParentBackupConfig(ctx context.Context, backup *backupv1alpha1.ResticBackup) (*backupv1alpha1.BackupConfig, error) {
	backupConfigName := backup.Labels["backup-config"]
	if backupConfigName == "" {
		return nil, nil // No error, just no BackupConfig found
	}

	backupConfig := &backupv1alpha1.BackupConfig{}
	err := r.Deps.Client.Get(ctx, types.NamespacedName{
		Name:      backupConfigName,
		Namespace: backup.Namespace,
	}, backupConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent BackupConfig: %w", err)
	}

	return backupConfig, nil
}

// handlePendingPhase handles the pending phase of a backup
func HandlePendingPhase(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.With("name", backup.Name, "namespace", backup.Namespace)

	// Get parent BackupConfig if it exists
	backupConfig, err := getParentBackupConfig(ctx, backup)
	if err != nil {
		log.Errorw("Failed to get parent BackupConfig", "error", err)
		// Continue without stopping pods if we can't get the BackupConfig
		return HandleBackupPending(ctx, deps, backup, false)
	}

	// Determine if we should stop pods

	return HandleBackupPending(ctx, deps, backup, stopPods)
}
