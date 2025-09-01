package logic

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
)

// FindBackupsForRepository finds all backups using this repository
func FindBackupsForRepository(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) ([]*backupv1alpha1.ResticBackup, error) {
	// Find backups referencing this repository
	selector := utils.SelectorForResource(repo.Name, repo.Namespace)
	backupList := &backupv1alpha1.ResticBackupList{}
	backups, err := utils.FindMatchingResources[*backupv1alpha1.ResticBackup](ctx, deps, []backupv1alpha1.Selector{selector}, backupList)
	if err != nil {
		return nil, err
	}

	// Filter to only include backups that reference this repository
	var matchingBackups []*backupv1alpha1.ResticBackup
	for _, backup := range backups {
		if backup.Spec.RepositoryRef.Name == repo.Name {
			matchingBackups = append(matchingBackups, backup)
		}
	}

	return matchingBackups, nil
}

// FindRestoresForRepository finds all restores using this repository
func FindRestoresForRepository(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) ([]*backupv1alpha1.ResticRestore, error) {
	// Find restores referencing this repository
	selector := utils.SelectorForResource(repo.Name, repo.Namespace)
	restoreList := &backupv1alpha1.ResticRestoreList{}
	restores, err := utils.FindMatchingResources[*backupv1alpha1.ResticRestore](ctx, deps, []backupv1alpha1.Selector{selector}, restoreList)
	if err != nil {
		return nil, err
	}

	// Filter to only include restores that reference this repository
	var matchingRestores []*backupv1alpha1.ResticRestore
	for _, restore := range restores {
		if restore.Spec.RepositoryRef.Name == repo.Name {
			matchingRestores = append(matchingRestores, restore)
		}
	}

	return matchingRestores, nil
}

// FindActiveOperations finds all active backups and restores using this repository
func FindActiveOperations(ctx context.Context, deps *utils.Dependencies, repo *backupv1alpha1.ResticRepository) ([]string, error) {
	var activeOps []string

	// Check backups
	backups, err := FindBackupsForRepository(ctx, deps, repo)
	if err != nil {
		return nil, err
	}
	for _, backup := range backups {
		if backup.Status.Phase == string(backupv1alpha1.PhaseRunning) {
			activeOps = append(activeOps, "backup/"+backup.Name)
		}
	}

	// Check restores
	restores, err := FindRestoresForRepository(ctx, deps, repo)
	if err != nil {
		return nil, err
	}
	for _, restore := range restores {
		if restore.Status.Phase == string(backupv1alpha1.PhaseRunning) {
			activeOps = append(activeOps, "restore/"+restore.Name)
		}
	}

	return activeOps, nil
}
