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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleRepoInitialization handles the logic for a repository in the Initializing phase.
func HandleRepoInitialization(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-init")
	log.Info("Handling repository initialization")

	// 1. Create the initialization job
	job, err := createInitJob(ctx, deps, repo)
	if err != nil {
		log.Errorw("Failed to create init job", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Update status to Initializing
	return updateRepoStatus(ctx, deps, repo, "Initializing", metav1.Now(), job.Name)
}

// HandleRepoInitializationStatus handles checking the status of the initialization job.
func HandleRepoInitializationStatus(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-init-status")
	log.Info("Handling repository initialization status")

	// 1. Get the Kubernetes Job
	job := &batchv1.Job{}
	jobName := repo.Status.CheckJob.JobRef.Name
	if err := deps.Client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: repo.Namespace}, job); err != nil {
		if errors.IsNotFound(err) {
			log.Errorw("Init job not found, assuming failure and moving to Failed phase", "jobName", jobName, "error", err)
			return updateRepoStatus(ctx, deps, repo, "Failed", metav1.Now(), jobName)
		}
		log.Errorw("Failed to get init job", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Check job status
	if job.Status.Succeeded > 0 {
		log.Info("Init job succeeded, repository is ready")
		repo.Status.Initialized = true
		return updateRepoStatus(ctx, deps, repo, "Ready", metav1.Now(), job.Name)
	}

	if job.Status.Failed > 0 {
		log.Errorw("Init job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message)
		return updateRepoStatus(ctx, deps, repo, "Failed", metav1.Now(), job.Name)
	}

	log.Debug("Init job is still running")
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// HandleRepoMaintenance handles scheduled maintenance (check, prune) for a ready repository.
func HandleRepoMaintenance(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-maintenance")
	log.Debug("Handling repository maintenance")
	// This is a placeholder for future maintenance logic (e.g., cron-based check/prune jobs)
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil // Requeue periodically to check for maintenance
}

// HandleRepoFailed handles a repository in the Failed phase.
func HandleRepoFailed(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-failed")
	log.Info("Handling failed repository")
	// Terminal state. We might want to requeue after a long interval to allow for manual intervention.
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
}

// HandleRepoDeletion handles the deletion of a ResticRepository.
func HandleRepoDeletion(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-deletion")
	log.Info("Starting deletion logic for ResticRepository")

	if controllerutil.ContainsFinalizer(repo, constants.ResticRepositoryFinalizer) {
		// Cleanup logic here (e.g., delete jobs if any are running)
		// For now, we assume jobs are owned and garbage collected.

		// Remove the finalizer
		log.Debug("Removing finalizer")
		controllerutil.RemoveFinalizer(repo, constants.ResticRepositoryFinalizer)
		if err := deps.Client.Update(ctx, repo); err != nil {
			log.Errorw("Failed to remove finalizer", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	log.Info("Completed deletion logic for ResticRepository")
	return ctrl.Result{}, nil
}

// CheckRepositoryExists checks if a repository exists.
func CheckRepositoryExists(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (bool, bool, error) {
	log := utils.NewLogger(nil).Named("repo-check").With("repository", repository.Name)
	log.Info("Checking if repository exists")

	// Check if there's an existing check job
	if repository.Status.CheckJob.JobRef != nil {
		log.Debug("Found existing check job")
		return checkExistingJob(ctx, c, scheme, repository)
	}

	// Start new check job
	log.Debug("Starting new check job")
	return startRepositoryCheckJob(ctx, c, scheme, repository)
}

// checkExistingJob checks the status of an existing job.
func checkExistingJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (bool, bool, error) {
	log := utils.NewLogger(nil).Named("repo-check-status").With("repository", repository.Name, "job", repository.Status.CheckJob.JobRef.Name)

	// Get the job
	job := &batchv1.Job{}
	if err := c.Get(ctx, client.ObjectKey{
		Name:      repository.Status.CheckJob.JobRef.Name,
		Namespace: repository.Namespace,
	}, job); err != nil {
		// Job not found - clear reference and retry
		repository.Status.CheckJob.JobRef = nil
		return false, false, c.Status().Update(ctx, repository)
	}

	// Check job status
	if job.Status.Succeeded > 0 {
		// Job succeeded - repository exists
		log.Debug("Repository check job succeeded")
		repository.Status.CheckJob.JobRef = nil
		if err := c.Status().Update(ctx, repository); err != nil {
			return false, false, err
		}
		return true, false, nil
	}

	if job.Status.Failed > 0 {
		// Job failed - repository doesn't exist or is inaccessible
		log.Debug("Repository check job failed")
		repository.Status.CheckJob.JobRef = nil
		if err := c.Status().Update(ctx, repository); err != nil {
			return false, false, err
		}
		return false, false, nil
	}

	// Job still running
	log.Debug("Repository check job still running")
	return false, true, nil
}

// startRepositoryCheckJob starts a new job to check the repository.
func startRepositoryCheckJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (bool, bool, error) {
	log := utils.NewLogger(nil).Named("repo-check-start").With("repository", repository.Name)
	log.Info("Starting repository check job")

	// Create restic job to check repository
	args := []string{"cat", "config"}

	jobName := fmt.Sprintf("restic-check-%s-%d", repository.Name, time.Now().Unix())
	job, err := utils.CreateRepositoryJob(ctx,
		c,
		repository.Namespace,
		jobName,
		repository.Spec.Repository,
		repository.Spec.PasswordSecretRef,
		repository.Spec.Image,
		repository.Spec.Env,
		"check",
		args,
		repository)
	if err != nil {
		return false, false, fmt.Errorf("failed to create check job: %w", err)
	}

	// Save job reference in status
	repository.Status.CheckJob.JobRef = &corev1.LocalObjectReference{Name: job.Name}
	if err := c.Status().Update(ctx, repository); err != nil {
		// Clean up job if status update fails
		_ = c.Delete(ctx, job)
		return false, false, err
	}

	log.Info("Repository check job started")
	return false, true, nil
}

// InitializeRepository initializes a new repository.
func InitializeRepository(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) error {
	log := utils.NewLogger(nil).Named("repo-init").With("repository", repository.Name)
	log.Info("Initializing repository")

	args := []string{"init"}

	jobName := fmt.Sprintf("restic-init-%s", repository.Name)
	_, err := utils.CreateRepositoryJob(ctx,
		c,
		repository.Namespace,
		jobName,
		repository.Spec.Repository,
		repository.Spec.PasswordSecretRef,
		repository.Spec.Image,
		repository.Spec.Env,
		"init",
		args,
		repository)
	if err != nil {
		return fmt.Errorf("failed to create init job: %w", err)
	}

	// Don't wait for completion, let the status checker handle it
	log.Info("Initialization job created")
	return nil
}

// createInitJob creates the job to initialize a Restic repository.
func createInitJob(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) (*batchv1.Job, error) {
	log := deps.Logger.Named("create-init-job")
	jobName := fmt.Sprintf("init-%s", repo.Name)

	command, args, err := utils.BuildInitJobCommand("init")
	if err != nil {
		return nil, fmt.Errorf("failed to build init command: %w", err)
	}

	// Build job spec
	jobSpec := utils.ResticJobSpec{
		JobName:    jobName,
		Namespace:  repo.Namespace,
		JobType:    "init",
		Command:    command,
		Args:       args,
		Repository: repo.Spec.Repository,
		Image:      repo.Spec.Image,
		Env:        repo.Spec.Env,
		Owner:      repo,
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	log.Infow("Init job created", "jobName", job.Name)
	return job, nil
}

// updateRepoStatus updates the status of the ResticRepository resource.
func updateRepoStatus(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository, phase string, transitionTime metav1.Time, jobName string) (ctrl.Result, error) {
	repo.Status.Phase = phase
	repo.Status.Error = ""
	repo.Status.LastChecked = &transitionTime

	// Update job reference if jobName is provided
	if jobName != "" {
		repo.Status.CheckJob.JobRef = &corev1.LocalObjectReference{Name: jobName}
	}

	if repo.Status.InitializedTime == nil && (phase == "Ready" || phase == "Failed") {
		repo.Status.InitializedTime = &transitionTime
	}

	if err := deps.Client.Status().Update(ctx, repo); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// ShouldUpdateStats checks if the stats should be updated.
func ShouldUpdateStats(repository *backupv1alpha1.ResticRepository) bool {
	if repository.Status.Stats == nil || repository.Status.Stats.LastUpdated == nil {
		return true
	}
	return time.Since(repository.Status.Stats.LastUpdated.Time) > time.Hour
}

// ShouldRunMaintenance checks if maintenance should be run.
func ShouldRunMaintenance(repository *backupv1alpha1.ResticRepository) bool {
	if repository.Spec.MaintenanceSchedule == nil || !repository.Spec.MaintenanceSchedule.Enabled {
		return false
	}

	if repository.Status.NextMaintenance == nil {
		return true
	}

	return time.Now().After(repository.Status.NextMaintenance.Time)
}

// CalculateNextMaintenanceTime calculates the next maintenance time.
func CalculateNextMaintenanceTime(deps *utils.Dependencies, repository *backupv1alpha1.ResticRepository) time.Time {
	if repository.Spec.MaintenanceSchedule == nil || !repository.Spec.MaintenanceSchedule.Enabled {
		return time.Now().Add(24 * time.Hour)
	}

	// Use default maintenance schedule if not specified
	scheduleExpr := constants.DefaultMaintenanceSchedule
	if repository.Spec.MaintenanceSchedule.CheckCron != "" {
		scheduleExpr = repository.Spec.MaintenanceSchedule.CheckCron
	}

	cronSchedule, err := deps.CronParser.Parse(scheduleExpr)
	if err != nil {
		return time.Now().Add(24 * time.Hour)
	}

	return cronSchedule.Next(time.Now())
}

// UpdateRepositoryStats updates the stats of the repository.
func UpdateRepositoryStats(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository) error {
	// TODO: Implement stats collection
	log := utils.NewLogger(nil).Named("repo-stats").With("repository", repository.Name)
	log.Info("Stats collection not yet implemented")
	return nil
}

// RunMaintenance runs maintenance on the repository.
func RunMaintenance(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	// TODO: Implement maintenance operations
	log := utils.NewLogger(nil).Named("repo-maintenance").With("repository", repository.Name)
	log.Info("Maintenance operations not yet implemented")
	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

// ShouldVerifySnapshots checks if snapshots should be verified.
func ShouldVerifySnapshots(deps *utils.Dependencies, repository *backupv1alpha1.ResticRepository) bool {
	// Check if maintenance is enabled
	if repository.Spec.MaintenanceSchedule == nil || !repository.Spec.MaintenanceSchedule.Enabled {
		return false
	}

	// Get configured verification schedule (default to daily at 1 AM)
	scheduleExpr := constants.DefaultVerificationSchedule
	if repository.Spec.MaintenanceSchedule.VerificationCron != "" {
		scheduleExpr = repository.Spec.MaintenanceSchedule.VerificationCron
	}

	// Parse the cron schedule
	schedule, err := deps.CronParser.Parse(scheduleExpr)
	if err != nil {
		// Invalid cron expression, fallback to daily check
		if repository.Status.LastSnapshotVerification == nil {
			return true
		}
		return time.Since(repository.Status.LastSnapshotVerification.Time) >= 24*time.Hour
	}

	now := time.Now()

	// If there's no last verification time, verify now
	if repository.Status.LastSnapshotVerification == nil {
		return true
	}

	// Check if it's time for the next verification
	nextVerification := schedule.Next(repository.Status.LastSnapshotVerification.Time)
	return now.After(nextVerification) || now.Equal(nextVerification)
}

// VerifyAllSnapshots verifies all snapshots in the repository.
func VerifyAllSnapshots(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) error {
	log := utils.NewLogger(nil).Named("snapshot-verification").With("repository", repository.Name)
	log.Info("Starting snapshot verification")

	// Get all ResticBackup CRDs for this repository
	resticBackups := &backupv1alpha1.ResticBackupList{}
	if err := c.List(ctx, resticBackups, client.MatchingLabels{
		"repository": repository.Name,
	}); err != nil {
		log.Errorw("Failed to list restic backups", "error", err)
		return fmt.Errorf("failed to list ResticBackup CRDs: %w", err)
	}

	log.Debugw("Found ResticBackup CRDs to verify", "count", len(resticBackups.Items))

	// Track verification results
	verificationErrors := []string{}
	verifiedCount := 0
	failedCount := 0

	// Verify each snapshot exists in the repository
	for _, backup := range resticBackups.Items {
		if backup.Spec.SnapshotID == "" {
			continue // Skip backups without snapshot ID
		}

		backupLog := log.With("backup", backup.Name, "snapshotID", backup.Spec.SnapshotID)

		// Verify this specific snapshot exists
		if err := VerifySnapshot(ctx, c, scheme, repository, backup.Spec.SnapshotID); err != nil {
			backupLog.Errorw("Snapshot verification failed", "error", err)
			failedCount++
			verificationErrors = append(verificationErrors, fmt.Sprintf("%s: %v", backup.Name, err))

			// Mark backup as failed if snapshot doesn't exist
			backup.Status.Phase = "Failed"
			backup.Status.Message = fmt.Sprintf("Snapshot verification failed: %v", err)
			if err := c.Status().Update(ctx, &backup); err != nil {
				backupLog.Debugw("Failed to update backup status", "error", err)
			}
		} else {
			verifiedCount++
			backupLog.Debug("Snapshot verified successfully")

			// Update backup status if it was failed before
			if backup.Status.Phase == "Failed" && backup.Status.Message != "" {
				backup.Status.Phase = "Completed"
				backup.Status.Message = ""
				if err := c.Status().Update(ctx, &backup); err != nil {
					backupLog.Debugw("Failed to update backup status", "error", err)
				}
			}
		}
	}

	// Update repository status with verification results
	now := metav1.Now()
	repository.Status.LastSnapshotVerification = &now
	repository.Status.SnapshotVerificationCount = int64(verifiedCount)
	repository.Status.SnapshotVerificationErrors = int64(failedCount)

	if len(verificationErrors) > 0 {
		repository.Status.SnapshotVerificationError = fmt.Sprintf("Failed to verify %d snapshots", failedCount)
	} else {
		repository.Status.SnapshotVerificationError = ""
	}

	if err := c.Status().Update(ctx, repository); err != nil {
		log.Errorw("Failed to update repository status", "error", err)
		return fmt.Errorf("failed to update repository status: %w", err)
	}

	log.Infow("Snapshot verification completed", "verified", verifiedCount, "failed", failedCount)
	return nil
}

// VerifySnapshot verifies a single snapshot.
func VerifySnapshot(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository, snapshotID string) error {
	args := []string{"cat", "snapshot", snapshotID}

	jobName := fmt.Sprintf("restic-verify-%s-%s", repository.Name, snapshotID[:min(8, len(snapshotID))])
	job, err := utils.CreateRepositoryJob(ctx,
		c,
		repository.Namespace,
		jobName,
		repository.Spec.Repository,
		repository.Spec.PasswordSecretRef,
		repository.Spec.Image,
		repository.Spec.Env,
		"verify",
		args,
		repository)
	if err != nil {
		return fmt.Errorf("failed to create verification job: %w", err)
	}

	// TODO: Implement job completion waiting
	// if err := utils.WaitForJobCompletion(ctx, c, job); err != nil {
	//     return fmt.Errorf("snapshot verification failed: %w", err)
	// }

	return nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HandleStartupDiscovery handles the discovery of repositories on startup.
func HandleStartupDiscovery(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	log := utils.NewLogger(nil).Named("startup-discovery").With("repository", repository.Name)
	log.Info("Repository discovery on startup not implemented")
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// FindRepositoryCRByURL finds a repository CR by its URL.
func FindRepositoryCRByURL(ctx context.Context, c client.Client, namespace, repoURL string) (*backupv1alpha1.ResticRepository, error) {
	log := utils.NewLogger(nil).Named("find-repo-cr").With("repository", repoURL, "namespace", namespace)
	log.Debug("Finding repository CR by URL")

	// List all ResticRepository CRs in the namespace
	var repositories backupv1alpha1.ResticRepositoryList
	if err := c.List(ctx, &repositories, client.InNamespace(namespace)); err != nil {
		log.Errorw("Failed to list repositories", "error", err)
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	// Find repository with matching URL
	for i := range repositories.Items {
		if repositories.Items[i].Spec.Repository == repoURL {
			log.Debugw("Found existing repository CR", "name", repositories.Items[i].Name)
			return &repositories.Items[i], nil
		}
	}

	log.Debug("No existing repository CR found")
	return nil, nil
}

// CreateRepositoryCR creates a new repository CR.
func CreateRepositoryCR(ctx context.Context, c client.Client, namespace, repoURL string) (*backupv1alpha1.ResticRepository, error) {
	log := utils.NewLogger(nil).Named("create-repo-cr").With("repository", repoURL, "namespace", namespace)
	log.Info("Creating repository CR")

	// Check if repository already exists
	existingCR, err := FindRepositoryCRByURL(ctx, c, namespace, repoURL)
	if err != nil {
		return nil, err
	}
	if existingCR != nil {
		return existingCR, nil
	}

	// Generate a name for the new CR
	name := fmt.Sprintf("discovered-repo-%d", time.Now().Unix())

	// Create new CR
	repository := &backupv1alpha1.ResticRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"auto-discovered": "true",
			},
		},
		Spec: backupv1alpha1.ResticRepositorySpec{
			Repository: repoURL,
			PasswordSecretRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "restic-password"},
				Key:                  "password",
			},
			Image: "restic/restic:latest",
		},
	}

	// Create the CR
	if err := c.Create(ctx, repository); err != nil {
		log.Errorw("Failed to create repository CR", "error", err)
		return nil, fmt.Errorf("failed to create repository CR: %w", err)
	}

	log.Infow("Repository CR created", "name", name)
	return repository, nil
}
