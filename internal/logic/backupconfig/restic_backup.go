package logic

import (
	"context"
	"fmt"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createResticBackup creates a ResticBackup resource.
func createResticBackup(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, pvc *corev1.PersistentVolumeClaim, backupName string, target backupv1alpha1.BackupTarget) error {
	repoName := fmt.Sprintf("%s-%s", backupConfig.Name, target.Name)
	backup := &backupv1alpha1.ResticBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				constants.LabelPVCBackup: backupConfig.Name,
				constants.LabelPVC:       pvc.Name,
				"pvc":                    pvc.Name,
				"backup-config":          backupConfig.Name,
				"backup-target":          target.Name,
			},
		},
		Spec: backupv1alpha1.ResticBackupSpec{
			Name: fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			SourcePVC: backupv1alpha1.PersistentVolumeClaimRef{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
			},
			Restic: backupv1alpha1.ResticRepositorySpec{
				Target: repoName,
				Env:    target.Restic.Env,
				Args:   target.Restic.Args,
				Image:  target.Restic.Image,
			},
			Type: backupv1alpha1.BackupTypeScheduled,
			Args: []string{
				"--tag", fmt.Sprintf("pvc=%s", pvc.Name),
				"--tag", fmt.Sprintf("namespace=%s", pvc.Namespace),
				"--tag", fmt.Sprintf("backup-config=%s", backupConfig.Name),
				"--tag", fmt.Sprintf("backup-type=scheduled"),
			},
		},
		Status: backupv1alpha1.ResticBackupStatus{
			CommonStatus: backupv1alpha1.CommonStatus{
				Phase: backupv1alpha1.PhasePending,
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(backupConfig, backup, deps.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	return deps.Client.Create(ctx, backup)
}
