package utils

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
)

// getSnapshotIDFromAnnotation determines the snapshot ID to use for a manual restore.
// It first tries to find a ResticBackup resource with the given name. If found, it returns the snapshot ID from its spec.
// If not found, it assumes the annotation value is a direct snapshot ID.
func GetSnapshotIDFromAnnotation(ctx context.Context, deps *Dependencies, backupConfig *v1.BackupConfig, annotationValue string) string {
	if annotationValue == "true" || annotationValue == "now" {
		return "" // Empty string means use latest snapshot
	}

	// Try to get a ResticBackup by this name first
	backup, err := GetResource[*v1.ResticBackup](ctx, deps.Client, backupConfig.Namespace, annotationValue)
	if err == nil && backup != nil && backup.Spec.SnapshotID != "" {
		// Found a ResticBackup and it has a snapshot ID
		return backup.Spec.SnapshotID
	}

	// If no ResticBackup was found, or it didn't have a snapshot ID,
	// assume the annotation is a direct snapshot ID.
	return annotationValue
}
