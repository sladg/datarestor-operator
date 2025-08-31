package stubs

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// createResticBackup creates a new ResticBackup resource (merged BackupJob)
func CreateResticBackup(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim, backupType string) error {
	/*
		if len(backupConfig.Spec.BackupTargets) == 0 {
			return fmt.Errorf("no backup targets configured")
		}

		backupName := fmt.Sprintf("%s-%s-%s", backupConfig.Name, pvc.Name, time.Now().Format("20060102-150405"))
		target := backupConfig.Spec.BackupTargets[0] // Use first target
		repositoryName := utils.GetRepositoryNameForTarget(target)

		resticBackup := &backupv1alpha1.ResticBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupName,
				Namespace: pvc.Namespace,
				Labels: map[string]string{
					constants.LabelPVCBackup: backupConfig.Name,
					constants.LabelPVC:       pvc.Name,
					"pvc":                    pvc.Name,
					"backup-config":          backupConfig.Name,
					"backup-target":          target.Name,
				},
			},
			Spec: backupv1alpha1.ResticBackupSpec{
				BackupName: fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
				PVCRef: backupv1alpha1.PVCReference{
					Name:      pvc.Name,
					Namespace: pvc.Namespace,
				},
				RepositoryRef: corev1.LocalObjectReference{
					Name: repositoryName,
				},
				BackupType: backupType,
				Tags: []string{
					fmt.Sprintf("pvc=%s", pvc.Name),
					fmt.Sprintf("namespace=%s", pvc.Namespace),
					fmt.Sprintf("backup-config=%s", backupConfig.Name),
					fmt.Sprintf("backup-type=%s", backupType),
				},
			},
			Status: backupv1alpha1.ResticBackupStatus{
				Phase: "Pending",
			},
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(backupConfig, resticBackup, scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		return c.Create(ctx, resticBackup)
	*/
	return nil
}

// createRestoreJob creates a new ResticRestore resource
func CreateRestoreJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim) error {
	/*
		if len(backupConfig.Spec.BackupTargets) == 0 {
			return fmt.Errorf("no backup targets configured")
		}

		jobName := fmt.Sprintf("restore-%s-%s-%s", backupConfig.Name, pvc.Name, time.Now().Format("20060102-150405"))
		target := backupConfig.Spec.BackupTargets[0] // Use first target

		resticRestore := &backupv1alpha1.ResticRestore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: pvc.Namespace,
				Labels: map[string]string{
					constants.LabelPVCBackup: backupConfig.Name,
					constants.LabelPVC:       pvc.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: backupConfig.APIVersion,
						Kind:       backupConfig.Kind,
						Name:       backupConfig.Name,
						UID:        backupConfig.UID,
					},
				},
			},
			Spec: backupv1alpha1.ResticRestoreSpec{
				BackupName:   fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
				TargetPVC:    pvc.Name,
				BackupTarget: target,
			},
		}

		return c.Create(ctx, resticRestore)
	*/
	return nil
}

// createManualRestoreJob creates a new manual ResticRestore resource
func CreateManualRestoreJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim) error {
	/*
		if len(backupConfig.Spec.BackupTargets) == 0 {
			return fmt.Errorf("no backup targets configured")
		}

		jobName := fmt.Sprintf("manual-restore-%s-%s-%s", backupConfig.Name, pvc.Name, time.Now().Format("20060102-150405"))
		target := backupConfig.Spec.BackupTargets[0] // Use first target

		resticRestore := &backupv1alpha1.ResticRestore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: pvc.Namespace,
				Labels: map[string]string{
					constants.LabelPVCBackup: backupConfig.Name,
					constants.LabelPVC:       pvc.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: backupConfig.APIVersion,
						Kind:       backupConfig.Kind,
						Name:       backupConfig.Name,
						UID:        backupConfig.UID,
					},
				},
			},
			Spec: backupv1alpha1.ResticRestoreSpec{
				BackupName:   fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
				TargetPVC:    pvc.Name,
				BackupTarget: target,
			},
		}

		return c.Create(ctx, resticRestore)
	*/
	return nil
}

// needsAutoRestore checks if a PVC needs automatic restoration
func NeedsAutoRestore(ctx context.Context, c client.Client, pvc corev1.PersistentVolumeClaim) bool {
	/*
		logger := utils.LoggerFrom(ctx, "auto-restore").WithPVC(pvc)

		// Check if PVC has the restore annotation
		if val, ok := pvc.Annotations[constants.AnnotationRestoreNeeded]; ok && val == "true" {
			logger.Debug("PVC has restore annotation, needs auto-restore")
			return true
		}

		// Check if a ResticRestore already exists for this PVC
		var resticRestores backupv1alpha1.ResticRestoreList
		if err := c.List(ctx, &resticRestores,
			client.InNamespace(pvc.Namespace),
			client.MatchingLabels{
				constants.LabelPVC: pvc.Name,
			}); err != nil {
			logger.WithValues("error", err).Debug("Failed to check for existing restic restores")
			return true // Default to creating job if check fails
		}

		// Don't create duplicate ResticRestores
		for _, restore := range resticRestores.Items {
			if restore.Spec.TargetPVC == pvc.Name { // Approximation: if a restore for this PVC exists, don't create another automated one.
				logger.Debug("Automated ResticRestore already exists, skipping")
				return false
			}
		}

		// Always create ResticRestore for auto-restore - let the ResticRestore controller handle the backup existence check
		logger.Debug("No automated ResticRestore exists, needs auto-restore")
		return true
	*/
	return false
}

// shouldCreateBackupJob checks if a backup job should be created for a specific PVC
func ShouldCreateBackupJob(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim, backupType string) bool {
	/*
		logger := utils.LoggerFrom(ctx, "scheduler").
			WithPVC(pvc).
			WithValues("type", backupType)

		// Check existing backup jobs for this PVC
		var resticBackups backupv1alpha1.ResticBackupList
		if err := c.List(ctx, &resticBackups,
			client.InNamespace(pvc.Namespace),
			client.MatchingLabels{
				constants.LabelPVCBackup: backupConfig.Name,
				constants.LabelPVC:       pvc.Name,
			}); err != nil {
			logger.WithValues("error", err).Debug("Failed to check existing backup jobs")
			return true // Default to creating job if check fails
		}

		// Count only pending backups - we don't care about running/completed backups for scheduling decisions
		var pendingBackups []backupv1alpha1.ResticBackup

		for _, backup := range resticBackups.Items {
			switch backup.Status.Phase {
			case "", "Pending":
				pendingBackups = append(pendingBackups, backup)
			case "Running", "Completed", "Ready", "Failed":
				// Don't count these for scheduling decisions
			default:
				// Unknown state, treat as pending to be safe
				pendingBackups = append(pendingBackups, backup)
			}
		}

		logger.WithValues("pending_backups", len(pendingBackups)).Debug("Current pending backup count")

		// Limit pending backups to prevent queue buildup - allow maximum 1 pending backup
		if len(pendingBackups) >= 1 {
			logger.WithValues("pending", len(pendingBackups)).Debug("Maximum pending backup limit reached, skipping backup to prevent queue buildup")
			return false
		}

		// For manual backups, allow if no pending jobs
		if backupType == "manual" {
			logger.Debug("Creating manual backup job - no pending jobs")
			return true
		}

		// Scheduled backup can be created - no pending jobs to cause queue buildup
		// Note: Running job concurrency should be limited by the BackupJob controller
		logger.Debug("Creating scheduled backup job - no pending jobs")
		return true
	*/
	return false
}

// updateBackupStatus updates the BackupConfig status after creating backup jobs
func UpdateBackupStatus(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig, cronParser cron.Parser) {
	/*
		logger := utils.LoggerFrom(ctx, "scheduler").
			WithValues("name", backupConfig.Name)

		now := time.Now()
		backupConfig.Status.LastBackup = &metav1.Time{Time: now}

		// Calculate next backup time
		if backupConfig.Spec.Schedule.Cron != "" {
			schedule, err := cronParser.Parse(backupConfig.Spec.Schedule.Cron)
			if err == nil {
				nextBackup := schedule.Next(now)
				backupConfig.Status.NextBackup = &metav1.Time{Time: nextBackup}
			}
		}

		// Update job statistics by counting actual job states
		if err := UpdateJobStatistics(ctx, c, backupConfig); err != nil {
			logger.WithValues("error", err).Debug("Failed to update job statistics")
		}

		if err := c.Status().Update(ctx, backupConfig); err != nil {
			logger.Failed("update status", err)
		}
	*/
}

// shouldPerformBackup checks if it's time to perform a backup based on schedule
func ShouldPerformBackup(backupConfig *backupv1alpha1.BackupConfig, cronParser cron.Parser) bool {
	/*
		// If no cron schedule, only perform manual backups
		if backupConfig.Spec.Schedule.Cron == "" {
			return false
		}

		// Parse the cron schedule
		schedule, err := cronParser.Parse(backupConfig.Spec.Schedule.Cron)
		if err != nil {
			return false
		}

		now := time.Now()

		// If there's no last backup time, perform backup now
		if backupConfig.Status.LastBackup == nil {
			return true
		}

		// Check if it's time for the next backup
		nextBackup := schedule.Next(backupConfig.Status.LastBackup.Time)
		return now.After(nextBackup) || now.Equal(nextBackup)
	*/
	return false
}

// updateJobStatistics counts and updates BackupJob and RestoreJob statistics
func UpdateJobStatistics(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig) error {
	/*
		logger := utils.LoggerFrom(ctx, "statistics").
			WithValues("backupConfig", backupConfig.Name)

		// Initialize statistics if not already done
		if backupConfig.Status.BackupJobs == nil {
			backupConfig.Status.BackupJobs = &backupv1alpha1.JobStatistics{}
		}
		if backupConfig.Status.RestoreJobs == nil {
			backupConfig.Status.RestoreJobs = &backupv1alpha1.JobStatistics{}
		}

		// Count ResticBackups
		var resticBackups backupv1alpha1.ResticBackupList
		if err := c.List(ctx, &resticBackups,
			client.MatchingLabels{constants.LabelPVCBackup: backupConfig.Name},
		); err != nil {
			logger.WithValues("error", err).Debug("Failed to list ResticBackups")
		} else {
			backupConfig.Status.BackupJobs.Successful = 0
			backupConfig.Status.BackupJobs.Running = 0
			backupConfig.Status.BackupJobs.Failed = 0

			for _, backup := range resticBackups.Items {
				switch backup.Status.Phase {
				case "Ready":
					backupConfig.Status.BackupJobs.Successful++
				case "Running", "Pending":
					backupConfig.Status.BackupJobs.Running++
				case "Failed":
					backupConfig.Status.BackupJobs.Failed++
				}
			}

			logger.WithValues(
				"successful", backupConfig.Status.BackupJobs.Successful,
				"running", backupConfig.Status.BackupJobs.Running,
				"failed", backupConfig.Status.BackupJobs.Failed,
			).Debug("Updated ResticBackup statistics")
		}

		// Count ResticRestores for this BackupConfig
		var resticRestores backupv1alpha1.ResticRestoreList
		if err := c.List(ctx, &resticRestores,
			client.MatchingLabels{constants.LabelPVCBackup: backupConfig.Name},
		); err != nil {
			logger.WithValues("error", err).Debug("Failed to list ResticRestores")
		} else {
			backupConfig.Status.RestoreJobs.Successful = 0
			backupConfig.Status.RestoreJobs.Running = 0
			backupConfig.Status.RestoreJobs.Failed = 0

			for _, restore := range resticRestores.Items {
				switch restore.Status.Phase {
				case "Completed":
					backupConfig.Status.RestoreJobs.Successful++
				case "Running", "Pending":
					backupConfig.Status.RestoreJobs.Running++
				case "Failed":
					backupConfig.Status.RestoreJobs.Failed++
				}
			}
			logger.WithValues(
				"successful", backupConfig.Status.RestoreJobs.Successful,
				"running", backupConfig.Status.RestoreJobs.Running,
				"failed", backupConfig.Status.RestoreJobs.Failed,
			).Debug("Updated ResticRestore statistics")
		}

		return nil
	*/
	return nil
}

// handleJobSchedulingOriginal handles all job scheduling logic (backup and restore jobs)
func HandleJobSchedulingOriginal(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, matchedPVCs []corev1.PersistentVolumeClaim, cronParser cron.Parser) (ctrl.Result, error) {
	/*
		logger := utils.LoggerFrom(ctx, "job-scheduling").
			WithValues("backupConfig", backupConfig.Name, "pvcCount", len(matchedPVCs))

		// If no matching PVCs, nothing to schedule
		if len(matchedPVCs) == 0 {
			logger.WithValues("namespaces", backupConfig.Spec.PVCSelector.Namespaces).
				Debug("No matching PVCs found, skipping job scheduling")
			return ctrl.Result{}, nil
		}

		logger.Debug("Processing job scheduling for matched PVCs")

		// 1. Handle manual backup trigger (highest priority)
		if result, err := HandleManualBackupTrigger(ctx, c, scheme, backupConfig, matchedPVCs); err != nil {
			return ctrl.Result{}, err
		} else if !result.IsZero() {
			return result, nil
		}

		// 2. Handle scheduled backups
		if err := HandleScheduledBackups(ctx, c, scheme, backupConfig, matchedPVCs, cronParser); err != nil {
			return ctrl.Result{}, err
		}

		// 3. Handle auto-restore for new PVCs and manual restore triggers
		if err := HandleRestoreLogic(ctx, c, scheme, backupConfig, matchedPVCs); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	*/
	return ctrl.Result{}, nil
}

// handleManualBackupTrigger handles manual backup triggers
func HandleManualBackupTrigger(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, matchedPVCs []corev1.PersistentVolumeClaim) (ctrl.Result, error) {
	/*
		logger := utils.LoggerFrom(ctx, "manual-backup")

		// Check for manual backup trigger
		if val, ok := backupConfig.Annotations[constants.AnnotationManualBackupTrigger]; !ok || val != "now" {
			return ctrl.Result{}, nil
		}

		logger.Starting("schedule manual backup")

		for _, pvc := range matchedPVCs {
			if ShouldCreateBackupJob(ctx, c, backupConfig, pvc, "manual") {
				if err := CreateResticBackup(ctx, c, scheme, backupConfig, pvc, "manual"); err != nil {
					logger.WithPVC(pvc).Failed("create manual backup job", err)
				} else {
					logger.WithPVC(pvc).Debug("Created manual backup job")
				}
			}
		}

		// Remove the annotation after scheduling
		delete(backupConfig.Annotations, constants.AnnotationManualBackupTrigger)
		if err := c.Update(ctx, backupConfig); err != nil {
			logger.Failed("remove manual-trigger annotation", err)
			return ctrl.Result{}, err
		}

		logger.Completed("schedule manual backup")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	*/
	return ctrl.Result{}, nil
}

// handleScheduledBackups handles scheduled backup jobs
func HandleScheduledBackups(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, matchedPVCs []corev1.PersistentVolumeClaim, cronParser cron.Parser) error {
	/*
		logger := utils.LoggerFrom(ctx, "scheduled-backup")

		// Check if it's time for scheduled backup
		if !ShouldPerformBackup(backupConfig, cronParser) {
			return nil
		}

		logger.Starting("schedule backup")
		jobsCreated := false

		for _, pvc := range matchedPVCs {
			if ShouldCreateBackupJob(ctx, c, backupConfig, pvc, "scheduled") {
				if err := CreateResticBackup(ctx, c, scheme, backupConfig, pvc, "scheduled"); err != nil {
					logger.WithPVC(pvc).Failed("create scheduled backup job", err)
				} else {
					jobsCreated = true
					logger.WithPVC(pvc).Debug("Created scheduled backup job")
				}
			}
		}

		if jobsCreated {
			UpdateBackupStatus(ctx, c, backupConfig, cronParser)
		}

		logger.Completed("schedule backup")
		return nil
	*/
	return nil
}

// handleRestoreLogic handles both automatic and manual restore job creation
func HandleRestoreLogic(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, matchedPVCs []corev1.PersistentVolumeClaim) error {
	/*
		logger := utils.LoggerFrom(ctx, "restore-logic")

		for _, pvc := range matchedPVCs {
			// Check for manual restore trigger on PVC
			if val, ok := pvc.Annotations[constants.AnnotationManualRestoreTrigger]; ok && val == "now" {
				logger.WithPVC(pvc).Starting("schedule manual restore")

				// Create manual restore job for this specific PVC
				if err := CreateManualRestoreJob(ctx, c, scheme, backupConfig, pvc); err != nil {
					logger.WithPVC(pvc).Failed("create manual restore job", err)
				} else {
					logger.WithPVC(pvc).Debug("Created manual restore job")

					// Remove the annotation after scheduling
					pvcCopy := pvc.DeepCopy()
					delete(pvcCopy.Annotations, constants.AnnotationManualRestoreTrigger)
					if err := c.Update(ctx, pvcCopy); err != nil {
						logger.WithPVC(pvc).Failed("remove manual-restore-trigger annotation", err)
					}
				}
				continue // Skip auto-restore check for manually triggered PVCs
			}

			// Handle auto-restore for new PVCs (only if not manually triggered)
			if backupConfig.Spec.AutoRestore && NeedsAutoRestore(ctx, c, pvc) {
				logger.WithPVC(pvc).Starting("schedule auto restore")

				if err := CreateRestoreJob(ctx, c, scheme, backupConfig, pvc); err != nil {
					logger.WithPVC(pvc).Failed("create auto restore job", err)
				} else {
					logger.WithPVC(pvc).Debug("Created auto restore job")
				}
			}
		}

		return nil
	*/
	return nil
}

// findMatchingPVCs finds PVCs that match the BackupConfig's selector
func FindMatchingPVCs(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig) ([]corev1.PersistentVolumeClaim, error) {
	/*
		return utils.FindMatchingPVCs(ctx, c, backupConfig.Spec.PVCSelector.LabelSelector.MatchLabels, backupConfig.Spec.PVCSelector.Namespaces)
	*/
	return nil, nil
}

// findObjectsForPVC returns BackupConfig reconcile requests for a PVC
func FindObjectsForPVC(ctx context.Context, c client.Client, obj client.Object) []reconcile.Request {
	/*
		return watches.FindObjectsForPVC(ctx, c, obj)
	*/
	return nil
}

// ensureResticRepositories creates ResticRepository CRDs for all backup targets
func EnsureResticRepositories(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig) error {
	/*
		logger := utils.LoggerFrom(ctx, "repositories").
			WithValues("name", backupConfig.Name, "namespace", backupConfig.Namespace)
		logger.Starting("ensure restic repositories")

		for _, target := range backupConfig.Spec.BackupTargets {
			if target.Restic == nil {
				// Skip non-restic targets
				continue
			}

			repoName := utils.GetRepositoryNameForTarget(target)
			if err := EnsureResticRepository(ctx, c, scheme, backupConfig, target, repoName); err != nil {
				return fmt.Errorf("failed to ensure repository %s: %w", repoName, err)
			}
		}

		logger.Completed("ensure restic repositories")
		return nil
	*/
	return nil
}

// ensureResticRepository creates a single ResticRepository CRD if it doesn't exist
func EnsureResticRepository(ctx context.Context, c client.Client, scheme *runtime.Scheme, backupConfig *backupv1alpha1.BackupConfig, target backupv1alpha1.BackupTarget, repoName string) error {
	/*
		logger := utils.LoggerFrom(ctx, "repository").
			WithValues("repository", repoName, "target", target.Name)

		// Check if repository already exists
		existingRepo := &backupv1alpha1.ResticRepository{}
		err := c.Get(ctx, client.ObjectKey{
			Name:      repoName,
			Namespace: backupConfig.Namespace,
		}, existingRepo)

		if err == nil {
			// Repository exists, verify ownership
			if !IsOwnedByBackupConfig(existingRepo, backupConfig) {
				logger.Starting("set owner reference")
				if err := controllerutil.SetControllerReference(backupConfig, existingRepo, scheme); err != nil {
					return fmt.Errorf("failed to set owner reference: %w", err)
				}
				if err := c.Update(ctx, existingRepo); err != nil {
					return fmt.Errorf("failed to update repository ownership: %w", err)
				}
				logger.Completed("set owner reference")
			}
			return nil
		}

		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check repository existence: %w", err)
		}

		// Create new repository
		logger.Starting("create repository")
		repo := &backupv1alpha1.ResticRepository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      repoName,
				Namespace: backupConfig.Namespace,
				Labels: map[string]string{
					"backup-config": backupConfig.Name,
					"target":        target.Name,
				},
			},
			Spec: backupv1alpha1.ResticRepositorySpec{
				Repository: target.Restic.Repository,
				Password:   target.Restic.Password,
				Env:        target.Restic.Env,
				Flags:      target.Restic.Flags,
				Image:      target.Restic.Image,
				AutoInit:   true,
				MaintenanceSchedule: &backupv1alpha1.RepositoryMaintenanceSchedule{
					CheckCron: "0 4 * * 0", // Weekly at 4 AM
					PruneCron: "0 5 * * 0", // Weekly at 5 AM
					Enabled:   true,
				},
			},
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(backupConfig, repo, scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		// Create the repository
		if err := c.Create(ctx, repo); err != nil {
			return fmt.Errorf("failed to create repository: %w", err)
		}

		logger.Completed("create repository")
		return nil
	*/
	return nil
}

// isOwnedByBackupConfig checks if a ResticRepository is owned by the given BackupConfig
func IsOwnedByBackupConfig(repo *backupv1alpha1.ResticRepository, backupConfig *backupv1alpha1.BackupConfig) bool {
	/*
		for _, ownerRef := range repo.OwnerReferences {
			if ownerRef.Kind == "BackupConfig" &&
				ownerRef.Name == backupConfig.Name &&
				ownerRef.UID == backupConfig.UID {
				return true
			}
		}
		return false
	*/
	return false
}

// shouldApplyRetentionPolicy checks if retention policy should be applied using cron schedule
func ShouldApplyRetentionPolicy(backupConfig *backupv1alpha1.BackupConfig, cronParser cron.Parser) bool {
	/*
		if backupConfig.Spec.RetentionPolicy == nil {
			return false // No retention policy defined
		}

		// Get configured schedule (default to daily at 3 AM)
		scheduleExpr := constants.DefaultRetentionSchedule
		if backupConfig.Spec.RetentionPolicy.Schedule != "" {
			scheduleExpr = backupConfig.Spec.RetentionPolicy.Schedule
		}

		// Parse the cron schedule
		schedule, err := cronParser.Parse(scheduleExpr)
		if err != nil {
			// Invalid cron expression, fallback to daily check
			if backupConfig.Status.LastRetentionCheck == nil {
				return true
			}
			return time.Since(backupConfig.Status.LastRetentionCheck.Time) >= 24*time.Hour
		}

		now := time.Now()

		// If there's no last retention check time, check if it's time based on schedule
		if backupConfig.Status.LastRetentionCheck == nil {
			return true // First time - run now and then follow schedule
		}

		// Check if it's time for the next retention check
		nextRetention := schedule.Next(backupConfig.Status.LastRetentionCheck.Time)
		return now.After(nextRetention) || now.Equal(nextRetention)
	*/
	return false
}

// applyRetentionPolicy deletes ResticBackup CRDs that exceed the retention policy using restic forget --dry-run
func ApplyRetentionPolicy(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig) error {
	/*
		if backupConfig.Spec.RetentionPolicy == nil {
			// No retention policy defined
			return nil
		}

		logger := utils.LoggerFrom(ctx, "retention").
			WithValues("backupConfig", backupConfig.Name)

		// Get all PVCs managed by this BackupConfig
		matchedPVCs, err := FindMatchingPVCs(ctx, c, backupConfig)
		if err != nil {
			return fmt.Errorf("failed to find matching PVCs: %w", err)
		}

		// Apply retention policy for each PVC separately
		for _, pvc := range matchedPVCs {
			if err := utils.ApplyRetentionPolicyForPVC(ctx, c, backupConfig, pvc); err != nil {
				logger.WithValues("error", err, "pvc", pvc.Name).Debug("Failed to apply retention policy for PVC")
				// Continue with other PVCs even if one fails
			}
		}

		return nil
	*/
	return nil
}

// findBackupTargetByName finds a backup target by name
func FindBackupTargetByName(targets []backupv1alpha1.BackupTarget, name string) *backupv1alpha1.BackupTarget {
	/*
		for _, target := range targets {
			if target.Name == name {
				return &target
			}
		}
		return nil
	*/
	return nil
}

// findObjectsForPod returns BackupConfig reconcile requests for a Pod
func FindObjectsForPod(ctx context.Context, c client.Client, obj client.Object) []reconcile.Request {
	/*
		return watches.FindObjectsForPod(ctx, c, obj)
	*/
	return nil
}

// IndexPodByPVCClaimName is an indexer function to find pods by PVC claim name.
func IndexPodByPVCClaimName(obj client.Object) []string {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return []string{}
	}
	var pvcNames []string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}
	return pvcNames
}

// Restic Repository Stubs

func CheckRepositoryExists(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (bool, bool, error) {
	/*
		logger := utils.LoggerFrom(ctx, "repo-check").
			WithValues("repository", repository.Name)
		logger.Starting("check repository exists")

		// Check if there's an existing check job
		if repository.Status.CheckJobRef != nil {
			logger.Debug("Found existing check job")
			return CheckExistingJob(ctx, c, scheme, repository)
		}

		// Start new check job
		logger.Debug("Starting new check job")
		return StartRepositoryCheckJob(ctx, c, scheme, repository)
	*/
	return false, false, nil
}

func CheckExistingJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (bool, bool, error) {
	/*
		logger := utils.LoggerFrom(ctx, "repo-check-status").
			WithValues("repository", repository.Name, "job", repository.Status.CheckJobRef.Name)

		// Get the job
		job := &batchv1.Job{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      repository.Status.CheckJobRef.Name,
			Namespace: repository.Namespace,
		}, job); err != nil {
			// Job not found - clear reference and retry
			repository.Status.CheckJobRef = nil
			return false, false, c.Status().Update(ctx, repository)
		}

		// Check job status
		if job.Status.Succeeded > 0 {
			// Job succeeded - repository exists
			logger.Debug("Repository check job succeeded")
			repository.Status.CheckJobRef = nil
			if err := c.Status().Update(ctx, repository); err != nil {
				return false, false, err
			}
			// Clean up job
			_ = c.Delete(ctx, job)
			return true, false, nil
		}

		if job.Status.Failed > 0 {
			// Job failed - repository doesn't exist or is inaccessible
			logger.Debug("Repository check job failed")
			repository.Status.CheckJobRef = nil
			if err := c.Status().Update(ctx, repository); err != nil {
				return false, false, err
			}
			// Clean up job
			_ = c.Delete(ctx, job)
			return false, false, nil
		}

		// Job still running
		logger.Debug("Repository check job still running")
		return false, true, nil
	*/
	return false, false, nil
}

func StartRepositoryCheckJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (bool, bool, error) {
	/*
		logger := utils.LoggerFrom(ctx, "repo-check-start").
			WithValues("repository", repository.Name)
		logger.Starting("start repository check job")

		// Create restic job to check repository
		args := []string{"cat", "config"}

		jobName := fmt.Sprintf("restic-check-%s-%d", repository.Name, time.Now().Unix())
		job, err := CreateRepositoryJob(ctx,
			c,
			repository.Namespace,
			jobName,
			repository.Spec.Repository,
			repository.Spec.Password,
			repository.Spec.Image,
			repository.Spec.Env,
			"check",
			args,
			repository)
		if err != nil {
			return false, false, fmt.Errorf("failed to create check job: %w", err)
		}

		// Save job reference in status
		repository.Status.CheckJobRef = &corev1.LocalObjectReference{Name: job.Name}
		if err := c.Status().Update(ctx, repository); err != nil {
			// Clean up job if status update fails
			_ = c.Delete(ctx, job)
			return false, false, err
		}

		logger.Completed("repository check job started")
		return false, true, nil
	*/
	return false, false, nil
}

func InitializeRepository(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) error {
	/*
		logger := utils.LoggerFrom(ctx, "repo-init").
			WithValues("repository", repository.Name)
		logger.Starting("initialize repository")

		args := []string{"init"}

		jobName := fmt.Sprintf("restic-init-%s", repository.Name)
		_, err := CreateRepositoryJob(ctx,
			c,
			repository.Namespace,
			jobName,
			repository.Spec.Repository,
			repository.Spec.Password,
			repository.Spec.Image,
			repository.Spec.Env,
			"init",
			args,
			repository)
		if err != nil {
			return fmt.Errorf("failed to create init job: %w", err)
		}

		// Don't wait for completion, let the status checker handle it
		logger.Completed("initialization job created")
		return nil
	*/
	return nil
}

func UpdateRepositoryStatus(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository, phase, errorMsg string, initialized bool) (ctrl.Result, error) {
	repository.Status.Phase = phase
	repository.Status.Initialized = initialized
	repository.Status.Error = errorMsg

	now := metav1.Now()
	repository.Status.LastChecked = &now

	if initialized && repository.Status.InitializedTime == nil {
		repository.Status.InitializedTime = &now
	}

	if err := c.Status().Update(ctx, repository); err != nil {
		return ctrl.Result{}, err
	}

	requeueTime := constants.DefaultRequeueInterval
	if phase == "Failed" {
		requeueTime = 5 * time.Minute
	}
	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

func ShouldUpdateStats(repository *backupv1alpha1.ResticRepository) bool {
	/*
		if repository.Status.Stats == nil || repository.Status.Stats.LastUpdated == nil {
			return true
		}
		return time.Since(repository.Status.Stats.LastUpdated.Time) > time.Hour
	*/
	return false
}

func ShouldRunMaintenance(repository *backupv1alpha1.ResticRepository) bool {
	/*
		schedule := repository.Spec.MaintenanceSchedule
		if schedule == nil || !schedule.Enabled {
			return false
		}

		if repository.Status.NextMaintenance == nil {
			return true
		}

		return time.Now().After(repository.Status.NextMaintenance.Time)
	*/
	return false
}

func CalculateNextMaintenanceTime(repository *backupv1alpha1.ResticRepository, cronParser cron.Parser) time.Time {
	/*
		schedule := repository.Spec.MaintenanceSchedule
		if schedule == nil || !schedule.Enabled {
			return time.Now().Add(24 * time.Hour)
		}

		// Use default maintenance schedule if not specified
		scheduleExpr := constants.DefaultMaintenanceSchedule
		if schedule.CheckCron != "" {
			scheduleExpr = schedule.CheckCron
		}

		cronSchedule, err := cronParser.Parse(scheduleExpr)
		if err != nil {
			return time.Now().Add(24 * time.Hour)
		}

		return cronSchedule.Next(time.Now())
	*/
	return time.Time{}
}

func UpdateRepositoryStats(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository) error {
	// Stats collection not yet implemented
	return nil
}

func RunMaintenance(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	// Maintenance operations not yet implemented
	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func ShouldVerifySnapshots(repository *backupv1alpha1.ResticRepository, cronParser cron.Parser) bool {
	/*
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
		schedule, err := cronParser.Parse(scheduleExpr)
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
	*/
	return false
}

func VerifyAllSnapshots(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) error {
	/*
		logger := utils.LoggerFrom(ctx, "snapshot-verification").
			WithValues("repository", repository.Name)
		logger.Starting("verify all snapshots")

		// Get all ResticBackup CRDs for this repository
		resticBackups := &backupv1alpha1.ResticBackupList{}
		if err := c.List(ctx, resticBackups, client.MatchingLabels{
			"repository": repository.Name,
		}); err != nil {
			logger.Failed("list restic backups", err)
			return fmt.Errorf("failed to list ResticBackup CRDs: %w", err)
		}

		logger.WithValues("backupCount", len(resticBackups.Items)).Debug("Found ResticBackup CRDs to verify")

		// Track verification results
		verificationErrors := []string{}
		verifiedCount := 0
		failedCount := 0

		// Verify each snapshot exists in the repository
		for _, backup := range resticBackups.Items {
			if backup.Spec.SnapshotID == "" {
				continue // Skip backups without snapshot ID
			}

			logger := logger.WithValues("backup", backup.Name, "snapshotID", backup.Spec.SnapshotID)

			// Verify this specific snapshot exists
			if err := VerifySnapshot(ctx, c, scheme, repository, backup.Spec.SnapshotID); err != nil {
				logger.Failed("verify snapshot", err)
				failedCount++
				verificationErrors = append(verificationErrors, fmt.Sprintf("%s: %v", backup.Name, err))

				// Mark backup as failed if snapshot doesn't exist
				backup.Status.Phase = "Failed"
				backup.Status.Error = fmt.Sprintf("Snapshot verification failed: %v", err)
				if err := c.Status().Update(ctx, &backup); err != nil {
					logger.WithValues("error", err).Debug("Failed to update backup status")
				}
			} else {
				verifiedCount++
				logger.Debug("Snapshot verified successfully")

				// Update backup status if it was failed before
				if backup.Status.Phase == "Failed" && backup.Status.Error != "" {
					backup.Status.Phase = "Ready"
					backup.Status.Error = ""
					if err := c.Status().Update(ctx, &backup); err != nil {
						logger.WithValues("error", err).Debug("Failed to update backup status")
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
			repository.Status.SnapshotVerificationError = fmt.Sprintf("Failed to verify %d snapshots: %v", failedCount, verificationErrors[:min(len(verificationErrors), 3)]) // Limit error message length
		} else {
			repository.Status.SnapshotVerificationError = ""
		}

		if err := c.Status().Update(ctx, repository); err != nil {
			logger.Failed("update repository status", err)
			return fmt.Errorf("failed to update repository status: %w", err)
		}

		logger.WithValues("verified", verifiedCount, "failed", failedCount).Completed("snapshot verification")
		return nil
	*/
	return nil
}

func VerifySnapshot(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository, snapshotID string) error {
	/*
		args := []string{"cat", "snapshot", snapshotID}

		jobName := fmt.Sprintf("restic-verify-%s-%s", repository.Name, snapshotID[:8])
		job, err := CreateRepositoryJob(ctx,
			c,
			repository.Namespace,
			jobName,
			repository.Spec.Repository,
			repository.Spec.Password,
			repository.Spec.Image,
			repository.Spec.Env,
			"verify",
			args,
			repository)
		if err != nil {
			return fmt.Errorf("failed to create verification job: %w", err)
		}
		defer func() { _ = c.Delete(ctx, job) }()

		// Wait for job completion
		if err := WaitForJobCompletion(ctx, c, job); err != nil {
			return fmt.Errorf("snapshot verification failed: %w", err)
		}

		return nil
	*/
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func HandleStartupDiscovery(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	/*
		logger := utils.LoggerFrom(ctx, "startup-discovery").
			WithValues("repository", repository.Name)
		logger.Starting("discover repository on startup")

		// Check if repository exists and get its info
		info, err := DiscoverRepository(ctx, c, repository.Namespace, repository.Spec.Repository, repository.Spec.Password, repository.Spec.Env, repository.Spec.Image)
		if err != nil {
			logger.Failed("discover repository", err)
			return UpdateRepositoryStatus(ctx, c, repository, "Failed", fmt.Sprintf("Failed to discover repository: %v", err), false)
		}

		if !info.Exists {
			// Repository doesn't exist
			if repository.Spec.AutoInit {
				// Initialize if auto-init is enabled
				return handleInitialization(ctx, repository)
			}
			return UpdateRepositoryStatus(ctx, c, repository, "Failed", "Repository doesn't exist and auto-init is disabled", false)
		}

		// Repository exists, collect stats
		stats, err := CollectRepositoryStats(ctx, c, repository.Namespace, repository.Spec.Repository, repository.Spec.Password, repository.Spec.Env, repository.Spec.Image)
		if err != nil {
			logger.Failed("collect stats", err)
			return UpdateRepositoryStatus(ctx, c, repository, "Failed", fmt.Sprintf("Failed to collect repository stats: %v", err), false)
		}

		// Update repository status with stats
		repository.Status.Stats = stats
		repository.Status.Phase = "Ready"
		repository.Status.Initialized = true
		repository.Status.InitializedTime = &metav1.Time{Time: info.LastModified}
		repository.Status.LastChecked = &metav1.Time{Time: time.Now()}
		repository.Status.Error = ""

		if err := c.Status().Update(ctx, repository); err != nil {
			logger.Failed("update status", err)
			return ctrl.Result{}, err
		}

		logger.Completed("repository discovered and stats collected")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	*/
	return ctrl.Result{}, nil
}

func FindRepositoryCRByURL(ctx context.Context, c client.Client, namespace, repoURL string) (*backupv1alpha1.ResticRepository, error) {
	/*
		logger := utils.LoggerFrom(ctx, "find-repo-cr").
			WithValues("repository", repoURL, "namespace", namespace)
		logger.Starting("find repository CR by URL")

		// List all ResticRepository CRs in the namespace
		var repositories backupv1alpha1.ResticRepositoryList
		if err := c.List(ctx, &repositories, client.InNamespace(namespace)); err != nil {
			logger.Failed("list repositories", err)
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		// Find repository with matching URL
		for i := range repositories.Items {
			if repositories.Items[i].Spec.Repository == repoURL {
				logger.WithValues("name", repositories.Items[i].Name).Debug("Found existing repository CR")
				return &repositories.Items[i], nil
			}
		}

		logger.Debug("No existing repository CR found")
		return nil, nil
	*/
	return nil, nil
}

func CreateRepositoryCR(ctx context.Context, c client.Client, namespace, repoURL string) (*backupv1alpha1.ResticRepository, error) {
	/*
		logger := utils.LoggerFrom(ctx, "create-repo-cr").
			WithValues("repository", repoURL, "namespace", namespace)
		logger.Starting("create repository CR")

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
				Password:   constants.DefaultResticPassword, // Use default password for discovered repositories
				AutoInit:   true,                            // Enable auto-init for discovered repositories
				Image:      "restic/restic:latest",          // Use default image
			},
		}

		// Create the CR
		if err := c.Create(ctx, repository); err != nil {
			logger.Failed("create repository", err)
			return nil, fmt.Errorf("failed to create repository CR: %w", err)
		}

		logger.WithValues("name", name).Completed("repository CR created")
		return repository, nil
	*/
	return nil, nil
}

// Stubs for missing utils functions
func CollectRepositoryStats(ctx context.Context, c client.Client, namespace, repository, password string, env []corev1.EnvVar, image string) (*backupv1alpha1.RepositoryStats, error) {
	return nil, nil
}

func CreateRepositoryJob(ctx context.Context, c client.Client, namespace, name, repository, password, image string, env []corev1.EnvVar, command string, args []string, owner metav1.Object) (*batchv1.Job, error) {
	return nil, nil
}

func WaitForJobCompletion(ctx context.Context, c client.Client, job *batchv1.Job) error {
	return nil
}

type RepositoryInfo struct {
	Exists       bool
	LastModified time.Time
}

func DiscoverRepository(ctx context.Context, c client.Client, namespace, repository, password string, env []corev1.EnvVar, image string) (*RepositoryInfo, error) {
	return &RepositoryInfo{Exists: false}, nil
}

// ResticBackup Stubs

func CreateBackupJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository) (*batchv1.Job, error) {
	/*
		logger := utils.LoggerFrom(ctx, "create-job").
			WithValues("backup", backup.Name)
		logger.Starting("create backup job")

		// Generate backup ID for tracking
		backupID := fmt.Sprintf("%s-%d", backup.Name, time.Now().Unix())
		backup.Status.BackupID = backupID

		// Build backup command
		command, args, err := utils.BuildBackupJobCommand(backupID, backup.Spec.Tags)
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
						ClaimName: backup.Spec.PVCRef.Name,
					},
				},
			},
		}

		// Build job spec
		jobName := fmt.Sprintf("backup-%s-%d", backup.Name, time.Now().Unix())
		jobSpec := utils.ResticJobSpec{
			JobName:      jobName,
			Namespace:    backup.Namespace,
			JobType:      "backup",
			Command:      command,
			Args:         args,
			Repository:   repository.Spec.Repository,
			Password:     repository.Spec.Password,
			Image:        repository.Spec.Image,
			Env:          repository.Spec.Env,
			VolumeMounts: volumeMounts,
			Volumes:      volumes,
			Owner:        backup,
		}

		job, _, err := utils.CreateResticJobWithOutput(ctx, c, jobSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to create restic job: %w", err)
		}

		logger.Completed("backup job created")
		return job, nil
	*/
	return nil, nil
}

func CreateVerificationJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository) (*batchv1.Job, error) {
	/*
		logger := utils.LoggerFrom(ctx, "create-verify-job").
			WithValues("backup", backup.Name, "snapshotID", backup.Spec.SnapshotID)
		logger.Starting("create verification job")

		// Build job spec
		jobName := fmt.Sprintf("verify-%s-%d", backup.Name, time.Now().Unix())
		jobSpec := utils.ResticJobSpec{
			JobName:    jobName,
			Namespace:  backup.Namespace,
			JobType:    "verify",
			Command:    []string{"restic"},
			Args:       []string{"snapshots", "--json", backup.Spec.SnapshotID},
			Repository: repository.Spec.Repository,
			Password:   repository.Spec.Password,
			Image:      repository.Spec.Image,
			Env:        repository.Spec.Env,
			Owner:      backup,
		}

		job, _, err := utils.CreateResticJobWithOutput(ctx, c, jobSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to create verify job: %w", err)
		}

		logger.Completed("verification job created")
		return job, nil
	*/
	return nil, nil
}

func CreateDeletionJob(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository) (*batchv1.Job, error) {
	/*
		logger := utils.LoggerFrom(ctx, "create-delete-job").
			WithValues("backup", backup.Name, "snapshotID", backup.Spec.SnapshotID)
		logger.Starting("create deletion job")

		// Build job spec
		jobName := fmt.Sprintf("delete-%s-%d", backup.Name, time.Now().Unix())
		jobSpec := utils.ResticJobSpec{
			JobName:    jobName,
			Namespace:  backup.Namespace,
			JobType:    "delete",
			Command:    []string{"restic"},
			Args:       []string{"forget", backup.Spec.SnapshotID, "--prune"},
			Repository: repository.Spec.Repository,
			Password:   repository.Spec.Password,
			Image:      repository.Spec.Image,
			Env:        repository.Spec.Env,
			Owner:      backup,
		}

		job, _, err := utils.CreateResticJobWithOutput(ctx, c, jobSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to create delete job: %w", err)
		}

		logger.Completed("deletion job created")
		return job, nil
	*/
	return nil, nil
}

// JobStatusOptions holds all possible options for updating a ResticBackup's status.
type JobStatusOptions struct {
	Phase         string
	Message       string
	JobName       string
	IsJobFinished bool
	SnapshotID    string
	Size          string
}

// UpdateResticBackupStatus updates the status of a ResticBackup based on the provided options.
func UpdateResticBackupStatus(ctx context.Context, c client.Client, backup *backupv1alpha1.ResticBackup, options JobStatusOptions) (ctrl.Result, error) {
	backup.Status.Phase = options.Phase
	backup.Status.Error = options.Message

	if options.JobName != "" {
		backup.Status.BackupJobRef = &corev1.LocalObjectReference{Name: options.JobName}
	}
	if options.IsJobFinished && backup.Status.CompletionTime == nil {
		now := metav1.Now()
		backup.Status.CompletionTime = &now
	}
	if options.SnapshotID != "" {
		backup.Spec.SnapshotID = options.SnapshotID
		if err := c.Update(ctx, backup); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update backup spec with snapshotID: %w", err)
		}
	}
	if options.Size != "" {
		var size int64
		// In a real implementation, parse this more robustly.
		fmt.Sscanf(options.Size, "%d", &size)
		backup.Status.Size = size
	}

	if err := c.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update backup status: %w", err)
	}

	requeueTime := constants.DefaultRequeueInterval
	if options.Phase == "Failed" {
		requeueTime = 5 * time.Minute
	}
	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

func ShouldVerifyBackup(backup *backupv1alpha1.ResticBackup) bool {
	/*
		if backup.Status.LastVerified == nil {
			return true
		}
		// Verify daily
		return time.Since(backup.Status.LastVerified.Time) > 24*time.Hour
	*/
	return false
}

func CalculateNextVerificationTime(backup *backupv1alpha1.ResticBackup) time.Duration {
	/*
		if backup.Status.LastVerified == nil {
			return 5 * time.Minute
		}
		// Verify daily
		nextVerify := backup.Status.LastVerified.Time.Add(24 * time.Hour)
		timeUntilNext := time.Until(nextVerify)
		if timeUntilNext < 0 {
			return 5 * time.Minute
		}
		return timeUntilNext
	*/
	return 0
}

func AddPVCProtection(ctx context.Context, c client.Client, backup *backupv1alpha1.ResticBackup) error {
	/*
		// Get the PVC
		pvc := &corev1.PersistentVolumeClaim{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      backup.Spec.PVCRef.Name,
			Namespace: backup.Spec.PVCRef.Namespace,
		}, pvc); err != nil {
			return fmt.Errorf("failed to get PVC: %w", err)
		}

		// Add protection finalizer if not already present
		if !controllerutil.ContainsFinalizer(pvc, constants.PVCBackupFinalizer) {
			controllerutil.AddFinalizer(pvc, constants.PVCBackupFinalizer)
			if err := c.Update(ctx, pvc); err != nil {
				return fmt.Errorf("failed to add PVC protection finalizer: %w", err)
			}
		}

		return nil
	*/
	return nil
}

func RemovePVCProtection(ctx context.Context, c client.Client, backup *backupv1alpha1.ResticBackup) error {
	/*
		// Get the PVC
		pvc := &corev1.PersistentVolumeClaim{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      backup.Spec.PVCRef.Name,
			Namespace: backup.Spec.PVCRef.Namespace,
		}, pvc); err != nil {
			// PVC might already be deleted, which is fine
			return nil
		}

		// Remove protection finalizer if present
		if controllerutil.ContainsFinalizer(pvc, constants.PVCBackupFinalizer) {
			controllerutil.RemoveFinalizer(pvc, constants.PVCBackupFinalizer)
			if err := c.Update(ctx, pvc); err != nil {
				return fmt.Errorf("failed to remove PVC protection finalizer: %w", err)
			}
		}

		return nil
	*/
	return nil
}

// ResticRestore stubs

func HandleResticRestoreDeletion(ctx context.Context, c client.Client, scheme *runtime.Scheme, resticRestore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	/*
		logger := utils.LoggerFrom(ctx, "deletion").
			WithValues("restore", resticRestore.Name)

		if !controllerutil.ContainsFinalizer(resticRestore, constants.ResticRestoreFinalizer) {
			logger.Debug("Finalizer not found, skipping cleanup")
			return ctrl.Result{}, nil
		}

		// Clean up associated job if it exists
		if resticRestore.Status.JobRef != nil {
			if err := utils.CleanupJob(ctx, c, resticRestore.Status.JobRef.Name, resticRestore.Namespace); err != nil {
				logger.WithValues("error", err).Debug("Failed to cleanup job")
			} else {
				logger.Debug("Job cleanup completed")
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(resticRestore, constants.ResticRestoreFinalizer)
		if err := c.Update(ctx, resticRestore); err != nil {
			logger.Failed("remove finalizer", err)
			return ctrl.Result{}, err
		}

		logger.Completed("finalizer cleanup completed")
		return ctrl.Result{}, nil
	*/
	return ctrl.Result{}, nil
}

func HandleRestorePending(ctx context.Context, c client.Client, scheme *runtime.Scheme, resticRestore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "restore-pending").
		WithValues("restore", resticRestore.Name).
		WithLogging(ctx, "handlePending", func(ctx context.Context) (ctrl.Result, error) {
			// Initialize status if empty
			if resticRestore.Status.Phase == "" {
				resticRestore.Status.Phase = "Pending"
				resticRestore.Status.StartTime = &metav1.Time{Time: time.Now()}
				if err := c.Status().Update(ctx, resticRestore); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			// Look up the ResticBackup by name
			resticBackup := &backupv1alpha1.ResticBackup{}
			if err := c.Get(ctx, client.ObjectKey{
				Name:      resticRestore.Spec.BackupName,
				Namespace: resticRestore.Namespace,
			}, resticBackup); err != nil {
				return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
					Phase:         string(constants.JobPhaseFailed),
					Message:       fmt.Sprintf("Failed to get ResticBackup %s: %v", resticRestore.Spec.BackupName, err),
					IsJobFinished: true,
				})
			}
			// Get the backup ID and source PVC from the backup
			backupID := resticBackup.Status.BackupID
			if backupID == "" {
				return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
					Phase:         string(constants.JobPhaseFailed),
					Message:       fmt.Sprintf("Backup ID not found in ResticBackup %s", resticRestore.Spec.BackupName),
					IsJobFinished: true,
				})
			}
			// Get or create the target PVC
			targetPVC := &corev1.PersistentVolumeClaim{}
			err := c.Get(ctx, types.NamespacedName{
				Name:      resticRestore.Spec.TargetPVC,
				Namespace: resticRestore.Namespace,
			}, targetPVC)
			if err != nil {
				if !errors.IsNotFound(err) {
					return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
						Phase:         string(constants.JobPhaseFailed),
						Message:       fmt.Sprintf("Failed to get target PVC: %v", err),
						IsJobFinished: true,
					})
				}
				// PVC doesn't exist - check if we should create it
				if !resticRestore.Spec.CreateTargetPVC {
					return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
						Phase:         string(constants.JobPhaseFailed),
						Message:       fmt.Sprintf("Target PVC %s not found and CreateTargetPVC is false", resticRestore.Spec.TargetPVC),
						IsJobFinished: true,
					})
				}
				// Get the source PVC to copy its spec
				sourcePVC := &corev1.PersistentVolumeClaim{}
				if err := c.Get(ctx, types.NamespacedName{
					Name:      resticBackup.Spec.PVCRef.Name,
					Namespace: resticBackup.Namespace,
				}, sourcePVC); err != nil {
					return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
						Phase:         string(constants.JobPhaseFailed),
						Message:       fmt.Sprintf("Failed to get source PVC from backup: %v", err),
						IsJobFinished: true,
					})
				}
				// Create new PVC based on source or template
				newPVC := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resticRestore.Spec.TargetPVC,
						Namespace: resticRestore.Namespace,
					},
				}
				if resticRestore.Spec.TargetPVCTemplate != nil {
					newPVC.Spec = resticRestore.Spec.TargetPVCTemplate.Spec
				} else {
					newPVC.Spec = sourcePVC.Spec
				}
				// Create the PVC
				if err := c.Create(ctx, newPVC); err != nil {
					return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
						Phase:         string(constants.JobPhaseFailed),
						Message:       fmt.Sprintf("Failed to create target PVC: %v", err),
						IsJobFinished: true,
					})
				}
				targetPVC = newPVC
			}
			// Add labels/annotations to track restore source
			if targetPVC.Labels == nil {
				targetPVC.Labels = make(map[string]string)
			}
			if targetPVC.Annotations == nil {
				targetPVC.Annotations = make(map[string]string)
			}
			targetPVC.Labels["backup.autorestore.com/restored-from"] = resticBackup.Spec.PVCRef.Name
			targetPVC.Labels["backup.autorestore.com/backup-name"] = resticRestore.Spec.BackupName
			if err := c.Update(ctx, targetPVC); err != nil {
				return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
					Phase:         string(constants.JobPhaseFailed),
					Message:       fmt.Sprintf("Failed to update target PVC labels: %v", err),
					IsJobFinished: true,
				})
			}
			// Create restore job with ConfigMap output
			job, _, err := utils.CreateRestoreJobWithOutput(ctx, c, *targetPVC, resticRestore.Spec.BackupTarget, backupID, resticRestore)
			if err != nil {
				return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
					Phase:         string(constants.JobPhaseFailed),
					Message:       fmt.Sprintf("Failed to create restore job: %v", err),
					IsJobFinished: true,
				})
			}
			// Update status to Running
			return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
				Phase:   string(constants.JobPhaseRunning),
				JobName: job.Name,
			})
		})
}

func HandleRestoreRunning(ctx context.Context, c client.Client, scheme *runtime.Scheme, resticRestore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "restore-running").
		WithValues("restore", resticRestore.Name).
		WithLogging(ctx, "handleRunning", func(ctx context.Context) (ctrl.Result, error) {
			if resticRestore.Status.JobRef == nil {
				return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
					Phase:         string(constants.JobPhaseFailed),
					Message:       "Job reference is missing",
					IsJobFinished: true,
				})
			}
			// Read job output from ConfigMap
			jobOutput, err := utils.ReadJobOutput(ctx, c, resticRestore.Status.JobRef.Name, resticRestore.Namespace)
			if err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
			return utils.ProcessSteps(
				utils.Step{
					Condition: jobOutput.IsJobCompleted,
					Action: func() (ctrl.Result, error) {
						return processIfRestoreJobCompleted(ctx, c, resticRestore, jobOutput)
					},
				},
				utils.Step{
					Condition: jobOutput.IsJobFailed,
					Action: func() (ctrl.Result, error) {
						return processIfRestoreJobFailed(ctx, c, resticRestore, jobOutput)
					},
				},
				utils.Step{
					Condition: func() bool { return true }, // Default case for running
					Action: func() (ctrl.Result, error) {
						return processIfRestoreJobRunning(ctx, c, resticRestore, jobOutput)
					},
				},
			)
		})
}

func processIfRestoreJobCompleted(ctx context.Context, c client.Client, resticRestore *backupv1alpha1.ResticRestore, jobOutput *utils.JobOutput) (ctrl.Result, error) {
	return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
		Phase:         string(constants.JobPhaseCompleted),
		IsJobFinished: true,
	})
}

func processIfRestoreJobFailed(ctx context.Context, c client.Client, resticRestore *backupv1alpha1.ResticRestore, jobOutput *utils.JobOutput) (ctrl.Result, error) {
	return UpdateResticRestoreStatus(ctx, c, resticRestore, RestoreJobStatusOptions{
		Phase:         string(constants.JobPhaseFailed),
		Message:       fmt.Sprintf("Restore job failed: %s", jobOutput.Error),
		IsJobFinished: true,
	})
}

func processIfRestoreJobRunning(ctx context.Context, c client.Client, resticRestore *backupv1alpha1.ResticRestore, jobOutput *utils.JobOutput) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// GetRepositoryForBackup fetches the ResticRepository associated with a ResticBackup.
func GetRepositoryForBackup(ctx context.Context, c client.Client, backup *backupv1alpha1.ResticBackup) (*backupv1alpha1.ResticRepository, error) {
	/*
		repository := &backupv1alpha1.ResticRepository{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      backup.Spec.RepositoryRef.Name,
			Namespace: backup.Namespace,
		}, repository); err != nil {
			return nil, fmt.Errorf("repository %s not found: %w", backup.Spec.RepositoryRef.Name, err)
		}
		return repository, nil
	*/
	return &backupv1alpha1.ResticRepository{
		Status: backupv1alpha1.ResticRepositoryStatus{
			Initialized: true,
		},
	}, nil // Return an initialized repo to allow processing to continue
}

func HandleBackupDeletion(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "backup-deletion").
		WithValues("backup", backup.Name, "snapshotID", backup.Spec.SnapshotID).
		WithLogging(ctx, "handleDeletion", func(ctx context.Context) (ctrl.Result, error) {
			// Get the repository to delete the snapshot from
			repository := &backupv1alpha1.ResticRepository{}
			if err := c.Get(ctx, client.ObjectKey{
				Name:      backup.Spec.RepositoryRef.Name,
				Namespace: backup.Namespace,
			}, repository); err != nil {
				if errors.IsNotFound(err) {
					// Repository is gone, just remove finalizer
					utils.LoggerFrom(ctx, "backup-deletion").
						WithValues("backup", backup.Name, "snapshotID", backup.Spec.SnapshotID).
						Debug("Repository not found, removing finalizer")
				} else {
					return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
				}
			} else {
				// Create deletion job
				job, err := CreateDeletionJob(ctx, c, scheme, backup, repository)
				if err != nil {
					return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
				}

				// Update status with deletion job reference
				backup.Status.DeletionJobRef = &corev1.LocalObjectReference{Name: job.Name}
				if err := c.Status().Update(ctx, backup); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			// Remove the finalizer
			controllerutil.RemoveFinalizer(backup, constants.ResticBackupFinalizer)
			if err := c.Update(ctx, backup); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		})
}

// HandleBackupPending handles a ResticBackup in the Pending phase.
func HandleBackupPending(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "restic-backup").
		WithValues("name", backup.Name, "namespace", backup.Namespace).
		WithLogging(ctx, "handlePendingBackup", func(ctx context.Context) (ctrl.Result, error) {
			// Get repository
			repository, err := GetRepositoryForBackup(ctx, c, backup)
			if err != nil {
				if errors.IsNotFound(err) {
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
				return ctrl.Result{}, err
			}

			// Check if repository is ready
			if !repository.Status.Initialized {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			// Create backup job
			job, err := CreateBackupJob(ctx, c, scheme, backup, repository)
			if err != nil {
				return ctrl.Result{}, err
			}

			// Update status with job reference and phase
			return UpdateResticBackupStatus(ctx, c, backup, JobStatusOptions{
				Phase:         "Running",
				JobName:       job.Name,
				Message:       fmt.Sprintf("Backup job %s created", job.Name),
				IsJobFinished: false,
			})
		})
}

// HandleBackupRunning handles a ResticBackup in the Running phase.
func HandleBackupRunning(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "restic-backup").
		WithValues("name", backup.Name, "namespace", backup.Namespace).
		WithLogging(ctx, "handleRunningBackup", func(ctx context.Context) (ctrl.Result, error) {
			if backup.Status.BackupJobRef == nil {
				return UpdateResticBackupStatus(ctx, c, backup, JobStatusOptions{
					Phase:   "Failed",
					Message: "Backup job reference not found",
				})
			}
			job := &batchv1.Job{}
			if err := c.Get(ctx, client.ObjectKey{Name: backup.Status.BackupJobRef.Name, Namespace: backup.Namespace}, job); err != nil {
				if errors.IsNotFound(err) {
					// Job is gone, assume it failed and move to failed state
					return UpdateResticBackupStatus(ctx, c, backup, JobStatusOptions{
						Phase:         "Failed",
						Message:       "Backup job not found, assuming failure",
						IsJobFinished: true,
					})
				}
				return ctrl.Result{}, err
			}

			// Check job status and update backup status accordingly
			jobOutput, err := utils.ReadJobOutput(ctx, c, backup.Status.BackupJobRef.Name, backup.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}

			return utils.ProcessSteps(
				utils.Step{
					Condition: jobOutput.IsJobFailed,
					Action: func() (ctrl.Result, error) {
						return UpdateResticBackupStatus(ctx, c, backup, JobStatusOptions{
							Phase:         string(constants.JobPhaseFailed),
							Message:       "Backup job failed",
							IsJobFinished: true,
						})
					},
				},
				utils.Step{
					Condition: jobOutput.IsJobCompleted,
					Action: func() (ctrl.Result, error) {
						return UpdateResticBackupStatus(ctx, c, backup, JobStatusOptions{
							Phase:         string(constants.JobPhaseCompleted),
							Message:       "Backup completed successfully",
							IsJobFinished: true,
							SnapshotID:    jobOutput.SnapshotID,
							Size:          jobOutput.Size,
						})
					},
				},
				utils.Step{
					Condition: func() bool { return true },
					Action: func() (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
					},
				},
			)
		})
}

// HandleBackupReady handles a ResticBackup in the Ready phase.
func HandleBackupReady(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "restic-backup").
		WithValues("name", backup.Name, "namespace", backup.Namespace).
		WithLogging(ctx, "handleReadyBackup", func(ctx context.Context) (ctrl.Result, error) {
			if !ShouldVerifyBackup(backup) {
				return ctrl.Result{}, nil
			}

			// Get the repository for the backup
			repository, err := GetRepositoryForBackup(ctx, c, backup)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get repository for backup verification: %w", err)
			}

			// Create verification job
			job, err := CreateVerificationJob(ctx, c, scheme, backup, repository)
			if err != nil {
				return ctrl.Result{}, err
			}

			// Update status with verification job reference
			backup.Status.VerificationJobRef = &corev1.LocalObjectReference{Name: job.Name}
			now := metav1.Now()
			backup.Status.LastVerified = &now
			if err := c.Status().Update(ctx, backup); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		})
}

// HandleBackupFailed handles a ResticBackup in the Failed phase.
func HandleBackupFailed(ctx context.Context, c client.Client, scheme *runtime.Scheme, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "restic-backup").
		WithValues("name", backup.Name, "namespace", backup.Namespace).
		WithLogging(ctx, "handleFailedBackup", func(ctx context.Context) (ctrl.Result, error) {
			// Failed backups are terminal. No action needed.
			// They will be cleaned up by retention policy eventually
			return ctrl.Result{}, nil
		})
}

// HandleRepoInitialization handles a ResticRepository in the initialization phase.
func HandleRepoInitialization(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "repo-init").
		WithValues("repository", repository.Name).
		WithLogging(ctx, "handleInitialization", func(ctx context.Context) (ctrl.Result, error) {
			// Check if repository exists first
			exists, jobInProgress, err := CheckRepositoryExists(ctx, c, scheme, repository)
			if err != nil {
				return UpdateRepoStatus(ctx, c, repository, "Failed", fmt.Sprintf("Failed to check repository: %v", err), false)
			}

			if jobInProgress {
				// Check job in progress, requeue
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			if exists {
				// Repository exists, collect stats
				stats, err := CollectRepositoryStats(ctx, c, repository.Namespace, repository.Spec.Repository, repository.Spec.Password, repository.Spec.Env, repository.Spec.Image)
				if err != nil {
					return UpdateRepoStatus(ctx, c, repository, "Failed", fmt.Sprintf("Failed to collect repository stats: %v", err), false)
				}

				// Update repository status with stats
				repository.Status.Stats = stats
				repository.Status.Phase = "Ready"
				repository.Status.Initialized = true
				repository.Status.InitializedTime = &metav1.Time{Time: time.Now()}
				repository.Status.LastChecked = &metav1.Time{Time: time.Now()}
				repository.Status.Error = ""

				if err := c.Status().Update(ctx, repository); err != nil {
					return ctrl.Result{}, err
				}

				return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
			}

			// Repository doesn't exist, check if we should auto-initialize
			if !repository.Spec.AutoInit {
				return UpdateRepoStatus(ctx, c, repository, "Failed", "Repository doesn't exist and auto-init is disabled", false)
			}

			// Start initialization
			// In a real implementation, you would create a job and update status to "Initializing"
			// For now, we'll assume it initializes instantly for simplicity of the stub
			return UpdateRepoStatus(ctx, c, repository, "Ready", "", true)
		})
}

// HandleRepoInitializationStatus handles a ResticRepository in the Initializing phase.
func HandleRepoInitializationStatus(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "repo-init-status").
		WithValues("repository", repository.Name).
		WithLogging(ctx, "handleInitializationStatus", func(ctx context.Context) (ctrl.Result, error) {
			// Check if repository is now initialized
			exists, jobInProgress, err := CheckRepositoryExists(ctx, c, scheme, repository)
			if err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			if jobInProgress {
				// Check job in progress, requeue
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			if !exists {
				// Repository still not initialized
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			// Repository is now ready
			return UpdateRepoStatus(ctx, c, repository, "Ready", "", true)
		})
}

// HandleRepoMaintenance handles a ResticRepository in the Ready phase.
func HandleRepoMaintenance(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository, cronParser cron.Parser) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "repo-maintenance").
		WithValues("repository", repository.Name).
		WithLogging(ctx, "handleMaintenance", func(ctx context.Context) (ctrl.Result, error) {
			// Update repository stats periodically
			if ShouldUpdateStats(repository) {
				if err := UpdateRepositoryStats(ctx, c, repository); err != nil {
					// Log error but don't fail reconciliation
				}
			}

			// Perform daily snapshot verification
			if ShouldVerifySnapshots(repository, cronParser) {
				if err := VerifyAllSnapshots(ctx, c, scheme, repository); err != nil {
					// Log error but don't fail reconciliation
				}
			}

			// Check if maintenance is due
			if ShouldRunMaintenance(repository) {
				return RunMaintenance(ctx, c, repository)
			}

			// Schedule next check
			nextCheck := CalculateNextMaintenanceTime(repository, cronParser)
			timeUntilNext := time.Until(nextCheck)
			if timeUntilNext > time.Hour {
				timeUntilNext = time.Hour // Check every hour at most
			}

			return ctrl.Result{RequeueAfter: timeUntilNext}, nil
		})
}

// HandleRepoFailed handles a ResticRepository in the Failed phase.
func HandleRepoFailed(ctx context.Context, c client.Client, scheme *runtime.Scheme, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "repo-failed").
		WithValues("repository", repository.Name).
		WithLogging(ctx, "handleFailedRepository", func(ctx context.Context) (ctrl.Result, error) {
			// Try to check repository again
			exists, jobInProgress, err := CheckRepositoryExists(ctx, c, scheme, repository)
			if err != nil {
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}

			if jobInProgress {
				// Check job in progress, requeue
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			if exists {
				return UpdateRepoStatus(ctx, c, repository, "Ready", "", true)
			}

			// Still failed, retry later
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		})
}

// HandleRepoDeletion handles a ResticRepository that is being deleted.
func HandleRepoDeletion(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "repo-deletion").
		WithValues("repository", repository.Name).
		WithLogging(ctx, "handleDeletion", func(ctx context.Context) (ctrl.Result, error) {
			controllerutil.RemoveFinalizer(repository, constants.ResticRepositoryFinalizer)
			if err := c.Update(ctx, repository); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		})
}

// UpdateRepoStatus updates the status of the repository.
func UpdateRepoStatus(ctx context.Context, c client.Client, repository *backupv1alpha1.ResticRepository, phase, message string, initialized bool) (ctrl.Result, error) {
	repository.Status.Phase = phase
	repository.Status.Error = message
	repository.Status.Initialized = initialized
	repository.Status.LastChecked = &metav1.Time{Time: time.Now()}

	if err := c.Status().Update(ctx, repository); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// RestoreJobStatusOptions holds all possible options for updating a ResticRestore's status.
type RestoreJobStatusOptions struct {
	Phase         string
	Message       string
	JobName       string
	IsJobFinished bool
}

// UpdateResticRestoreStatus updates the status of a ResticRestore.
func UpdateResticRestoreStatus(ctx context.Context, c client.Client, restore *backupv1alpha1.ResticRestore, options RestoreJobStatusOptions) (ctrl.Result, error) {
	restore.Status.Phase = options.Phase
	restore.Status.Error = options.Message

	if options.JobName != "" {
		restore.Status.JobRef = &corev1.LocalObjectReference{Name: options.JobName}
	}
	if options.IsJobFinished && restore.Status.CompletionTime == nil {
		now := metav1.Now()
		restore.Status.CompletionTime = &now
	}

	if err := c.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update restore status: %w", err)
	}

	requeueTime := constants.DefaultRequeueInterval
	if options.Phase == "Failed" {
		requeueTime = 5 * time.Minute
	}
	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

func HandleBackupConfigDeletion(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig) (ctrl.Result, error) {
	return utils.LoggerFrom(ctx, "backupconfig-reconciler").
		WithValues("name", backupConfig.Name, "namespace", backupConfig.Namespace).
		WithLogging(ctx, "handleBackupConfigDeletion", func(ctx context.Context) (ctrl.Result, error) {
			if controllerutil.ContainsFinalizer(backupConfig, constants.BackupConfigFinalizer) {
				// Your cleanup logic here
				// For example, delete associated ResticRepository objects

				// Remove finalizer
				controllerutil.RemoveFinalizer(backupConfig, constants.BackupConfigFinalizer)
				if err := c.Update(ctx, backupConfig); err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, nil
		})
}

func UpdateManagedPVCsStatus(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig, pvcs []corev1.PersistentVolumeClaim) error {
	_, err := utils.LoggerFrom(ctx, "backupconfig-reconciler").
		WithValues("name", backupConfig.Name, "namespace", backupConfig.Namespace).
		WithLogging(ctx, "updateManagedPVCsStatus", func(ctx context.Context) (ctrl.Result, error) {
			pvcNames := make([]string, len(pvcs))
			for i, pvc := range pvcs {
				pvcNames[i] = pvc.Name
			}
			backupConfig.Status.ManagedPVCs = pvcNames
			err := c.Status().Update(ctx, backupConfig)
			return ctrl.Result{}, err
		})
	return err
}
