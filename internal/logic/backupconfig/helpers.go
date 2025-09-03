package logic

import (
	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
)

// isOwnedByBackupConfig checks if a ResticRepository is owned by the given BackupConfig.
func isOwnedByBackupConfig(repo *v1.ResticRepository, backupConfig *v1.BackupConfig) bool {
	for _, ownerRef := range repo.OwnerReferences {
		if ownerRef.Kind == "BackupConfig" &&
			ownerRef.Name == backupConfig.Name &&
			ownerRef.UID == backupConfig.UID {
			return true
		}
	}
	return false
}
