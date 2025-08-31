package utils

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: These are TEMPORARY stubs used during aggressive cleanup to keep the
// codebase compiling while complex implementation logic is removed. Replace
// with real implementations when functionality is re-introduced.

// BuildBackupJobCommand returns placeholder command/args for a restic backup job.
func BuildBackupJobCommand(_ string, _ []string) ([]string, []string, error) {
	return []string{"/bin/true"}, []string{}, nil
}

// BuildRestoreJobCommand returns placeholder command/args for a restic restore job.
func BuildRestoreJobCommand(_ string) ([]string, []string, error) {
	return []string{"/bin/true"}, []string{}, nil
}

// JobOutput is a minimal placeholder returned by ReadJobOutput.
type JobOutput struct {
	Phase      string
	Error      string
	SnapshotID string
	Size       string // Keep as string to match original `fmt.Sscanf` usage
}

// IsJobFailed checks if the job has failed.
func (o *JobOutput) IsJobFailed() bool {
	return o.Phase == "Failed"
}

// IsJobCompleted checks if the job has completed successfully.
func (o *JobOutput) IsJobCompleted() bool {
	return o.Phase == "Completed"
}

// ReadJobOutput returns a completed phase by default to advance controllers.
func ReadJobOutput(_ context.Context, _ client.Client, _ string, _ string) (*JobOutput, error) {
	return &JobOutput{Phase: "Completed", Error: "", SnapshotID: "stub-snapshot-id", Size: "12345"}, nil
}

// CreateRestoreJobWithOutput returns placeholder Job and ConfigMap objects.
func CreateRestoreJobWithOutput(_ context.Context, _ client.Client, _ corev1.PersistentVolumeClaim, _ backupv1alpha1.BackupTarget, _ string, _ metav1.Object) (*batchv1.Job, *corev1.ConfigMap, error) {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "stub-restore-job"}}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "stub-output-cm"}}
	return job, cm, nil
}

// CleanupJob is a no-op placeholder.
func CleanupJob(_ context.Context, _ client.Client, _ string, _ string) error {
	return nil
}

// GetRepositoryNameForTarget computes a stable repository name for a target.
func GetRepositoryNameForTarget(target backupv1alpha1.BackupTarget) string {
	if target.Name != "" {
		return "repo-" + target.Name
	}
	return "repo-default"
}

// ApplyRetentionForTarget is a no-op placeholder used by retention helpers.
func ApplyRetentionForTarget(_ context.Context, _ client.Client, _ *backupv1alpha1.BackupConfig, _ *corev1.PersistentVolumeClaim, _ backupv1alpha1.BackupTarget) error {
	return nil
}
