package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/restic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Initialize repositories in status. This is used to initialize Restic's repositories that are not initialized yet (returns early if already initialized)
func InitRepositories(ctx context.Context, deps *utils.Dependencies, config *v1.Config) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[InitRepositories]").With(
		"name", config.Name,
		"namespace", config.Namespace,
	)

	reconcile := false

	for i := range config.Status.Repositories {
		repository := &config.Status.Repositories[i]
		if repository.InitializedAt.IsZero() {
			log := logger.With("repository", repository.Target)

			// If not initialized yet, run restic check + restic init and update the initialized --> rerun
			log.Info("Repository not initialized yet")

			specRepo := utils.FindRepositorySpec(config.Spec.Repositories, repository)

			reconcile = true
			mergedEnv := restic.MergeEnvs(config.Spec.Env, specRepo.Env)

			output, err := restic.ExecCheck(ctx, logger, repository.Target, mergedEnv)
			if err == nil {
				log.Infow("Repository checked", "output", output)
				repository.InitializedAt = metav1.Now()
				continue
			}

			output, err = restic.ExecInit(ctx, logger, repository.Target, mergedEnv)
			if err == nil {
				log.Infow("Repository initialized", "output", output)
				repository.InitializedAt = metav1.Now()
				continue
			}

			log.Errorw("Failed to check/initialize repository", err)
			return true, constants.DefaultRequeueInterval, err
		}
	}

	return reconcile, constants.ImmediateRequeueInterval, nil
}
