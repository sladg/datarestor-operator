package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// It syncs Spec into Status
func Init(ctx context.Context, deps *utils.Dependencies, config *v1.Config) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[InitConfig]").With("name", config.Name, "namespace", config.Namespace)

	// If not initiliazed yet, add finalizer
	if config.Status.InitializedAt.IsZero() {
		logger.Infow("Config not initialized yet, initializing...", "name", config.Name, "namespace", config.Namespace)

		controllerutil.AddFinalizer(config, constants.ConfigFinalizer)
		config.Status.InitializedAt = metav1.Now()
		config.Status.Repositories = make([]v1.RepositoryStatus, len(config.Spec.Repositories))

		// Initialize status for each repository
		for i, repo := range config.Spec.Repositories {
			config.Status.Repositories[i] = v1.RepositoryStatus{
				Target:                 repo.Target,
				Backups:                []string{},
				InitializedAt:          metav1.Time{},
				LastScheduledBackupRun: metav1.Time{},
			}
		}

		logger.Infow("Config initialized, adding finalizer", "finalizer", constants.ConfigFinalizer)
		if err := deps.Status().Update(ctx, config); err != nil {
			logger.Errorw("Failed to update Config status after initialization", "error", err)
			return true, constants.ImmediateRequeueInterval, err
		}

		return true, constants.ImmediateRequeueInterval, nil
	}

	return false, 0, nil
}
