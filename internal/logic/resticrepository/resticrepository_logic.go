package logic

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateInitJob creates a new initialization job for the repository
func CreateInitJob(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (*batchv1.Job, error) {
	log := deps.Logger.Named("repo-init")

	command, args, err := utils.BuildInitJobCommand("init")
	if err != nil {
		return nil, fmt.Errorf("failed to build init command: %w", err)
	}

	jobSpec := utils.ResticJobSpec{
		Namespace:  repo.Namespace,
		JobType:    "init",
		Command:    command,
		Args:       args,
		Repository: repo.Spec.Target,
		Image:      repo.Spec.Image,
		Env:        repo.Spec.Env,
		Owner:      repo,
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to create init job: %w", err)
	}

	log.Infow("Init job created", "jobName", job.Name)
	return job, nil
}

// GetJobBySelector finds a job using a selector
func GetJobBySelector(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository, jobType string) (*batchv1.Job, error) {
	jobSelector := v1.Selector{
		Namespaces: []string{repo.Namespace},
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance": repo.Name,
				"job-type":                   jobType,
			},
		},
	}
	jobList := &batchv1.JobList{}
	jobs, err := utils.FindMatchingResources[*batchv1.Job](ctx, deps, []v1.Selector{jobSelector}, jobList)
	if err != nil {
		return nil, fmt.Errorf("failed to find job via selector: %w", err)
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return jobs[0], nil
}

// HandleRepoMaintenance handles scheduled maintenance (check, prune) for a ready repository.
func HandleRepoMaintenance(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-maintenance")
	log.Debug("Handling repository maintenance")
	// This is a placeholder for future maintenance logic (e.g., cron-based check/prune jobs)
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil // Requeue periodically to check for maintenance
}

// HandleRepoFailed handles a repository in the Failed phase.
func HandleRepoFailed(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-failed")
	log.Info("Handling failed repository")
	// Terminal state. We might want to requeue after a long interval to allow for manual intervention.
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
}

// HandleRepoDeletion handles the deletion of a ResticRepository.
func HandleRepoDeletion(ctx context.Context, deps *utils.Dependencies, repo *v1.ResticRepository) (ctrl.Result, error) {
	log := deps.Logger.Named("repo-deletion")
	log.Info("Starting deletion logic for ResticRepository")

	if controllerutil.ContainsFinalizer(repo, constants.ResticRepositoryFinalizer) {
		// Cleanup logic here (e.g., delete jobs if any are running)
		// For now, we assume jobs are owned and garbage collected.

		// Remove the finalizer
		log.Debug("Removing finalizer")
		controllerutil.RemoveFinalizer(repo, constants.ResticRepositoryFinalizer)
		if err := deps.Client.Update(ctx, repo); err != nil {
			log.Errorw("Failed to remove finalizer", "error", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
	}

	log.Info("Completed deletion logic for ResticRepository")
	return ctrl.Result{}, nil
}

// CheckRepositoryExists checks if a repository exists by running a check job.
func CheckRepositoryExists(ctx context.Context, deps *utils.Dependencies, repository *v1.ResticRepository) error {
	log := deps.Logger.Named("repo-check").With("repository", repository.Name)
	log.Info("Checking if repository exists")

	// Create restic job to check repository
	jobSpec := utils.ResticJobSpec{
		Namespace:  repository.Namespace,
		JobType:    "check",
		Command:    []string{"restic", "check"},
		Args:       []string{"--repo", repository.Spec.Target},
		Repository: repository.Spec.Target,
		Image:      repository.Spec.Image,
		Env:        repository.Spec.Env,
		Owner:      repository,
	}
	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, repository)
	if err != nil {
		return fmt.Errorf("failed to create check job: %w", err)
	}

	// Wait for job completion
	if err := deps.Client.Get(ctx, client.ObjectKey{
		Name:      job.Name,
		Namespace: repository.Namespace,
	}, job); err != nil {
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
	repo.Status.Error = ""

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
	return ctrl.Result{RequeueAfter: time.Hour}, nil
}
