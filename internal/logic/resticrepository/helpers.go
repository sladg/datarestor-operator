package resticrepository

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
)

func CheckRepositoryReady(ctx context.Context, deps *utils.Dependencies, repository corev1.ObjectReference) (bool, *v1.ResticRepository) {
	log := deps.Logger.Named("[CheckRepositoryReady]")

	repositoryObj, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, repository.Namespace, repository.Name)
	if err != nil {
		log.Errorw("Failed to get repository", "namespace", repository.Namespace, "name", repository.Name, "error", err)
		return false, nil
	}

	if repositoryObj.Status.Phase != v1.PhaseCompleted {
		log.Debug("Repository exists but not ready", "phase", repositoryObj.Status.Phase)
		return false, nil
	}

	return true, repositoryObj
}
