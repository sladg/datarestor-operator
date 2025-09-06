package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func handleBackupConfigAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) (bool, error) {
	annotations := backupConfig.GetAnnotations()
	if annotations == nil {
		return false, nil
	}

	pvcRefs := make([]corev1.ObjectReference, len(pvcs))
	for i, pvc := range pvcs {
		pvcRefs[i] = corev1.ObjectReference{Namespace: pvc.Namespace, Name: pvc.Name}
	}

	// If backupconfig annotation exists, apply it to all managed PVCs.
	if backupId, ok := annotations[constants.AnnotationManualBackup]; ok && backupId != "" {
		if err := utils.RemoveOwnAnnotation(ctx, deps, backupConfig, constants.AnnotationManualBackup); err != nil {
			return false, err
		}

		// Apply the annotation to the PVCs
		return true, utils.ApplyBulkAnnotation(ctx, deps, utils.BulkOperationParams{
			Objects:   pvcRefs,
			Operation: utils.AnnotationOperation,
			Map: map[string]*string{
				constants.AnnotationManualBackup: &backupId,
			},
		})
	}

	// If restoreconfig annotation exists, apply it to all managed PVCs.
	if restoreId, ok := annotations[constants.AnnotationManualRestore]; ok && restoreId != "" {
		if err := utils.RemoveOwnAnnotation(ctx, deps, backupConfig, constants.AnnotationManualRestore); err != nil {
			return false, err
		}

		// Apply the annotation to the PVCs
		return true, utils.ApplyBulkAnnotation(ctx, deps, utils.BulkOperationParams{
			Objects:   pvcRefs,
			Operation: utils.AnnotationOperation,
			Map: map[string]*string{
				constants.AnnotationManualRestore: &restoreId,
			},
		})
	}

	return false, nil
}

func handlePVCAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) bool {
	var reconcile = false

	for _, pvc := range pvcs {
		annotations := pvc.GetAnnotations()
		if annotations == nil {
			continue
		}

		// If the PVC has a manual backup annotation, create a backup for it
		if backupId, ok := annotations[constants.AnnotationManualBackup]; ok && backupId != "" {
			for _, repo := range backupConfig.Status.Repositories {
				req := BackupRequest{
					PVC:        pvc,
					Repository: repo,
					BackupType: v1.BackupTypeManual,
				}
				if err := CreateBackupForPVC(ctx, deps, backupConfig, req); err != nil {
					reconcile = true
					continue
				}
			}
		}

		// If the PVC has a manual restore annotation, create a restore for it
		if restoreId, ok := annotations[constants.AnnotationManualRestore]; ok && restoreId != "" {
			for _, repo := range backupConfig.Status.Repositories {
				req := RestoreRequest{
					PVC:         pvc,
					Repository:  repo,
					RestoreType: v1.RestoreTypeManual,
				}
				if err := CreateRestoreForPVC(ctx, deps, backupConfig, req); err != nil {
					reconcile = true
					continue
				}
			}
		}
	}

	return reconcile
}

func HandleAnnotations(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvcs []*corev1.PersistentVolumeClaim) (bool, ctrl.Result, error) {
	log := deps.Logger.Named("[BackupConfig][HandleAnnotations]")
	if reconcile, err := handleBackupConfigAnnotations(ctx, deps, backupConfig, pvcs); err != nil {
		log.Warnw("Failed to handle backup config annotations for BackupConfig", "backupConfig", backupConfig.Name)
		return false, ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	} else if reconcile {
		return true, ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	reconcile := handlePVCAnnotations(ctx, deps, backupConfig, pvcs)
	return reconcile, ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
}
