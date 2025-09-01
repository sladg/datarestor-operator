package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleBackupConfigDeletion handles the deletion logic for a BackupConfig.
func HandleBackupConfigDeletion(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig) (ctrl.Result, error) {
	log := deps.Logger.Named("backupconfig-deletion").With("name", backupConfig.Name)

	log.Info("start")

	if controllerutil.ContainsFinalizer(backupConfig, constants.BackupConfigFinalizer) {
		// Clean up any resources created by this BackupConfig
		if err := cleanupBackupConfigResources(ctx, deps, backupConfig); err != nil {
			log.Errorw("Failed to cleanup resources", "error", err)
			// Don't return error to avoid blocking deletion
		}

		// Release all managed PVCs
		pvcList := &corev1.PersistentVolumeClaimList{}
		if err := deps.Client.List(ctx, pvcList, client.InNamespace(backupConfig.Namespace)); err != nil {
			log.Errorw("Failed to list PVCs for cleanup", "error", err)
			// Continue with cleanup
		} else {
			for i := range pvcList.Items {
				pvc := &pvcList.Items[i]
				managedKey := fmt.Sprintf("%s/%s", backupConfig.Namespace, backupConfig.Name)
				if managedBy, ok := pvc.Labels[constants.LabelManagedBy]; ok && managedBy == managedKey {
					delete(pvc.Labels, constants.LabelManagedBy)
					if err := deps.Client.Update(ctx, pvc); err != nil {
						log.Errorw("remove managed-by label failed", "error", err)
					}
				}
			}
		}

		// Remove finalizer
		log.Debugw("Removing finalizer", "name", backupConfig.Name)
		controllerutil.RemoveFinalizer(backupConfig, constants.BackupConfigFinalizer)
		if err := deps.Client.Update(ctx, backupConfig); err != nil {
			log.Errorw("remove finalizer failed", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	log.Info("end")
	return ctrl.Result{}, nil
}

// cleanupBackupConfigResources cleans up resources created by the BackupConfig
func cleanupBackupConfigResources(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig) error {
	// Note: BackupJobs and RestoreJobs are automatically cleaned up by Kubernetes
	// due to owner references, and their finalizers handle their own cleanup
	//
	// Kubernetes Jobs cleanup is handled by individual BackupJob/RestoreJob finalizers
	// Restic cleanup is also handled by individual BackupJob finalizers
	// This ensures each backup and its associated resources are properly cleaned up

	return nil
}

// EnsureResticRepositories ensures that ResticRepository CRDs exist for all backup targets.
func EnsureResticRepositories(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig) error {
	log := deps.Logger.Named("ensure-restic-repositories")
	log.Info("Ensuring ResticRepository CRDs exist for all backup targets")

	for _, target := range backupConfig.Spec.BackupTargets {
		repoName := fmt.Sprintf("%s-%s", backupConfig.Name, target.Name)
		repository := &backupv1alpha1.ResticRepository{}
		err := deps.Client.Get(ctx, types.NamespacedName{Name: repoName, Namespace: backupConfig.Namespace}, repository)
		if err == nil {
			// Repository exists, ensure ownership
			if !isOwnedByBackupConfig(repository, backupConfig) {
				if err := controllerutil.SetControllerReference(backupConfig, repository, deps.Scheme); err == nil {
					_ = deps.Client.Update(ctx, repository)
				}
			}
			continue
		}
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check repository existence: %w", err)
		}

		// Create new repository
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
				Target: target.Restic.Target,
				Env:    target.Restic.Env,
				Args:   target.Restic.Args,
				Image:  target.Restic.Image,
			},
		}
		if err := controllerutil.SetControllerReference(backupConfig, repo, deps.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
		if err := deps.Client.Create(ctx, repo); err != nil {
			return fmt.Errorf("failed to create repository: %w", err)
		}
	}

	return nil
}

// isOwnedByBackupConfig checks if a ResticRepository is owned by the given BackupConfig.
func isOwnedByBackupConfig(repo *backupv1alpha1.ResticRepository, backupConfig *backupv1alpha1.BackupConfig) bool {
	for _, ownerRef := range repo.OwnerReferences {
		if ownerRef.Kind == "BackupConfig" &&
			ownerRef.Name == backupConfig.Name &&
			ownerRef.UID == backupConfig.UID {
			return true
		}
	}
	return false
}

// HandleAutoRestore handles the auto-restore logic for newly claimed PVCs.
func HandleAutoRestore(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, managedPVCs []string) error {
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

		// Check if the PVC is newly created and doesn't have a finalizer from us yet.
		// A more robust check might involve looking at creation timestamp or a status annotation.
		if !controllerutil.ContainsFinalizer(pvc, constants.PVCRestoreFinalizer) {
			log.Infow("New PVC detected, creating ResticRestore for auto-restore", "pvc", pvc.Name)
			if err := createResticRestore(ctx, deps, backupConfig, pvc); err != nil {
				log.Errorw("Failed to create ResticRestore for new PVC", "pvc", pvc.Name, "error", err)
				// Continue to next PVC, don't block everything
			}
			// Add finalizer to mark it as processed by us for restore
			controllerutil.AddFinalizer(pvc, constants.PVCRestoreFinalizer)
			if err := deps.Client.Update(ctx, pvc); err != nil {
				log.Errorw("Failed to add finalizer to PVC after auto-restore", "pvc", pvc.Name, "error", err)
			}
		}
	}
	return nil
}

// HandleScheduledBackups handles scheduled backups on a per-target basis.
func HandleScheduledBackups(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, managedPVCs []string, cronParser *cron.Parser) error {
	log := deps.Logger.Named("handle-scheduled-backups")

	// Get existing ResticBackups to check their status
	existingBackups := &backupv1alpha1.ResticBackupList{}
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
					if backup.Status.CompletionTime != nil && backup.Status.Phase == backupv1alpha1.PhaseCompleted {
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

				// Create a backup with a predictable name
				backupName := fmt.Sprintf("%s-%s-%s-%d",
					backupConfig.Name,
					target.Name,
					pvc.Name,
					time.Now().Unix())

				if err := createResticBackup(ctx, deps, backupConfig, pvc, backupName, *target); err != nil {
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
