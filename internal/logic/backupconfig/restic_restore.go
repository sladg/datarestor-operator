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

// RestoreRequest represents a flexible restore request that can handle various scenarios
type RestoreRequest struct {
	PVC         *corev1.PersistentVolumeClaim
	Repository  corev1.ObjectReference
	RestoreType v1.RestoreType
	SnapshotID  string // "latest", "now", specific ID, or empty for default behavior
}

// CreateRestoreForPVC creates a restore for a PVC with flexible parameters
func CreateRestoreForPVC(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, req RestoreRequest) error {
	log := deps.Logger.Named("create-restore-pvc")

	// Create TargetPVC reference
	targetPVC, err := utils.CreateObjectReference(req.PVC.Name, req.PVC.Namespace, "TargetPVC")
	if err != nil {
		log.Warnw("Failed to create TargetPVC reference", err)
		return err
	}

	// Generate restore names
	restoreName := utils.GenerateUniqueName(backupConfig.Name, req.PVC.Name, string(req.RestoreType))
	shortHash := restoreName[len(restoreName)-6:]
	restoreSpecName := fmt.Sprintf("%s-%s-%s", req.Repository.Name, req.PVC.Name, shortHash)

	// Create the ResticRestore CRD directly
	restore := &v1.ResticRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreName,
			Namespace: backupConfig.Namespace,
		},
		Spec: v1.ResticRestoreSpec{
			Name:       restoreSpecName,
			Repository: req.Repository,
			TargetPVC:  targetPVC,
			Type:       req.RestoreType,
			SnapshotID: req.SnapshotID,
		},
		Status: v1.ResticRestoreStatus{
			CommonStatus: v1.CommonStatus{
				Phase: v1.PhaseUnknown,
			},
		},
	}

	// Set controller reference
	if err := controllerutil.SetControllerReference(backupConfig, restore, deps.Scheme); err != nil {
		log.Warnw("Failed to set owner reference on ResticRestore", err)
		return err
	}

	// Create the resource
	if err := deps.Create(ctx, restore); err != nil {
		log.Warnw("Failed to create ResticRestore", err)
		return err
	}

	return nil
}
