package logic

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DeleteResticDataAnnotation indicates whether to delete the underlying restic data
	DeleteResticDataAnnotation = "backup.autorestore-backup-operator.com/delete-restic-data"
)

// HandleBackupDeletion handles the deletion logic for a ResticBackup.
// It ensures:
// 1. No active restores are using this backup
// 2. Optionally deletes restic data if explicitly requested
// 3. Handles cleanup of any resources owned by this backup
func HandleBackupDeletion(ctx context.Context, deps *utils.Dependencies, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-deletion")

	// Check deletion state
	state := constants.DeletionState(backup.Annotations[constants.AnnotationDeletionState])
	if state == "" {
		state = constants.DeletionStateCleanup
		// Set initial state
		if err := utils.AnnotateResources(ctx, deps, []client.Object{backup}, map[string]string{
			constants.AnnotationDeletionState: string(state),
		}); err != nil {
			log.Errorw("Failed to set initial deletion state", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}
	}

	switch state {
	case constants.DeletionStateCleanup:
		// Check for active restores using this backup
		restores, err := utils.FindRestoresByBackupName(ctx, deps, backup.Namespace, backup.Name)
		if err != nil {
			log.Errorw("Failed to list restores", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}

		for _, restore := range restores {
			if restore.Status.Phase != string(backupv1alpha1.PhaseCompleted) {
				log.Infow("Backup is in use by active restore", "restore", restore.Name)
				return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
			}
		}

		// Check if we should delete restic data
		if backup.Annotations[DeleteResticDataAnnotation] == "true" {
			// TODO: Implement restic data deletion
			// This would involve:
			// 1. Check for shared incremental data
			// 2. Use restic forget/remove commands
			// 3. Handle errors appropriately
			log.Info("Restic data deletion requested but not yet implemented")
		}

		// Move to next state
		if err := utils.AnnotateResources(ctx, deps, []client.Object{backup}, map[string]string{
			constants.AnnotationDeletionState: string(constants.DeletionStateRemoveFinalizer),
		}); err != nil {
			log.Errorw("Failed to update deletion state", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}
		return ctrl.Result{Requeue: true}, nil

	case constants.DeletionStateRemoveFinalizer:
		// Remove our finalizer
		if err := utils.RemoveFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
			log.Errorw("Failed to remove finalizer", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}
