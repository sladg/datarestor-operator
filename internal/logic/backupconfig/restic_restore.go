package logic

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createResticRestore creates a new ResticRestore resource.
// If snapshotID is provided, it creates a manual restore; otherwise, it creates an automated restore.
func createResticRestore(ctx context.Context, deps *utils.Dependencies, backupConfig *v1.BackupConfig, pvc *corev1.PersistentVolumeClaim, repository *v1.ResticRepository, snapshotID string) error {

	// Determine restore type and name
	var restoreType v1.RestoreType
	var namePrefix string
	if snapshotID != "" {
		restoreType = v1.RestoreTypeManual
		namePrefix = fmt.Sprintf("manual-restore-%s-%s-%s", backupConfig.Name, pvc.Name, snapshotID)
	} else {
		restoreType = v1.RestoreTypeAutomated
		namePrefix = fmt.Sprintf("restore-%s-%s-%d", backupConfig.Name, pvc.Name, time.Now().Unix())
	}

	restore := &v1.ResticRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namePrefix,
			Namespace: pvc.Namespace,
		},
		Spec: v1.ResticRestoreSpec{
			Name: fmt.Sprintf("%s-%s", backupConfig.Name, pvc.Name),
			Repository: corev1.ObjectReference{
				Name:      repository.Name,
				Namespace: repository.Namespace,
			},
			Type: restoreType,
			TargetPVC: corev1.ObjectReference{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
			},
			SnapshotID: snapshotID,
		},
		Status: v1.ResticRestoreStatus{
			CommonStatus: v1.CommonStatus{
				Phase: v1.PhaseUnknown,
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(backupConfig, restore, deps.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on ResticRestore: %w", err)
	}

	// Create the ResticRestore
	return deps.Create(ctx, restore)
}
