package config_util

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
)

type ListPVCsForConfigResult struct {
	MatchedPVCs   []*corev1.PersistentVolumeClaim
	NewPVCs       []*corev1.PersistentVolumeClaim
	UnclaimedPVCs []*corev1.PersistentVolumeClaim
}

func ListPVCsForConfig(ctx context.Context, deps *utils.Dependencies, config *v1.Config) ListPVCsForConfigResult {
	// All managed PVCs (which match selectors)
	managedPVCs := utils.GetPVCsForConfig(ctx, deps, config)
	config.Status.MatchedPVCsCount = int32(len(managedPVCs))

	// Filter new and unclaimed PVCs for auto-restore processing
	newPVCs := utils.FilterNewPVCs(managedPVCs, deps.Logger)
	unclaimedPVCs := utils.FilterUnclaimedPVCs(newPVCs, deps.Logger)

	return ListPVCsForConfigResult{
		MatchedPVCs:   managedPVCs,
		NewPVCs:       newPVCs,
		UnclaimedPVCs: unclaimedPVCs,
	}
}
