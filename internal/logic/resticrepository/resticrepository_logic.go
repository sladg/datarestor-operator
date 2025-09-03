package resticrepository

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleRepoPending(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-pending")
	log.Info("Handling pending repository", "name", repo.Name)

	if repo.Status.Job == nil {
		jobSpec := utils.BuildCheckJobSpec(repo)
		job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
		if err != nil {
			log.Errorw("Failed to create check job", "error", err)
			repo.Status.Phase = v1.PhaseFailed
			repo.Status.Error = err.Error()
			return ctrl.Result{}, err
		}

		repo.Status.Job = job

		if err := deps.Status().Update(ctx, repo); err != nil {
			log.Errorw("Failed to update repository status", "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Check if the job is finished
	finished, succeeded := utils.IsJobFinished(repo.Status.Job)
	if !finished {
		log.Debug("Check job still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Repository exists, marking as completed")
		repo.Status.Phase = v1.PhaseCompleted
	} else {
		log.Info("Repository check failed, assuming not initialized")
		repo.Status.Phase = v1.PhaseRunning

		if err := deps.Delete(ctx, repo.Status.Job); err != nil {
			log.Errorw("Failed to clean up check repository job", "error", err)
		}

		repo.Status.Job = nil
	}

	if err := deps.Status().Update(ctx, repo); err != nil {
		log.Errorw("Failed to update repository status", "error", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// HandleRepoRunning handles the logic when a ResticRepository is in the Running phase.
// This function is focused on initializing the repository and monitoring the initialization job.
func HandleRepoRunning(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-running")
	log.Info("Handling running repository")

	if repo.Status.Job == nil {
		jobSpec := utils.BuildInitJobSpec(repo)
		job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
		if err != nil {
			log.Errorw("Failed to create init job", "error", err)
			repo.Status.Phase = v1.PhaseFailed
			repo.Status.Error = err.Error()
			return ctrl.Result{}, err
		}

		repo.Status.Job = job

		if err := deps.Status().Update(ctx, repo); err != nil {
			log.Errorw("Failed to update repository status", "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	finished, succeeded := utils.IsJobFinished(repo.Status.Job)
	if !finished {
		log.Debug("Init job still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Repository initialization job succeeded")
		repo.Status.Phase = v1.PhaseCompleted
	} else {
		log.Errorw("Repository initialization job failed", "error", repo.Status.Job.Status.Conditions[0].Message)
		repo.Status.Phase = v1.PhaseFailed
	}

	if err := deps.Status().Update(ctx, repo); err != nil {
		log.Errorw("Failed to update repository status", "error", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

func HandleRepoCompleted(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-completed")
	log.Info("Handling completed repository")

	if repo.Status.Job == nil {
		return ctrl.Result{}, nil
	}

	if err := deps.Delete(ctx, repo.Status.Job); err != nil {
		log.Errorw("Failed to clean up completed repository job", "error", err)
	} else {
		log.Info("Successfully cleaned up completed backup job")
	}

	return ctrl.Result{}, nil
}

func HandleRepoFailed(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-failed")
	log.Info("Handling failed repository")

	if repo.Status.Job == nil {
		return ctrl.Result{}, nil
	}

	podLogs, _ := utils.GetJobLogs(ctx, deps, repo.Status.Job)

	repo.Status.Error = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", repo.Status.Job.Status.Conditions[0].Reason, repo.Status.Job.Status.Conditions[0].Message, podLogs)
	log.Errorw("Repository initialization job failed", repo.Status.Error)

	if err := deps.Status().Update(ctx, repo); err != nil {
		log.Errorw("Failed to update repository status with failure logs", "error", err)
	}

	if err := deps.Delete(ctx, repo.Status.Job); err != nil {
		log.Errorw("Failed to clean up failed repository job", "error", err)
	} else {
		log.Info("Successfully cleaned up failed repository job")
	}

	return ctrl.Result{}, nil
}

func HandleRepoDeletion(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-deletion")

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

	if err := utils.RemoveFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
