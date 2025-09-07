package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"

	ctrl "sigs.k8s.io/controller-runtime"
)

func UpdateBackupConfigStatus(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) error {
	ownedRepos, err := findOwnedRepositories(ctx, deps, backupConfig)
	if err != nil {
		return err
	}

	if len(backupConfig.Status.Repositories) != len(ownedRepos) {
		backupConfig.Status.Repositories = ownedRepos
		backupConfig.Status.TargetsCount = int32(len(backupConfig.Spec.BackupTargets))
		backupConfig.Status.PVCsCount = int32(len(pvcs))
		return deps.Status().Update(ctx, backupConfig)
	}

	return nil
}

func HandleBackupConfigDeletion(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig) (ctrl.Result, error) {
	if err := utils.RemoveOwnFinalizer(ctx, deps, backupConfig, constants.BackupConfigFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	return ctrl.Result{}, nil
}
