package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sync repositories status with spec. It will add missing repositories to status, remove extra repositories from status, and update the ordering of repositories in status.
func SyncRepositories(ctx context.Context, deps *utils.Dependencies, config *v1.Config) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[SyncRepositories]").With("name", config.Name, "namespace", config.Namespace)

	changed := false

	// Build set of spec targets
	specTargets := make(map[string]struct{}, len(config.Spec.Repositories))
	for _, repoSpec := range config.Spec.Repositories {
		specTargets[repoSpec.Target] = struct{}{}
	}

	// Detect removals (status entries not in spec)
	for _, st := range config.Status.Repositories {
		if _, ok := specTargets[st.Target]; !ok {
			logger.Infow("Repository removed from spec - dropping from status", "repository", st.Target)
			changed = true
		}
	}

	// Index status by target for reuse
	statusByTarget := make(map[string]v1.RepositoryStatus, len(config.Status.Repositories))
	for _, st := range config.Status.Repositories {
		statusByTarget[st.Target] = st
	}

	// Rebuild status slice to mirror spec order
	newStatus := make([]v1.RepositoryStatus, 0, len(config.Spec.Repositories))
	for _, repoSpec := range config.Spec.Repositories {
		if existing, ok := statusByTarget[repoSpec.Target]; ok {
			newStatus = append(newStatus, existing)
		} else {
			logger.Infow("Repository added in spec - adding to status", "repository", repoSpec.Target)
			changed = true
			newStatus = append(newStatus, v1.RepositoryStatus{
				Target:                 repoSpec.Target,
				Backups:                []string{},
				InitializedAt:          metav1.Time{},
				LastScheduledBackupRun: metav1.Time{},
			})
		}
	}

	// Detect reorder/differences if not already marked changed
	if !changed {
		if len(newStatus) != len(config.Status.Repositories) {
			changed = true
		} else {
			for i := range newStatus {
				if newStatus[i].Target != config.Status.Repositories[i].Target {
					changed = true
					break
				}
			}
		}
	}

	if !changed {
		return false, 0, nil
	}

	config.Status.Repositories = newStatus
	if err := deps.Status().Update(ctx, config); err != nil {
		logger.Errorw("Failed to update Config status during repository sync", "error", err)
		return true, constants.ImmediateRequeueInterval, err
	}

	return true, constants.ImmediateRequeueInterval, nil
}
