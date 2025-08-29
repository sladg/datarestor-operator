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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

// NFSClient handles NFS operations for PVC backup and restore
type NFSClient struct{}

// NewNFSClient creates a new NFS client
func NewNFSClient() *NFSClient {
	return &NFSClient{}
}

// UploadBackup uploads backup data to NFS
func (n *NFSClient) UploadBackup(ctx context.Context, nfsConfig *storagev1alpha1.NFSConfig, backupData interface{}, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Create temporary mount point
	mountPoint, err := os.MkdirTemp("", "nfs-backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	// Mount NFS share
	if err := n.mountNFS(ctx, nfsConfig, mountPoint); err != nil {
		return fmt.Errorf("failed to mount NFS: %w", err)
	}
	defer n.umountNFS(mountPoint)

	// Generate backup path and filename
	timestamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(mountPoint, nfsConfig.Path, pvc.Namespace, pvc.Name)
	backupFile := filepath.Join(backupDir, fmt.Sprintf("backup-%s.tar.gz", timestamp))

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Handle different types of backup data
	switch data := backupData.(type) {
	case *snapshotv1.VolumeSnapshot:
		// For VolumeSnapshot, create metadata file
		metadata := fmt.Sprintf(`{"type":"volumesnapshot","name":"%s","pvc":"%s/%s","timestamp":"%s"}`,
			data.Name, pvc.Namespace, pvc.Name, timestamp)
		if err := n.writeBackupFile(backupFile, []byte(metadata)); err != nil {
			return fmt.Errorf("failed to write VolumeSnapshot metadata: %w", err)
		}
	case map[string]string:
		// For file-level backup, create metadata
		metadata := fmt.Sprintf(`{"type":"filebackup","pvc":"%s/%s","timestamp":"%s","data":%s}`,
			pvc.Namespace, pvc.Name, timestamp, data)
		if err := n.writeBackupFile(backupFile, []byte(metadata)); err != nil {
			return fmt.Errorf("failed to write file backup metadata: %w", err)
		}
	default:
		// For other types, create a generic metadata file
		metadata := fmt.Sprintf(`{"type":"unknown","pvc":"%s/%s","timestamp":"%s"}`,
			pvc.Namespace, pvc.Name, timestamp)
		if err := n.writeBackupFile(backupFile, []byte(metadata)); err != nil {
			return fmt.Errorf("failed to write generic metadata: %w", err)
		}
	}

	// Create symlink to latest backup
	latestLink := filepath.Join(backupDir, "latest.tar.gz")
	if err := os.Remove(latestLink); err != nil && !os.IsNotExist(err) {
		logger.Error(err, "Failed to remove existing latest symlink")
	}
	if err := os.Symlink(filepath.Base(backupFile), latestLink); err != nil {
		logger.Error(err, "Failed to create latest symlink")
	}

	logger.Info("Successfully uploaded to NFS", "server", nfsConfig.Server, "path", backupFile)
	return nil
}

// FindBackup searches for backups in NFS
func (n *NFSClient) FindBackup(ctx context.Context, nfsConfig *storagev1alpha1.NFSConfig, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Create temporary mount point
	mountPoint, err := os.MkdirTemp("", "nfs-restore-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	// Mount NFS share
	if err := n.mountNFS(ctx, nfsConfig, mountPoint); err != nil {
		return nil, fmt.Errorf("failed to mount NFS: %w", err)
	}
	defer n.umountNFS(mountPoint)

	// Look for the latest backup
	backupDir := filepath.Join(mountPoint, nfsConfig.Path, pvc.Namespace, pvc.Name)
	latestLink := filepath.Join(backupDir, "latest.tar.gz")

	// Check if latest symlink exists
	if _, err := os.Lstat(latestLink); err != nil {
		// If no latest symlink, look for the most recent backup file
		files, err := n.listBackupFiles(backupDir)
		if err != nil {
			return nil, fmt.Errorf("failed to list backup files: %w", err)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no backups found in NFS for PVC %s/%s", pvc.Namespace, pvc.Name)
		}
		// Use the most recent file
		latestLink = files[0].Path
	}

	// Read the backup file
	backupData, err := os.ReadFile(latestLink)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file %s: %w", latestLink, err)
	}

	logger.Info("Successfully found NFS backup", "path", latestLink, "size", len(backupData))
	return backupData, nil
}

// ListBackups lists all backups for a PVC in NFS
func (n *NFSClient) ListBackups(ctx context.Context, nfsConfig *storagev1alpha1.NFSConfig, pvc corev1.PersistentVolumeClaim) ([]BackupFile, error) {
	// Create temporary mount point
	mountPoint, err := os.MkdirTemp("", "nfs-list-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	// Mount NFS share
	if err := n.mountNFS(ctx, nfsConfig, mountPoint); err != nil {
		return nil, fmt.Errorf("failed to mount NFS: %w", err)
	}
	defer n.umountNFS(mountPoint)

	// List backup files
	backupDir := filepath.Join(mountPoint, nfsConfig.Path, pvc.Namespace, pvc.Name)
	return n.listBackupFiles(backupDir)
}

// CleanupBackups cleans up old NFS backups based on retention policy
func (n *NFSClient) CleanupBackups(ctx context.Context, nfsConfig *storagev1alpha1.NFSConfig, pvc corev1.PersistentVolumeClaim, retention *storagev1alpha1.RetentionPolicy) error {
	logger := log.FromContext(ctx)

	// List all backups
	backupFiles, err := n.ListBackups(ctx, nfsConfig, pvc)
	if err != nil {
		return fmt.Errorf("failed to list NFS backups for cleanup: %w", err)
	}

	if len(backupFiles) == 0 {
		return nil
	}

	// Sort by modification time (oldest first)
	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].ModTime.Before(backupFiles[j].ModTime)
	})

	// Apply retention policies
	if retention.MaxSnapshots > 0 && len(backupFiles) > int(retention.MaxSnapshots) {
		// Delete oldest backups beyond the limit
		backupsToDelete := backupFiles[:len(backupFiles)-int(retention.MaxSnapshots)]
		for _, backup := range backupsToDelete {
			if err := os.Remove(backup.Path); err != nil {
				logger.Error(err, "Failed to delete old NFS backup", "path", backup.Path)
				continue
			}
			logger.Info("Deleted old NFS backup", "path", backup.Path, "modTime", backup.ModTime)
		}
	}

	if retention.MaxAge != nil {
		// Delete backups older than MaxAge
		cutoffTime := time.Now().Add(-retention.MaxAge.Duration)
		for _, backup := range backupFiles {
			if backup.ModTime.Before(cutoffTime) {
				if err := os.Remove(backup.Path); err != nil {
					logger.Error(err, "Failed to delete old NFS backup", "path", backup.Path)
					continue
				}
				logger.Info("Deleted old NFS backup due to age", "path", backup.Path, "modTime", backup.ModTime, "age", time.Since(backup.ModTime))
			}
		}
	}

	return nil
}

// BackupFile represents a backup file in NFS
type BackupFile struct {
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
}

// listBackupFiles lists all backup files in a directory
func (n *NFSClient) listBackupFiles(backupDir string) ([]BackupFile, error) {
	var backupFiles []BackupFile

	// Read directory entries
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, entry := range entries {
		// Skip directories and symlinks
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		// Only process backup files
		if !strings.HasPrefix(entry.Name(), "backup-") || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		filePath := filepath.Join(backupDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		backupFiles = append(backupFiles, BackupFile{
			Path:    filePath,
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].ModTime.After(backupFiles[j].ModTime)
	})

	return backupFiles, nil
}

// mountNFS mounts an NFS share to the specified mount point
func (n *NFSClient) mountNFS(ctx context.Context, nfsConfig *storagev1alpha1.NFSConfig, mountPoint string) error {
	logger := log.FromContext(ctx)

	// Build mount command
	args := []string{"-t", "nfs"}

	// Add mount options if specified
	if len(nfsConfig.MountOptions) > 0 {
		args = append(args, "-o", strings.Join(nfsConfig.MountOptions, ","))
	}

	// Add server and path
	args = append(args, fmt.Sprintf("%s:%s", nfsConfig.Server, nfsConfig.Path), mountPoint)

	logger.Info("Mounting NFS", "server", nfsConfig.Server, "path", nfsConfig.Path, "mountPoint", mountPoint)

	// Execute mount command
	cmd := exec.CommandContext(ctx, "mount", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mount NFS: %s, output: %s", err, string(output))
	}

	// Wait a moment for mount to stabilize
	time.Sleep(2 * time.Second)

	// Verify mount was successful
	if !n.isNFSMounted(mountPoint) {
		return fmt.Errorf("NFS mount verification failed")
	}

	logger.Info("NFS mounted successfully", "mountPoint", mountPoint)
	return nil
}

// umountNFS unmounts an NFS share
func (n *NFSClient) umountNFS(mountPoint string) error {
	logger := log.FromContext(context.Background())

	logger.Info("Unmounting NFS", "mountPoint", mountPoint)

	// Execute umount command
	cmd := exec.Command("umount", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(err, "Failed to unmount NFS", "output", string(output))
		return fmt.Errorf("failed to unmount NFS: %s, output: %s", err, string(output))
	}

	logger.Info("NFS unmounted successfully", "mountPoint", mountPoint)
	return nil
}

// isNFSMounted checks if NFS is mounted at the specified point
func (n *NFSClient) isNFSMounted(mountPoint string) bool {
	// Read /proc/mounts to check if NFS is mounted
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == mountPoint && strings.Contains(fields[0], ":") {
			return true
		}
	}

	return false
}

// writeBackupFile writes backup data to a file
func (n *NFSClient) writeBackupFile(filePath string, data []byte) error {
	// Create parent directories if they don't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the backup file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file %s: %w", filePath, err)
	}

	return nil
}
