package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// BackupRequest represents a flexible backup request that can handle various scenarios
type BackupRequest struct {
	PVC        *corev1.PersistentVolumeClaim
	Repository corev1.ObjectReference
	BackupType v1.BackupType
	SnapshotID string // optional, can be empty for default behavior
}

// CreateBackupForPVC creates a backup for a PVC with flexible parameters
func CreateBackupForPVC(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, req BackupRequest) error {
	log := deps.Logger.Named("create-backup-pvc")

	// Create SourcePVC reference
	sourcePVC, err := utils.CreateObjectReference(req.PVC.Name, req.PVC.Namespace, "SourcePVC")
	if err != nil {
		log.Warnw("Failed to create SourcePVC reference", err)
		return err
	}

	// Generate backup names
	backupName := utils.GenerateUniqueName(backupConfig.Name, req.PVC.Name, string(req.BackupType))
	shortHash := backupName[len(backupName)-6:]
	backupSpecName := fmt.Sprintf("%s-%s-%s", req.Repository.Name, req.PVC.Name, shortHash)

	// Create the ResticBackup CRD directly
	backup := &v1.ResticBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: backupConfig.Namespace,
		},
		Spec: v1.ResticBackupSpec{
			Name:       backupSpecName,
			Repository: req.Repository,
			SourcePVC:  sourcePVC,
			Type:       req.BackupType,
			SnapshotID: req.SnapshotID,
		},
		Status: v1.ResticBackupStatus{
			CommonStatus: v1.CommonStatus{
				Phase: v1.PhaseUnknown,
			},
		},
	}

	// Set controller reference
	if err := controllerutil.SetControllerReference(backupConfig, backup, deps.Scheme); err != nil {
		log.Warnw("Failed to set controller reference on ResticBackup", err)
		return err
	}

	// Create the resource
	if err := deps.Create(ctx, backup); err != nil {
		log.Warnw("Failed to create ResticBackup", err)
		return err
	}

	return nil
}
