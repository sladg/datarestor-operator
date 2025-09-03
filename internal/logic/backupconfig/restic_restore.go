package logic

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createRestoreJob creates a new ResticRestore resource.
func createResticRestore(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvc *corev1.PersistentVolumeClaim, repo *v1.ResticRepository) error {
	// Create PVC reference
	pvcRef := v1.PersistentVolumeClaimRef{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}

	restore := &v1.ResticRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("restore-%s-%s-%d", backupConfig.Name, pvc.Name, time.Now().Unix()),
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				constants.LabelPVCName:      pvc.Name,
				constants.LabelBackupConfig: backupConfig.Name,
			},
		},
		Spec: v1.ResticRestoreSpec{
			Name:      fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			Type:      v1.RestoreTypeAutomated,
			TargetPVC: pvcRef,
			Restic:    repo.Spec,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(backupConfig, restore, deps.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on ResticRestore: %w", err)
	}

	// Create the ResticRestore
	return deps.Client.Create(ctx, restore)
}

// createManualRestoreJob creates a new manual ResticRestore resource.
func createResticRestoreWithID(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvc *corev1.PersistentVolumeClaim, backupID string, repo *v1.ResticRepository) error {
	// Create PVC reference
	pvcRef := v1.PersistentVolumeClaimRef{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}

	restore := &v1.ResticRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("manual-restore-%s-%s-%s", backupConfig.Name, pvc.Name, backupID),
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				constants.LabelPVCName:      pvc.Name,
				constants.LabelBackupConfig: backupConfig.Name,
			},
		},
		Spec: v1.ResticRestoreSpec{
			Name:       fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			Type:       v1.RestoreTypeManual,
			TargetPVC:  pvcRef,
			Restic:     repo.Spec,
			SnapshotID: backupID,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(backupConfig, restore, deps.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on ResticRestore: %w", err)
	}

	// Create the ResticRestore
	return deps.Client.Create(ctx, restore)
}
