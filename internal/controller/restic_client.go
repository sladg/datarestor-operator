package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResticClient handles all backup operations using Restic
type ResticClient struct {
	nfsMounter *NFSMounter
}

// NewResticClient creates a new Restic client
func NewResticClient(client client.Client) *ResticClient {
	return &ResticClient{
		nfsMounter: NewNFSMounter(client),
	}
}

// BackupInfo represents information about a backup
type BackupInfo struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Tags      []string  `json:"tags"`
	Host      string    `json:"host"`
	Paths     []string  `json:"paths"`
	Size      int64     `json:"size"`
	Snapshot  string    `json:"snapshot"`
	PVCName   string    `json:"pvcName"`
	Namespace string    `json:"namespace"`
}

// UploadBackup uploads backup data using Restic
func (r *ResticClient) UploadBackup(ctx context.Context, target storagev1alpha1.BackupTarget, backupData interface{}, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Handle NFS mounting if needed
	var err error
	if target.NFS != nil {
		_, err = r.nfsMounter.MountNFS(ctx, target)
		if err != nil {
			return fmt.Errorf("failed to mount NFS for target %s: %w", target.Name, err)
		}
		defer func() {
			if unmountErr := r.nfsMounter.UnmountNFS(ctx, target); unmountErr != nil {
				logger.Error(unmountErr, "Failed to unmount NFS after backup", "target", target.Name)
			}
		}()
	}

	// Use the restic config from the target
	config := target.Restic
	if config == nil {
		return fmt.Errorf("restic configuration is required")
	}

	// Create temporary directory for backup data
	tempDir, err := os.MkdirTemp("", "restic-backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Prepare backup data based on type
	var backupPath string
	var tags []string

	switch data := backupData.(type) {
	case *snapshotv1.VolumeSnapshot:
		// For VolumeSnapshot, create metadata file
		backupPath = filepath.Join(tempDir, "snapshot-metadata.json")
		metadata := fmt.Sprintf(`{"type":"volumesnapshot","name":"%s","pvc":"%s/%s","timestamp":"%s"}`,
			data.Name, pvc.Namespace, pvc.Name, time.Now().Format(time.RFC3339))
		if err := os.WriteFile(backupPath, []byte(metadata), 0644); err != nil {
			return fmt.Errorf("failed to write snapshot metadata: %w", err)
		}
		tags = []string{"volumesnapshot", pvc.Namespace, pvc.Name}

	case map[string]string:
		// For file-level backup, create metadata
		backupPath = filepath.Join(tempDir, "filebackup-metadata.json")
		metadata := fmt.Sprintf(`{"type":"filebackup","pvc":"%s/%s","timestamp":"%s","data":%s}`,
			pvc.Namespace, pvc.Name, time.Now().Format(time.RFC3339), data)
		if err := os.WriteFile(backupPath, []byte(metadata), 0644); err != nil {
			return fmt.Errorf("failed to write file backup metadata: %w", err)
		}
		tags = []string{"filebackup", pvc.Namespace, pvc.Name}

	default:
		// For other types, create generic metadata
		backupPath = filepath.Join(tempDir, "generic-metadata.json")
		metadata := fmt.Sprintf(`{"type":"unknown","pvc":"%s/%s","timestamp":"%s"}`,
			pvc.Namespace, pvc.Name, time.Now().Format(time.RFC3339))
		if err := os.WriteFile(backupPath, []byte(metadata), 0644); err != nil {
			return fmt.Errorf("failed to write generic metadata: %w", err)
		}
		tags = []string{"generic", pvc.Namespace, pvc.Name}
	}

	// Add custom tags if specified
	if len(config.Tags) > 0 {
		tags = append(tags, config.Tags...)
	}

	// Build restic backup command
	args := []string{"backup", backupPath}

	// Get effective repository path (handles NFS mounting)
	effectiveRepo, err := r.getEffectiveRepository(ctx, target)
	if err != nil {
		return fmt.Errorf("failed to get effective repository: %w", err)
	}

	// Add repository
	args = append(args, "--repo", effectiveRepo)

	// Add password
	args = append(args, "--password-stdin")

	// Add tags
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	// Add host if specified
	if config.Host != "" {
		args = append(args, "--host", config.Host)
	} else {
		args = append(args, "--host", fmt.Sprintf("%s-%s", pvc.Namespace, pvc.Name))
	}

	// Add custom flags
	args = append(args, config.Flags...)

	// Create command
	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Dir = tempDir

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PASSWORD=%s", config.Password))

	// Add custom environment variables
	for _, envVar := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	// Set up password input
	cmd.Stdin = strings.NewReader(config.Password + "\n")

	// Execute backup
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic backup failed: %w, output: %s", err, string(output))
	}

	logger.Info("Successfully created restic backup",
		"pvc", fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
		"tags", tags,
		"output", string(output))

	return nil
}

// FindBackup searches for backups using Restic
func (r *ResticClient) FindBackup(ctx context.Context, target storagev1alpha1.BackupTarget, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Handle NFS mounting if needed
	if target.NFS != nil {
		_, err := r.nfsMounter.MountNFS(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("failed to mount NFS for target %s: %w", target.Name, err)
		}
		defer func() {
			if unmountErr := r.nfsMounter.UnmountNFS(ctx, target); unmountErr != nil {
				logger.Error(unmountErr, "Failed to unmount NFS after search", "target", target.Name)
			}
		}()
	}

	// Use the restic config from the target
	config := target.Restic
	if config == nil {
		return nil, fmt.Errorf("restic configuration is required")
	}

	// Search for backups with PVC-specific tags
	tags := []string{pvc.Namespace, pvc.Name}

	// Build restic snapshots command
	effectiveRepo, err := r.getEffectiveRepository(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective repository: %w", err)
	}

	args := []string{"snapshots", "--repo", effectiveRepo, "--password-stdin"}

	// Add tag filters
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	// Add custom flags
	args = append(args, config.Flags...)

	// Create command
	cmd := exec.CommandContext(ctx, "restic", args...)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PASSWORD=%s", config.Password))

	// Add custom environment variables
	for _, envVar := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	// Set up password input
	cmd.Stdin = strings.NewReader(config.Password + "\n")

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("restic snapshots failed: %w, output: %s", err, string(output))
	}

	// Parse output to find latest backup
	snapshots, err := r.parseSnapshotsOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse snapshots output: %w", err)
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no backups found for PVC %s/%s", pvc.Namespace, pvc.Name)
	}

	// Return the latest backup
	latestBackup := snapshots[0]
	logger.Info("Found latest restic backup",
		"id", latestBackup.ID,
		"time", latestBackup.Time,
		"pvc", fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name))

	return latestBackup, nil
}

// ListBackups lists all backups for a PVC using Restic
func (r *ResticClient) ListBackups(ctx context.Context, config *storagev1alpha1.ResticConfig, pvc corev1.PersistentVolumeClaim) ([]BackupInfo, error) {
	// Build restic snapshots command
	args := []string{"snapshots", "--repo", config.Repository, "--password-stdin"}

	// Add tag filters
	tags := []string{pvc.Namespace, pvc.Name}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	// Add custom flags
	args = append(args, config.Flags...)

	// Create command
	cmd := exec.CommandContext(ctx, "restic", args...)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PASSWORD=%s", config.Password))

	// Add custom environment variables
	for _, envVar := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	// Set up password input
	cmd.Stdin = strings.NewReader(config.Password + "\n")

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("restic snapshots failed: %w, output: %s", err, string(output))
	}

	// Parse output
	snapshots, err := r.parseSnapshotsOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse snapshots output: %w", err)
	}

	return snapshots, nil
}

// CleanupBackups cleans up old backups using Restic's forget command
func (r *ResticClient) CleanupBackups(ctx context.Context, target storagev1alpha1.BackupTarget, pvc corev1.PersistentVolumeClaim, retention *storagev1alpha1.RetentionPolicy) error {
	logger := log.FromContext(ctx)

	// Handle NFS mounting if needed
	if target.NFS != nil {
		_, err := r.nfsMounter.MountNFS(ctx, target)
		if err != nil {
			return fmt.Errorf("failed to mount NFS for target %s: %w", target.Name, err)
		}
		defer func() {
			if unmountErr := r.nfsMounter.UnmountNFS(ctx, target); unmountErr != nil {
				logger.Error(unmountErr, "Failed to unmount NFS after cleanup", "target", target.Name)
			}
		}()
	}

	// Use the restic config from the target
	config := target.Restic
	if config == nil {
		return fmt.Errorf("restic configuration is required")
	}

	// Build restic forget command
	effectiveRepo, err := r.getEffectiveRepository(ctx, target)
	if err != nil {
		return fmt.Errorf("failed to get effective repository: %w", err)
	}

	args := []string{"forget", "--repo", effectiveRepo, "--password-stdin"}

	// Add tag filters
	tags := []string{pvc.Namespace, pvc.Name}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	// Add retention policies
	if retention.MaxSnapshots > 0 {
		args = append(args, "--keep-last", fmt.Sprintf("%d", retention.MaxSnapshots))
	}

	if retention.MaxAge != nil {
		args = append(args, "--keep-within", retention.MaxAge.Duration.String())
	}

	// Add custom flags
	args = append(args, config.Flags...)

	// Create command
	cmd := exec.CommandContext(ctx, "restic", args...)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PASSWORD=%s", config.Password))

	// Add custom environment variables
	for _, envVar := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	// Set up password input
	cmd.Stdin = strings.NewReader(config.Password + "\n")

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic forget failed: %w, output: %s", err, string(output))
	}

	logger.Info("Successfully cleaned up old backups",
		"pvc", fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
		"output", string(output))

	return nil
}

// RestoreBackup restores data from a Restic backup
func (r *ResticClient) RestoreBackup(ctx context.Context, config *storagev1alpha1.ResticConfig, backupID string, targetPath string) error {
	logger := log.FromContext(ctx)

	// Build restic restore command
	args := []string{"restore", backupID, "--repo", config.Repository, "--password-stdin", "--target", targetPath}

	// Add custom flags
	args = append(args, config.Flags...)

	// Create command
	cmd := exec.CommandContext(ctx, "restic", args...)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PASSWORD=%s", config.Password))

	// Add custom environment variables
	for _, envVar := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	// Set up password input
	cmd.Stdin = strings.NewReader(config.Password + "\n")

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic restore failed: %w, output: %s", err, string(output))
	}

	logger.Info("Successfully restored backup",
		"backupID", backupID,
		"target", targetPath,
		"output", string(output))

	return nil
}

// parseSnapshotsOutput parses the output of restic snapshots command
func (r *ResticClient) parseSnapshotsOutput(output string) ([]BackupInfo, error) {
	var snapshots []BackupInfo

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Skip header lines
	for _, line := range lines {
		if strings.Contains(line, "ID") || strings.Contains(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Parse snapshot line (format: ID Time Host Paths Tags)
		snapshot := BackupInfo{
			ID:   fields[0],
			Host: fields[2],
		}

		// Parse time
		if timeStr := fields[1]; timeStr != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
				snapshot.Time = t
			}
		}

		// Parse paths and tags
		if len(fields) > 3 {
			// Last field contains tags
			tagsStr := fields[len(fields)-1]
			if strings.HasPrefix(tagsStr, "[") && strings.HasSuffix(tagsStr, "]") {
				tagsStr = strings.Trim(tagsStr, "[]")
				snapshot.Tags = strings.Split(tagsStr, ",")
			}

			// Remaining fields are paths
			if len(fields) > 4 {
				snapshot.Paths = fields[3 : len(fields)-1]
			}
		}

		snapshots = append(snapshots, snapshot)
	}

	// Sort by time (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})

	return snapshots, nil
}

// CheckRepository checks if the Restic repository is accessible
func (r *ResticClient) CheckRepository(ctx context.Context, config *storagev1alpha1.ResticConfig) error {
	// Build restic snapshots command (lightweight operation)
	args := []string{"snapshots", "--repo", config.Repository, "--password-stdin", "--limit", "1"}

	// Create command
	cmd := exec.CommandContext(ctx, "restic", args...)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PASSWORD=%s", config.Password))

	// Add custom environment variables
	for _, envVar := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	// Set up password input
	cmd.Stdin = strings.NewReader(config.Password + "\n")

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to access restic repository: %w, output: %s", err, string(output))
	}

	return nil
}

// needsNFSMounting checks if a target requires NFS mounting
func (r *ResticClient) needsNFSMounting(target storagev1alpha1.BackupTarget) bool {
	return target.NFS != nil && target.NFS.Server != "" && target.NFS.Path != ""
}

// getEffectiveRepository returns the effective repository path, handling NFS mounting
func (r *ResticClient) getEffectiveRepository(ctx context.Context, target storagev1alpha1.BackupTarget) (string, error) {
	if !r.needsNFSMounting(target) {
		return target.Restic.Repository, nil
	}

	// For NFS targets, the repository should use the mount path
	mountPath, exists := r.nfsMounter.GetMountPath(target.Name)
	if !exists {
		return "", fmt.Errorf("NFS not mounted for target %s", target.Name)
	}

	// Replace the mount path placeholder in the repository URL
	// e.g., "local:/mnt/backup" -> "local:/mnt/nfs-target-name"
	repo := target.Restic.Repository
	if strings.HasPrefix(repo, "local:/mnt/") {
		repo = fmt.Sprintf("local:%s", mountPath)
	}

	return repo, nil
}
