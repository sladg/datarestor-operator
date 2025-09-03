package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HandleRepositoryDeletion handles the deletion logic for a ResticRepository.
// It ensures:
// 1. No active backups are using this repository
// 2. No active restores are using this repository
// 3. Cleans up any repository jobs (init, maintenance)
func HandleRepositoryDeletion(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repository-deletion")

	// Check deletion state
	state := repo.Annotations[constants.AnnotationDeletionState]
	if state == "" {
		// 2. Set deletion state annotation
		log.Debugw("Setting deletion state annotation", "state", constants.DeletionStateCleanup)
		cleanupState := constants.DeletionStateCleanup
		if err := utils.AnnotateResources(ctx, deps, []client.Object{repo}, map[string]*string{
			constants.AnnotationDeletionState: &cleanupState,
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set deletion state annotation: %w", err)
		}
	}

	switch state {
	case constants.DeletionStateCleanup:
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

		// Move to next state
		// 2. Update annotation
		log.Debugw("Setting deletion state annotation", "state", constants.DeletionStateRemoveFinalizer)
		removeFinalizerState := constants.DeletionStateRemoveFinalizer
		if err := utils.AnnotateResources(ctx, deps, []client.Object{repo}, map[string]*string{
			constants.AnnotationDeletionState: &removeFinalizerState,
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set deletion state annotation: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil

	case constants.DeletionStateRemoveFinalizer:
		// Remove our finalizer
		if err := utils.RemoveFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
			log.Errorw("Failed to remove finalizer", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}
