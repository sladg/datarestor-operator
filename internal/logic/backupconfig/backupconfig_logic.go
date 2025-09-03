package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleBackupConfigDeletion handles the deletion logic for a BackupConfig.
func HandleBackupConfigDeletion(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) (ctrl.Result, error) {
	log := deps.Logger.Named("backupconfig-deletion").With("name", backupConfig.Name)

	log.Info("start")

	if utils.ContainsFinalizer(backupConfig, constants.BackupConfigFinalizer) {
		// Clean up any resources created by this BackupConfig
		// NOTE: not needed as all resources are owned by the BackupConfig

		// PVCs are kept in place.

		// Remove finalizer
		log.Debugw("Removing finalizer", "name", backupConfig.Name)
		if err := utils.RemoveFinalizer(ctx, deps, backupConfig, constants.BackupConfigFinalizer); err != nil {
			log.Errorw("remove finalizer failed", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	log.Info("end")
	return ctrl.Result{}, nil
}

// EnsureResticRepositories ensures that ResticRepository CRDs exist for all backup targets.
func EnsureResticRepositories(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) error {
	log := deps.Logger.Named("ensure-restic-repositories")
	log.Info("Ensuring ResticRepository CRDs exist for all backup targets")

	for _, target := range backupConfig.Spec.BackupTargets {
		// Use selector-based matching for ResticRepository
		repoSelector := v1.Selector{
			Names:      []string{target.Repository.Name},
			Namespaces: []string{backupConfig.Namespace},
		}
		repoList := &v1.ResticRepositoryList{}
		repos, err := utils.FindMatchingResources[*v1.ResticRepository](ctx, deps, []v1.Selector{repoSelector}, repoList)
		if err != nil {
			return fmt.Errorf("failed to find repository: %w", err)
		}
		if len(repos) == 0 {
			// Not found, skip
			continue
		}
		for _, repository := range repos {
			if !isOwnedByBackupConfig(repository, backupConfig) {
				if err := controllerutil.SetControllerReference(backupConfig, repository, deps.Scheme); err == nil {
					_ = deps.Client.Update(ctx, repository)
				}
			}
		}
	}

	return nil
}

// HandleAutoRestore handles the auto-restore logic for newly claimed PVCs.
func HandleAutoRestore(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, managedPVCs []string) error {
	log := deps.Logger.Named("handle-auto-restore")
	if !backupConfig.Spec.AutoRestore {
		return nil
	}

	for _, pvcName := range managedPVCs {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := deps.Client.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: backupConfig.Namespace}, pvc); err != nil {
			log.Errorw("Failed to get PVC for auto-restore check", "pvc", pvcName, "error", err)
			continue
		}

		// If the PVC already has the restore finalizer, it's either being restored or has been restored.
		if controllerutil.ContainsFinalizer(pvc, constants.PVCRestoreFinalizer) {
			log.Debugw("PVC already has restore finalizer, skipping auto-restore", "pvc", pvc.Name)
			continue
		}

		log.Infow("New PVC detected, creating ResticRestore for auto-restore", "pvc", pvc.Name)

		// Get the ResticRepository
		// This logic assumes the first target is the one to restore from, which might need refinement.
		if len(backupConfig.Spec.BackupTargets) > 0 {
			target := backupConfig.Spec.BackupTargets[0]
			repo, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, backupConfig.Namespace, target.Repository.Name, log)
			if err != nil {
				log.Errorw("Failed to get ResticRepository for auto-restore", "repo", target.Repository.Name, "error", err)
				continue
			}
			if err := createResticRestore(ctx, deps, backupConfig, pvc, *repo); err != nil {
				log.Errorw("Failed to create ResticRestore for new PVC", "pvc", pvc.Name, "error", err)
				// Continue to next PVC, don't block everything
			}
		} else {
			log.Warnw("No backup targets defined for auto-restore", "backupconfig", backupConfig.Name)
		}
		// Add finalizer to mark it as processed by us for restore
		controllerutil.AddFinalizer(pvc, constants.PVCRestoreFinalizer)
		if err := deps.Client.Update(ctx, pvc); err != nil {
			log.Errorw("Failed to add finalizer to PVC after auto-restore", "pvc", pvc.Name, "error", err)
		}
	}
	return nil
}

// HandleScheduledBackups handles scheduled backups on a per-target basis.
func HandleScheduledBackups(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, managedPVCs []string, cronParser *cron.Parser) error {
	log := deps.Logger.Named("handle-scheduled-backups")

	// Get existing ResticBackups to check their status
	existingBackups := &v1.ResticBackupList{}
	if err := deps.Client.List(ctx, existingBackups, client.InNamespace(backupConfig.Namespace), client.MatchingLabels{
		"backup-config": backupConfig.Name,
	}); err != nil {
		log.Errorw("Failed to list existing backups", "error", err)
		return err
	}

	// Process each backup target
	for i := range backupConfig.Spec.BackupTargets {
		target := &backupConfig.Spec.BackupTargets[i]
		if target.BackupSchedule == "" {
			continue // Skip targets without schedule
		}

		// Process each PVC for this target
		for _, pvcName := range managedPVCs {
			pvc := &corev1.PersistentVolumeClaim{}
			if err := deps.Client.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: backupConfig.Namespace}, pvc); err != nil {
				log.Errorw("Failed to get PVC", "pvc", pvcName, "error", err)
				continue
			}

			// Find the latest successful backup for this PVC and target
			var lastBackupTime *time.Time
			for _, backup := range existingBackups.Items {
				if backup.Labels["pvc"] == pvc.Name && backup.Labels["backup-target"] == target.Name {
					if backup.Status.CompletionTime != nil && backup.Status.Phase == v1.PhaseCompleted {
						t := backup.Status.CompletionTime.Time
						if lastBackupTime == nil || t.After(*lastBackupTime) {
							lastBackupTime = &t
						}
					}
				}
			}

			// Check if we should create a new backup
			if utils.ShouldPerformBackup(target.BackupSchedule, lastBackupTime) {
				log.Infow("Creating scheduled backup",
					"target", target.Name,
					"pvc", pvc.Name,
					"lastBackup", lastBackupTime)

				repo, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, backupConfig.Namespace, target.Repository.Name, log)
				if err != nil {
					log.Errorw("Failed to get ResticRepository for scheduled backup", "repo", target.Repository.Name, "error", err)
					continue
				}

				// Create a backup with a predictable name
				backupName := fmt.Sprintf("%s-%s-%s-%d",
					backupConfig.Name,
					target.Name,
					pvc.Name,
					time.Now().Unix())

				if err := createResticBackup(ctx, deps, backupConfig, pvc, backupName, *target, *repo); err != nil {
					log.Errorw("Failed to create scheduled backup",
						"pvc", pvc.Name,
						"target", target.Name,
						"error", err)
					continue
				}
			}
		}
	}

	return nil
}

// HandleSchedule processes scheduled backups for all PVCs managed by the BackupConfig.
// For each target with a backup schedule:
//  1. Finds all managed PVCs
//  2. Checks the last successful backup for each PVC
//  3. Creates new backups when needed based on the schedule
//
// The function ensures each PVC is backed up according to its target's schedule,
// tracking the last successful backup to prevent unnecessary operations.
func HandleSchedule(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, managedPVCs []string) error {
	log := deps.Logger.Named("handle-schedule")

	// Find all managed PVCs once since we'll need them for all targets
	selector := v1.Selector{
		Names:      managedPVCs,
		Namespaces: []string{backupConfig.Namespace},
	}

	pvcs, err := utils.FindMatchingPVCs(ctx, deps, &selector)
	if err != nil {
		log.Errorw("Failed to find managed PVCs", "error", err)
		return err
	}

	// Get existing ResticBackups to check their status using selector
	backupSelector := v1.Selector{
		Namespaces: []string{backupConfig.Namespace},
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"backup-config": backupConfig.Name,
			},
		},
	}

	backupList := &v1.ResticBackupList{}
	existingBackups, err := utils.FindMatchingResources[*v1.ResticBackup](ctx, deps, []v1.Selector{backupSelector}, backupList)
	if err != nil {
		log.Errorw("Failed to find existing backups", "error", err)
		return err
	}

	// Process each backup target that has a schedule
	for i := range backupConfig.Spec.BackupTargets {
		target := &backupConfig.Spec.BackupTargets[i]
		if target.BackupSchedule == "" {
			continue // Skip targets without schedule
		}

		// Process each PVC for this target
		for _, pvc := range pvcs {
			// Find the latest successful backup for this PVC and target
			var lastBackupTime *time.Time
			for _, backup := range existingBackups {
				if backup.Labels["pvc"] == pvc.Name && backup.Labels["backup-target"] == target.Name {
					if backup.Status.CompletionTime != nil && backup.Status.Phase == v1.PhaseCompleted {
						t := backup.Status.CompletionTime.Time
						if lastBackupTime == nil || t.After(*lastBackupTime) {
							lastBackupTime = &t
						}
					}
				}
			}

			// Create new backup if needed based on schedule
			if utils.ShouldPerformBackup(target.BackupSchedule, lastBackupTime) {
				log.Infow("Creating scheduled backup",
					"target", target.Name,
					"pvc", pvc.Name,
					"lastBackup", lastBackupTime)

				repo, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, backupConfig.Namespace, target.Repository.Name, log)
				if err != nil {
					log.Errorw("Failed to get ResticRepository for scheduled backup", "repo", target.Repository.Name, "error", err)
					continue
				}
				backupName := getScheduledBackupName(backupConfig.Name, target.Name, pvc.Name)
				if err := createResticBackup(ctx, deps, backupConfig, &pvc, backupName, *target, *repo); err != nil {
					log.Errorw("Failed to create scheduled backup",
						"pvc", pvc.Name,
						"target", target.Name,
						"error", err)
					continue
				}
			}
		}
	}

	return nil
}
