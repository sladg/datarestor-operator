package resticrepository

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleRepoUnknown(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRepoUnknown]")
	log.Info("Handling unknown repository", "name", repo.Name, "currentPhase", repo.Status.Phase)

	repo.Status.Phase = v1.PhasePending

	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, deps.Status().Update(ctx, repo)
}

func HandleRepoPending(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRepoPending]")
	log.Info("Handling pending repository", "name", repo.Name)

	// Add the repository finalizer
	if err := utils.AddFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
		log.Errorw("Failed to add finalizer", err)
		repo.Status.Phase = v1.PhaseFailed
		if err := deps.Status().Update(ctx, repo); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
	}

	if repo.Status.Job.Name == "" {
		jobSpec := utils.BuildCheckJobSpec(repo)
		job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
		if err != nil {
			log.Errorw("Failed to create check job", err)
			repo.Status.Phase = v1.PhaseFailed
			repo.Status.Error = err.Error()

			if err := utils.UpdateStatusWithRetry(ctx, deps, repo); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
		}

		repo.Status.Job = job
		log.Infow("Created check job", "jobName", job.Name, "jobNamespace", job.Namespace)

		if err := deps.Status().Update(ctx, repo); err != nil {
			log.Errorw("Failed to update repository status", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Check if the job is finished
	finished, succeeded := utils.IsJobFinished(ctx, deps, repo.Status.Job)
	if !finished {
		log.Debug("Check job still running")
		return ctrl.Result{RequeueAfter: constants.QuickRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Repository exists, marking as completed")
		repo.Status.Phase = v1.PhaseCompleted
	} else {
		log.Info("Repository check failed, assuming not initialized")
		repo.Status.Phase = v1.PhaseRunning
	}

	// Clean up the job reference in both success and failure cases
	if err := utils.DeleteJob(ctx, deps, repo.Status.Job); err != nil {
		log.Errorw("Failed to clean up check repository job", err)
	}
	repo.Status.Job = corev1.ObjectReference{}

	if err := deps.Status().Update(ctx, repo); err != nil {
		log.Errorw("Failed to update repository status", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
}

func HandleRepoRunning(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRepoRunning]")
	log.Info("Handling running repository")

	if repo.Status.Job.Name == "" {
		jobSpec := utils.BuildInitJobSpec(repo)
		job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
		if err != nil {
			log.Errorw("Failed to create init job", err)
			repo.Status.Phase = v1.PhaseFailed
			if err := utils.UpdateStatusWithRetry(ctx, deps, repo); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
		}

		repo.Status.Job = job
		log.Infow("Created init job", "jobName", job.Name, "jobNamespace", job.Namespace)

		if err := deps.Status().Update(ctx, repo); err != nil {
			log.Errorw("Failed to update repository status", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	finished, succeeded := utils.IsJobFinished(ctx, deps, repo.Status.Job)
	if !finished {
		log.Debug("Init job still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Repository initialization job succeeded")
		repo.Status.Phase = v1.PhaseCompleted
	} else {
		log.Errorw("Repository initialization job failed", "Repository initialization job failed")
		repo.Status.Phase = v1.PhaseFailed
	}

	if err := deps.Status().Update(ctx, repo); err != nil {
		log.Errorw("Failed to update repository status", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

func HandleRepoCompleted(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRepoCompleted]")
	log.Info("Handling completed repository")

	if repo.Status.InitializedTime == nil {
		repo.Status.InitializedTime = &metav1.Time{Time: metav1.Now().Time}
	} else {
		return ctrl.Result{}, nil
	}

	if repo.Status.Job.Name != "" {
		if err := utils.DeleteJob(ctx, deps, repo.Status.Job); err != nil {
			log.Warnw("Failed to clean up completed repository job", err)
		} else {
			log.Info("Successfully cleaned up completed backup job")
		}
	}

	repo.Status.Job = corev1.ObjectReference{}

	return ctrl.Result{}, deps.Status().Update(ctx, repo)
}

func HandleRepoFailed(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRepoFailed]")
	log.Info("Handling failed repository")

	if repo.Status.FailedTime != nil {
		return ctrl.Result{}, nil
	}

	repo.Status.FailedTime = &metav1.Time{Time: metav1.Now().Time}

	podLogs := utils.CleanupJobWithLogs(ctx, deps, corev1.ObjectReference{Name: repo.Status.Job.Name, Namespace: repo.Status.Job.Namespace})
	repo.Status.Error = fmt.Sprintf("Repository job failed. Logs: %s", podLogs)

	return ctrl.Result{}, utils.UpdateStatusWithRetry(ctx, deps, repo)
}

func HandleRepoDeletion(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRepoDeletion]")

	// Check for active backups using this repository
	backups, err := utils.FindBackupsByRepository(ctx, deps, repo.Namespace, repo.Name)
	if err != nil {
		log.Errorw("Failed to list backups", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	for _, backup := range backups {
		if utils.Contains(constants.ActivePhases, backup.Status.Phase) {
			log.Infow("Repository has active backups", "backup", backup.Name)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	// Check for active restores using this repository
	restores, err := utils.FindRestoresByRepository(ctx, deps, repo.Namespace, repo.Name)
	if err != nil {
		log.Errorw("Failed to list restores", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	for _, restore := range restores {
		if utils.Contains(constants.ActivePhases, restore.Status.Phase) {
			log.Infow("Repository has active restores", "restore", restore.Name)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	if err := utils.RemoveFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
