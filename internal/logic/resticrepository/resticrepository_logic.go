package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HandleRepoPending handles the logic when a ResticRepository is in the Pending phase.
func HandleRepoPending(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-pending")
	log.Info("Handling pending repository")

	// 1. Add finalizer
	if err := utils.AddFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
		log.Errorw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Create the initialization job
	job, err := CreateInitJob(ctx, deps, repo)
	if err != nil {
		log.Errorw("Failed to create init job", "error", err)
		return ctrl.Result{}, err
	}

	// 3. Update status to Running
	return UpdateRepoStatus(ctx, deps, repo, v1.PhaseRunning, metav1.Now(), job.Name)
}

// CreateInitJob creates a new initialization job for the repository
func CreateInitJob(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (*batchv1.Job, error) {
	log := deps.Logger.Named("repo-init")

	jobSpec, err := utils.BuildInitJobSpec(repo)
	if err != nil {
		return nil, err
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to create init job: %w", err)
	}

	log.Infow("Init job created", "jobName", job.Name)
	return job, nil
}

// FindActiveJobForRepo finds the currently active job for a ResticRepository.
func FindActiveJobForRepo(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (*batchv1.Job, error) {
	selector := v1.Selector{
		Namespaces: []string{repo.Namespace},
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance":   repo.Name,
				"app.kubernetes.io/managed-by": constants.OperatorDomain,
			},
		},
	}
	jobList := &batchv1.JobList{}
	jobs, err := utils.FindMatchingResources[*batchv1.Job](ctx, deps, []v1.Selector{selector}, jobList)
	if err != nil {
		return nil, fmt.Errorf("failed to find job via selector: %w", err)
	}

	for _, job := range jobs {
		if job.Status.CompletionTime == nil {
			return job, nil // Found the active job
		}
	}

	return nil, nil // No active job found
}

// HandleRepoMaintenance handles scheduled maintenance (check, prune) for a ready repository.
func HandleRepoMaintenance(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-maintenance")
	log.Debug("Handling repository maintenance")
	// This is a placeholder for future maintenance logic (e.g., cron-based check/prune jobs)
	return ctrl.Result{RequeueAfter: constants.RepositoryRequeueInterval}, nil // Requeue periodically to check for maintenance
}

// HandleRepoFailed handles a repository in the Failed phase.
func HandleRepoFailed(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-failed")
	log.Info("Handling failed repository")

	// Get the failed job to extract logs
	job, err := FindActiveJobForRepo(ctx, deps, repo)
	if err == nil && job != nil { // Job exists
		// Attempt to get logs from the failed pod for better error reporting
		podLogs, logErr := utils.GetJobLogs(ctx, deps, job)
		if logErr != nil {
			log.Errorw("Failed to get logs from failed init job pod", "error", logErr)
		}
		log.Errorw("Init job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message, "logs", podLogs)
		repo.Status.Error = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", job.Status.Conditions[0].Reason, job.Status.Conditions[0].Message, podLogs)

		// Update status with the log message
		if err := deps.Client.Status().Update(ctx, repo); err != nil {
			log.Errorw("Failed to update repo status with failure logs", "error", err)
		}

		// Clean up the failed job
		if err := deps.Client.Delete(ctx, job); err != nil {
			log.Errorw("Failed to clean up failed init job", "error", err)
		} else {
			log.Info("Successfully cleaned up failed init job")
		}
	} else if err != nil {
		log.Errorw("Failed to get failed init job for log retrieval", "error", err)
	}

	// Terminal state. We might want to requeue after a long interval to allow for manual intervention.
	return ctrl.Result{RequeueAfter: constants.RepositoryRequeueInterval}, nil
}

// HandleRepoRunning handles the logic when a ResticRepository is in the Running phase.
func HandleRepoRunning(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-running")
	log.Info("Handling running repository")

	job, err := FindActiveJobForRepo(ctx, deps, repo)
	if err != nil {
		return ctrl.Result{}, err
	}
	if job == nil {
		log.Errorw("No init job found for repository, assuming failure")
		return UpdateRepoStatus(ctx, deps, repo, v1.PhaseFailed, metav1.Now(), "")
	}

	finished, succeeded := utils.IsJobFinished(job)
	if !finished {
		log.Debug("Init job still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		// Verify repository exists
		err := CheckRepositoryExists(ctx, deps, repo)
		if err == nil {
			log.Info("Repository initialized and verified")
			return UpdateRepoStatus(ctx, deps, repo, v1.PhaseCompleted, metav1.Now(), job.Name)
		}
		log.Errorw("Repository check failed after successful init", "error", err)
		return UpdateRepoStatus(ctx, deps, repo, v1.PhaseFailed, metav1.Now(), job.Name)
	}

	// Job failed
	log.Errorw("Init job failed")
	return UpdateRepoStatus(ctx, deps, repo, v1.PhaseFailed, metav1.Now(), job.Name)
}

// HandleRepoDeletion handles the deletion of a ResticRepository.
func HandleRepoDeletion(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-deletion")
	log.Info("Starting deletion logic for ResticRepository")

	// Cleanup logic here (e.g., delete jobs if any are running)
	// For now, we assume jobs are owned and garbage collected.
	if err := utils.RemoveFinalizer(ctx, deps, repo, constants.ResticRepositoryFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	log.Info("Completed deletion logic for ResticRepository")
	return ctrl.Result{}, nil
}

// CheckRepositoryExists checks if a repository exists by running a check job.
func CheckRepositoryExists(ctx context.Context, deps *utils.Dependencies, repository *v1.ResticRepository) error {
	log := deps.Logger.Named("repo-check").With("repository", repository.Name)
	log.Info("Checking if repository exists")

	// Create restic job to check repository
	jobSpec := utils.BuildCheckJobSpec(repository)
	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repository)
	if err != nil {
		return fmt.Errorf("failed to create check job: %w", err)
	}

	// Wait for job completion
	job, err = utils.GetResource[batchv1.Job](ctx, deps.Client, repository.Namespace, job.Name, log)
	if err != nil {
		return fmt.Errorf("failed to get check job: %w", err)
	}

	// Check job status
	if job.Status.Failed > 0 {
		return fmt.Errorf("repository check failed")
	}

	return nil
}

// UpdateRepoStatus updates the status of the ResticRepository resource.
func UpdateRepoStatus(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository, phase v1.Phase, transitionTime metav1.Time, jobName string) (ctrl.Result, error) {
	repo.Status.Phase = phase
	if phase != v1.PhaseFailed {
		repo.Status.Error = ""
	}

	// Update job reference if jobName is provided
	if jobName != "" {
		repo.Status.Job.JobRef = &corev1.LocalObjectReference{Name: jobName}
	}

	if repo.Status.InitializedTime == nil && (phase == v1.PhaseCompleted || phase == v1.PhaseFailed) {
		repo.Status.InitializedTime = &transitionTime
	}

	if err := deps.Client.Status().Update(ctx, repo); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// RunMaintenance runs maintenance on the repository.
func RunMaintenance(ctx context.Context, deps *utils.Dependencies, repository *v1.ResticRepository) (ctrl.Result, error) {
	// TODO: Implement maintenance operations
	log := deps.Logger.Named("repo-maintenance").With("repository", repository.Name)
	log.Info("Maintenance operations not yet implemented")
	return ctrl.Result{RequeueAfter: constants.RepositoryRequeueInterval}, nil
}
