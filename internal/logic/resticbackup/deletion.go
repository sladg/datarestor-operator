package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HandleBackupDeletion handles the deletion logic for a ResticBackup.
// It ensures:
// 1. No active restores are using this backup
// 2. Optionally deletes restic data if explicitly requested
func HandleBackupDeletion(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-deletion")

	// Block deletion if the backup is in an active phase
	if backup.Status.Phase == v1.PhaseRunning || backup.Status.Phase == v1.PhasePending {
		log.Info("Deletion is blocked because the backup is in an active phase", "phase", backup.Status.Phase)
		// Requeue to re-evaluate after the backup has completed or failed.
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Check for active restores using this backup
	restores, err := utils.FindRestoresByBackupName(ctx, deps, backup.Namespace, backup.Name)
	if err != nil {
		log.Errorw("Failed to list restores", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	for _, restore := range restores {
		if restore.Status.Phase != v1.PhaseCompleted {
			log.Infow("Backup is in use by active restore", "restore", restore.Name)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	// Check if we should delete restic data
	if backup.Annotations[constants.AnnotationDeleteResticData] == "true" {
		// TODO: Implement restic data deletion
		// This would involve:
		// 1. Check for shared incremental data
		// 2. Use restic forget/remove commands
		// 3. Handle errors appropriately
		log.Info("Restic data deletion requested but not yet implemented")
	}

	// Remove our finalizer
	if err := utils.RemoveFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
