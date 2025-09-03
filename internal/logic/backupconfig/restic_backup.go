package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createResticBackup creates a ResticBackup resource.
func createResticBackup(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvc *corev1.PersistentVolumeClaim, backupName string, target v1.BackupTarget, repository *v1.ResticRepository) error {
	backup := &v1.ResticBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				constants.LabelPVCName:      pvc.Name,
				constants.LabelBackupConfig: backupConfig.Name,
				"backup-target":             target.Name,
			},
		},
		Spec: v1.ResticBackupSpec{
			Name:       fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			Repository: repository,
			SourcePVC: corev1.ObjectReference{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
			},
			Type: v1.BackupTypeScheduled,
			Args: []string{
				"--tag", fmt.Sprintf("pvc=%s", pvc.Name),
				"--tag", fmt.Sprintf("namespace=%s", pvc.Namespace),
				"--tag", fmt.Sprintf("backup-config=%s", backupConfig.Name),
				"--tag", "backup-type=scheduled",
			},
			SnapshotID: backupName,
		},
		Status: v1.ResticBackupStatus{
			CommonStatus: v1.CommonStatus{
				Phase: v1.PhaseUnknown,
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(backupConfig, backup, deps.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	return deps.Create(ctx, backup)
}
