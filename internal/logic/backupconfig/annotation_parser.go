package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clearAnnotations(ctx context.Context, deps *utils.Dependencies, resources []client.Object, annotations map[string]*string) error {
	log := deps.Logger.Named("[clearAnnotations]")

	if err := utils.AnnotateResources(ctx, deps, resources, annotations); err != nil {
		log.Warnw("Failed to remove annotations", err)
		return err
	}
	return nil
}

func handleBackupConfigAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) error {
	annotations := backupConfig.GetAnnotations()
	if annotations == nil {
		return nil
	}

	if backupId, ok := annotations[constants.AnnotationManualBackup]; ok && backupId != "" {
		for _, pvc := range pvcs {
			for _, repo := range backupConfig.Status.Repositories {
				req := BackupRequest{
					PVC:        pvc,
					Repository: repo,
					BackupType: v1.BackupTypeManual,
					SnapshotID: backupId,
				}
				if err := CreateBackupForPVC(ctx, deps, backupConfig, req); err != nil {
					continue
				}
			}
		}

		return clearAnnotations(ctx, deps, []client.Object{backupConfig}, map[string]*string{
			constants.AnnotationManualBackup: nil,
		})
	}

	if restoreName, ok := annotations[constants.AnnotationManualRestore]; ok && restoreName != "" {
		for _, pvc := range pvcs {
			for _, repo := range backupConfig.Status.Repositories {
				req := RestoreRequest{
					PVC:         pvc,
					Repository:  repo,
					RestoreType: v1.RestoreTypeManual,
					SnapshotID:  restoreName,
				}
				if err := CreateRestoreForPVC(ctx, deps, backupConfig, req); err != nil {
					continue
				}
			}
		}

		return clearAnnotations(ctx, deps, []client.Object{backupConfig}, map[string]*string{
			constants.AnnotationManualRestore: nil,
		})
	}

	return nil
}

// handlePVCAnnotations processes individual PVC annotations for backup and restore
func handlePVCAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvc *corev1.PersistentVolumeClaim) error {
	annotations := pvc.GetAnnotations()
	if annotations == nil {
		return nil
	}

	if backupRequestId, ok := annotations[constants.AnnotationManualBackup]; ok && backupRequestId != "" {
		for _, repo := range backupConfig.Status.Repositories {
			req := BackupRequest{
				PVC:        pvc,
				Repository: repo,
				BackupType: v1.BackupTypeManual,
				SnapshotID: backupRequestId,
			}
			if err := CreateBackupForPVC(ctx, deps, backupConfig, req); err != nil {
				continue
			}
		}

		return clearAnnotations(ctx, deps, []client.Object{pvc}, map[string]*string{
			constants.AnnotationManualBackup: nil,
		})
	}

	if restoreRequestId, ok := annotations[constants.AnnotationManualRestore]; ok && restoreRequestId != "" {
		for _, repo := range backupConfig.Status.Repositories {
			req := RestoreRequest{
				PVC:         pvc,
				Repository:  repo,
				RestoreType: v1.RestoreTypeManual,
				SnapshotID:  restoreRequestId,
			}
			if err := CreateRestoreForPVC(ctx, deps, backupConfig, req); err != nil {
				continue
			}
		}

		return clearAnnotations(ctx, deps, []client.Object{pvc}, map[string]*string{
			constants.AnnotationManualRestore: nil,
		})
	}

	return nil
}

// HandleAnnotations processes manual backup/restore annotations on BackupConfig and PVCs
func HandleAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("[BackupConfig][HandleAnnotations]")
	if err := handleBackupConfigAnnotations(ctx, deps, backupConfig, pvcs); err != nil {
		log.Warnw("Failed to handle backup config annotations for BackupConfig", "backupConfig", backupConfig.Name)
		return err
	}

	for _, pvc := range pvcs {
		if err := handlePVCAnnotations(ctx, deps, backupConfig, pvc); err != nil {
			log.Warnw("Failed to handle PVC annotations for PVC", "pvc", pvc.Name)
		}
	}

	return nil
}
