package utils

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ValidateBackupReferences validates Repository and SourcePVC references for backup operations
func ValidateBackupReferences(ctx context.Context, deps *Dependencies, backup *v1.ResticBackup) error {
	log := deps.Logger.Named("backup-validation")

	// Validate Repository reference
	if err := ValidateObjectReference(backup.Spec.Repository, "Repository"); err != nil {
		log.Errorw("Invalid Repository reference", "repository", backup.Spec.Repository, "error", err)
		return fmt.Errorf("invalid Repository reference: %v", err)
	}

	// Validate SourcePVC reference
	if err := ValidateObjectReference(backup.Spec.SourcePVC, "SourcePVC"); err != nil {
		log.Errorw("Invalid SourcePVC reference", "sourcePVC", backup.Spec.SourcePVC, "error", err)
		return fmt.Errorf("invalid SourcePVC reference: %v", err)
	}

	return nil
}

// ValidateRestoreReferences validates Repository and TargetPVC references for restore operations
func ValidateRestoreReferences(ctx context.Context, deps *Dependencies, restore *v1.ResticRestore) error {
	log := deps.Logger.Named("restore-validation")

	// Validate Repository reference
	if err := ValidateObjectReference(restore.Spec.Repository, "Repository"); err != nil {
		log.Errorw("Invalid Repository reference", "repository", restore.Spec.Repository, "error", err)
		return fmt.Errorf("invalid Repository reference: %v", err)
	}

	// Validate TargetPVC reference
	if err := ValidateObjectReference(restore.Spec.TargetPVC, "TargetPVC"); err != nil {
		log.Errorw("Invalid TargetPVC reference", "targetPVC", restore.Spec.TargetPVC, "error", err)
		return fmt.Errorf("invalid TargetPVC reference: %v", err)
	}

	return nil
}

// ValidateBackupObjectsExist checks if Repository and SourcePVC objects exist for backup
func ValidateBackupObjectsExist(ctx context.Context, deps *Dependencies, backup *v1.ResticBackup) error {
	log := deps.Logger.Named("backup-existence-validation")

	// Check if Repository exists
	_, err := GetResource[*v1.ResticRepository](ctx, deps.Client, backup.Spec.Repository.Namespace, backup.Spec.Repository.Name)
	if err != nil {
		log.Errorw("Repository not found", "repository", backup.Spec.Repository, "error", err)
		return fmt.Errorf("repository '%s/%s' not found: %v", backup.Spec.Repository.Namespace, backup.Spec.Repository.Name, err)
	}

	// Check if SourcePVC exists
	_, err = GetResource[*corev1.PersistentVolumeClaim](ctx, deps.Client, backup.Spec.SourcePVC.Namespace, backup.Spec.SourcePVC.Name)
	if err != nil {
		log.Errorw("SourcePVC not found", "sourcePVC", backup.Spec.SourcePVC, "error", err)
		return fmt.Errorf("source PVC '%s/%s' not found: %v", backup.Spec.SourcePVC.Namespace, backup.Spec.SourcePVC.Name, err)
	}

	log.Info("Repository and SourcePVC objects exist")
	return nil
}

// ValidateRestoreObjectsExist checks if Repository and TargetPVC objects exist for restore
func ValidateRestoreObjectsExist(ctx context.Context, deps *Dependencies, restore *v1.ResticRestore) error {
	log := deps.Logger.Named("restore-existence-validation")

	// Check if Repository exists
	_, err := GetResource[*v1.ResticRepository](ctx, deps.Client, restore.Spec.Repository.Namespace, restore.Spec.Repository.Name)
	if err != nil {
		log.Errorw("Repository not found", "repository", restore.Spec.Repository, "error", err)
		return fmt.Errorf("repository '%s/%s' not found: %v", restore.Spec.Repository.Namespace, restore.Spec.Repository.Name, err)
	}

	// Check if TargetPVC exists
	_, err = GetResource[*corev1.PersistentVolumeClaim](ctx, deps.Client, restore.Spec.TargetPVC.Namespace, restore.Spec.TargetPVC.Name)
	if err != nil {
		log.Errorw("TargetPVC not found", "targetPVC", restore.Spec.TargetPVC, "error", err)
		return fmt.Errorf("target PVC '%s/%s' not found: %v", restore.Spec.TargetPVC.Namespace, restore.Spec.TargetPVC.Name, err)
	}

	log.Info("Repository and TargetPVC objects exist")
	return nil
}

// GetRepositoryForOperation retrieves and validates repository readiness for both backup and restore
func GetRepositoryForOperation(ctx context.Context, deps *Dependencies, namespace, name string) (*v1.ResticRepository, error) {
	log := deps.Logger.Named("repository-validation")

	repositoryObj, err := GetResource[*v1.ResticRepository](ctx, deps.Client, namespace, name)
	if err != nil {
		log.Errorw("Failed to get repository", "namespace", namespace, "name", name, "error", err)
		return nil, err
	}

	if repositoryObj.Status.Phase != v1.PhaseCompleted {
		log.Debug("Repository exists but not ready", "phase", repositoryObj.Status.Phase)
		return nil, fmt.Errorf("repository not ready")
	}

	log.Debug("Repository is ready")
	return repositoryObj, nil
}

// SetOperationFailed sets the operation status to failed and updates it
// This is a generic function that works for both backup and restore operations
func SetOperationFailed(ctx context.Context, deps *Dependencies, obj interface{}, errorMsg string) error {
	log := deps.Logger.Named("operation-failed")
	log.Errorw("Setting operation to failed state", "error", errorMsg)

	// Use reflection or type assertion to handle different object types
	switch operation := obj.(type) {
	case *v1.ResticBackup:
		operation.Status.Phase = v1.PhaseFailed
		operation.Status.Error = errorMsg
		return deps.Status().Update(ctx, operation)
	case *v1.ResticRestore:
		operation.Status.Phase = v1.PhaseFailed
		operation.Status.Error = errorMsg
		return deps.Status().Update(ctx, operation)
	default:
		return fmt.Errorf("unsupported operation type: %T", obj)
	}
}
