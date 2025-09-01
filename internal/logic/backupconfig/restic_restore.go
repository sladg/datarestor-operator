package logic

import (
	"context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createRestoreJob creates a new ResticRestore resource.
func createResticRestore(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, pvc *corev1.PersistentVolumeClaim) error {
	// For auto-restore, we need to find the appropriate target.
	// This logic assumes the first target is the one to restore from, which might need refinement.
	if len(backupConfig.Spec.BackupTargets) == 0 {
		return fmt.Errorf("no targets defined in BackupConfig %s for auto-restore", backupConfig.Name)
	}
	target := backupConfig.Spec.BackupTargets[0]

	// Create PVC reference
	pvcRef := backupv1alpha1.PersistentVolumeClaimRef{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}

	restore := &backupv1alpha1.ResticRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("restore-%s-%s-%d", backupConfig.Name, pvc.Name, time.Now().Unix()),
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				constants.LabelPVCBackup: backupConfig.Name,
				constants.LabelPVC:       pvc.Name,
			},
		},
		Spec: backupv1alpha1.ResticRestoreSpec{
			Name:      fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			Type:      backupv1alpha1.RestoreTypeAutomated,
			TargetPVC: pvcRef,
			Restic: backupv1alpha1.ResticRepositorySpec{
				Target: target.Restic.Target,
				Env:    target.Restic.Env,
				Args:   target.Restic.Args,
				Image:  target.Restic.Image,
			},
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
func createResticRestoreWithID(ctx context.Context, deps *utils.Dependencies, backupConfig *backupv1alpha1.BackupConfig, pvc *corev1.PersistentVolumeClaim, backupID string) error {
	if len(backupConfig.Spec.BackupTargets) == 0 {
		return fmt.Errorf("no targets defined in BackupConfig %s for restore", backupConfig.Name)
	}
	target := backupConfig.Spec.BackupTargets[0]

	// Create PVC reference
	pvcRef := backupv1alpha1.PersistentVolumeClaimRef{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
	}

	restore := &backupv1alpha1.ResticRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("manual-restore-%s-%s-%s", backupConfig.Name, pvc.Name, backupID),
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				constants.LabelPVCBackup: backupConfig.Name,
				constants.LabelPVC:       pvc.Name,
			},
		},
		Spec: backupv1alpha1.ResticRestoreSpec{
			Name:      fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			Type:      backupv1alpha1.RestoreTypeManual,
			TargetPVC: pvcRef,
			Restic: backupv1alpha1.ResticRepositorySpec{
				Target: target.Restic.Target,
				Env:    target.Restic.Env,
				Args:   target.Restic.Args,
				Image:  target.Restic.Image,
			},
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
