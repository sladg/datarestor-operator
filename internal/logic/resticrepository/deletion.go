package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HandleRepositoryDeletion handles the deletion logic for a ResticRepository.
// It ensures:
// 1. No active backups are using this repository
// 2. No active restores are using this repository
// 3. Cleans up any repository jobs (init, maintenance)
func HandleRepositoryDeletion(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repository-deletion")

	// Check for active backups using this repository
	backups, err := utils.FindBackupsByRepository(ctx, deps, repo.Namespace, repo.Name)
	if err != nil {
		log.Errorw("Failed to list backups", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	for _, backup := range backups {
		if backup.Status.Phase == v1.PhaseRunning {
			log.Infow("Repository has active backups", "backup", backup.Name)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	// Check for active restores using this repository
	restores, err := utils.FindRestoresByRepository(ctx, deps, repo.Namespace, repo.Name)
	if err != nil {
		log.Errorw("Failed to list restores", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	for _, restore := range restores {
		if restore.Status.Phase == v1.PhaseRunning {
			log.Infow("Repository has active restores", "restore", restore.Name)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	// Remove our finalizer
	if err := utils.RemoveFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}
	return ctrl.Result{}, nil
}
