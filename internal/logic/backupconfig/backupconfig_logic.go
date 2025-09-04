package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"

	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleBackupConfigDeletion(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) (ctrl.Result, error) {
	if err := utils.RemoveFinalizer(ctx, deps, backupConfig, constants.BackupConfigFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}
	return ctrl.Result{}, nil
}

func UpdateBackupConfigStatus(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) error {
	ownedRepos, err := findOwnedRepositories(ctx, deps, backupConfig)
	if err != nil {
		return err
	}

	if len(backupConfig.Status.Repositories) != len(ownedRepos) {
		backupConfig.Status.Repositories = ownedRepos
		backupConfig.Status.TargetsCount = int32(len(backupConfig.Spec.BackupTargets))
		return deps.Status().Update(ctx, backupConfig)
	}

	return nil
}
