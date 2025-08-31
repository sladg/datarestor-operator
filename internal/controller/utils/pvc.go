package utils

import (
	"context"
	"fmt"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyRetentionPolicyForPVC applies retention policy for a specific PVC using restic forget --dry-run
func ApplyRetentionPolicyForPVC(ctx context.Context, c client.Client, backupConfig *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "retention-pvc").
		WithValues("pvc", pvc.Name, "namespace", pvc.Namespace)

	// Get ResticBackup CRDs for this PVC
	resticBackups := &backupv1alpha1.ResticBackupList{}
	if err := c.List(ctx, resticBackups, client.InNamespace(pvc.Namespace), client.MatchingLabels{
		"pvc": pvc.Name,
	}); err != nil {
		return fmt.Errorf("failed to list ResticBackup CRDs: %w", err)
	}

	if len(resticBackups.Items) == 0 {
		return nil
	}

	// Apply retention policy to all backups for this PVC
	// Since ResticBackup now stores repository reference, we can apply retention per PVC
	if len(backupConfig.Spec.BackupTargets) > 0 {
		target := backupConfig.Spec.BackupTargets[0] // Use first target for retention policy
		if err := ApplyRetentionForTarget(ctx, c, backupConfig, nil, target); err != nil {
			logger.WithValues("error", err, "pvc", pvc.Name).Debug("Failed to apply retention for PVC")
		}
	}

	return nil
}

// (ApplyRetentionForTarget and GetRepositoryNameForTarget moved to restic.go to centralize restic-related helpers)
