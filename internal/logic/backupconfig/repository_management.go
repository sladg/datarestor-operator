package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ensureRepositoryOwnership ensures a single repository is owned by the BackupConfig
func ensureRepositoryOwnership(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, target v1.BackupTarget) error {
	log := deps.Logger.Named("ensure-repository-ownership")

	// Find repository by name and namespace
	repoSelector := v1.Selector{
		Names:      []string{target.Repository.Name},
		Namespaces: []string{backupConfig.Namespace},
	}

	repos, err := utils.FindMatchingResources[*v1.ResticRepository](ctx, deps, []v1.Selector{repoSelector}, &v1.ResticRepositoryList{})
	if err != nil {
		return fmt.Errorf("failed to find repository %s: %w", target.Repository.Name, err)
	}

	if len(repos) == 0 {
		log.Debugw("Repository not found, will be created later", "repository", target.Repository.Name)
		return nil
	}

	// Ensure ownership for all matching repositories
	for _, repository := range repos {
		if !isOwnedByBackupConfig(repository, backupConfig) {
			log.Infow("Setting controller reference on repository", "repository", repository.Name)
			if err := controllerutil.SetControllerReference(backupConfig, repository, deps.Scheme); err != nil {
				log.Warnw("Failed to set controller reference", "repository", repository.Name, err)
				continue
			}
			if err := deps.Update(ctx, repository); err != nil {
				log.Warnw("Failed to update repository ownership", "repository", repository.Name, err)
			}
		}
	}

	return nil
}

// EnsureResticRepositories ensures that ResticRepository CRDs exist for all backup targets.
func EnsureResticRepositories(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) error {
	log := deps.Logger.Named("ensure-restic-repositories")
	log.Info("Ensuring ResticRepository CRDs exist for all backup targets")

	// Process each backup target
	for _, target := range backupConfig.Spec.BackupTargets {
		if err := ensureRepositoryOwnership(ctx, deps, backupConfig, target); err != nil {
			log.Errorw("Failed to ensure repository ownership", "target", target.Name, err)
			// Continue with other targets even if one fails
		}
	}

	return nil
}

// findOwnedRepositories finds all repositories owned by the BackupConfig
func findOwnedRepositories(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) ([]corev1.ObjectReference, error) {
	// Find all repositories in the same namespace
	repoSelector := v1.Selector{
		Names:      []string{}, // Get all repositories and filter by owner
		Namespaces: []string{backupConfig.Namespace},
	}

	repos, err := utils.FindMatchingResources[*v1.ResticRepository](ctx, deps, []v1.Selector{repoSelector}, &v1.ResticRepositoryList{})
	if err != nil {
		return nil, fmt.Errorf("failed to find repositories: %w", err)
	}

	// Filter repositories owned by this BackupConfig
	var ownedRepos []corev1.ObjectReference
	for _, repo := range repos {
		if isOwnedByBackupConfig(repo, backupConfig) {
			ownedRepos = append(ownedRepos, corev1.ObjectReference{
				Name:      repo.Name,
				Namespace: repo.Namespace,
			})
		}
	}

	return ownedRepos, nil
}
