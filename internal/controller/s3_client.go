package controller

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

// S3Client handles S3 operations for PVC backup and restore
type S3Client struct{}

// NewS3Client creates a new S3 client
func NewS3Client() *S3Client {
	return &S3Client{}
}

// createAWSConfig creates AWS configuration with custom credentials if provided
func (s *S3Client) createAWSConfig(ctx context.Context, s3Config *storagev1alpha1.S3Config) (aws.Config, error) {
	var cfg aws.Config
	var err error

	// If custom credentials are provided, use them
	if s3Config.AccessKeyID != "" && s3Config.SecretAccessKey != "" {
		// Create custom credentials provider
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(s3Config.Region),
			config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID:     s3Config.AccessKeyID,
					SecretAccessKey: s3Config.SecretAccessKey,
				},
			}),
		)
	} else {
		// Use default AWS configuration (IAM roles, environment variables, etc.)
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(s3Config.Region))
	}

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Set custom endpoint if provided (for MinIO, etc.)
	if s3Config.Endpoint != "" {
		cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               s3Config.Endpoint,
				SigningRegion:     s3Config.Region,
				HostnameImmutable: true,
			}, nil
		})
	}

	return cfg, nil
}

// UploadBackup uploads backup data to S3
func (s *S3Client) UploadBackup(ctx context.Context, s3Config *storagev1alpha1.S3Config, backupData interface{}, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Load AWS credentials and config
	cfg, err := s.createAWSConfig(ctx, s3Config)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// Create S3 uploader
	uploader := manager.NewUploader(client)

	// Generate backup key with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupKey := fmt.Sprintf("backups/%s/%s/backup-%s.tar.gz", pvc.Namespace, pvc.Name, timestamp)

	// Add path prefix if specified
	if s3Config.PathPrefix != "" {
		backupKey = fmt.Sprintf("%s/%s", s3Config.PathPrefix, backupKey)
	}

	// Handle different types of backup data
	var uploadBody io.Reader
	switch data := backupData.(type) {
	case *snapshotv1.VolumeSnapshot:
		// For VolumeSnapshot, we'll create a metadata file
		metadata := fmt.Sprintf(`{"type":"volumesnapshot","name":"%s","pvc":"%s/%s","timestamp":"%s"}`,
			data.Name, pvc.Namespace, pvc.Name, timestamp)
		uploadBody = strings.NewReader(metadata)
	case map[string]string:
		// For file-level backup, create metadata
		metadata := fmt.Sprintf(`{"type":"filebackup","pvc":"%s/%s","timestamp":"%s","data":%s}`,
			pvc.Namespace, pvc.Name, timestamp, data)
		uploadBody = strings.NewReader(metadata)
	default:
		// For other types, create a generic metadata file
		metadata := fmt.Sprintf(`{"type":"unknown","pvc":"%s/%s","timestamp":"%s"}`,
			pvc.Namespace, pvc.Name, timestamp)
		uploadBody = strings.NewReader(metadata)
	}

	// Prepare the upload
	uploadInput := &s3.PutObjectInput{
		Bucket: aws.String(s3Config.Bucket),
		Key:    aws.String(backupKey),
		Body:   uploadBody,
		Metadata: map[string]string{
			"pvc-namespace": pvc.Namespace,
			"pvc-name":      pvc.Name,
			"backup-type":   "pvc-backup",
			"timestamp":     timestamp,
		},
	}

	// Perform the upload
	_, err = uploader.Upload(ctx, uploadInput)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	logger.Info("Successfully uploaded to S3", "bucket", s3Config.Bucket, "key", backupKey)
	return nil
}

// FindBackup searches for backups in S3
func (s *S3Client) FindBackup(ctx context.Context, s3Config *storagev1alpha1.S3Config, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Load AWS credentials and config
	cfg, err := s.createAWSConfig(ctx, s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for S3 backup: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// List objects in the backup prefix
	prefix := fmt.Sprintf("backups/%s/%s/", pvc.Namespace, pvc.Name)
	if s3Config.PathPrefix != "" {
		prefix = fmt.Sprintf("%s/%s", s3Config.PathPrefix, prefix)
	}

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s3Config.Bucket),
		Prefix: aws.String(prefix),
	}

	// List all backup objects
	var backupObjects []s3types.Object
	paginator := s3.NewListObjectsV2Paginator(client, listInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}
		backupObjects = append(backupObjects, page.Contents...)
	}

	if len(backupObjects) == 0 {
		return nil, fmt.Errorf("no backups found in S3 for PVC %s/%s", pvc.Namespace, pvc.Name)
	}

	// Sort by last modified time (newest first)
	sort.Slice(backupObjects, func(i, j int) bool {
		return backupObjects[i].LastModified.After(*backupObjects[j].LastModified)
	})

	// Get the latest backup
	latestBackup := backupObjects[0]
	logger.Info("Found latest S3 backup", "key", *latestBackup.Key, "size", latestBackup.Size, "lastModified", latestBackup.LastModified)

	// Create S3 downloader
	downloader := manager.NewDownloader(client)

	// Prepare the download
	downloadInput := &s3.GetObjectInput{
		Bucket: aws.String(s3Config.Bucket),
		Key:    latestBackup.Key,
	}

	// Create a temporary file to download to
	tempFile, err := os.CreateTemp("", "pvc-backup-restore-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file for S3 download: %w", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up the temporary file

	// Perform the download
	_, err = downloader.Download(ctx, tempFile, downloadInput)
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}

	logger.Info("Successfully downloaded from S3", "bucket", s3Config.Bucket, "key", *latestBackup.Key)

	// Read the downloaded file into a byte slice
	backupData, err := io.ReadAll(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read downloaded file: %w", err)
	}

	return backupData, nil
}

// ListBackups lists all backups for a PVC in S3
func (s *S3Client) ListBackups(ctx context.Context, s3Config *storagev1alpha1.S3Config, pvc corev1.PersistentVolumeClaim) ([]s3types.Object, error) {
	// Load AWS credentials and config
	cfg, err := s.createAWSConfig(ctx, s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for S3 backup listing: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// List objects in the backup prefix
	prefix := fmt.Sprintf("backups/%s/%s/", pvc.Namespace, pvc.Name)
	if s3Config.PathPrefix != "" {
		prefix = fmt.Sprintf("%s/%s", s3Config.PathPrefix, prefix)
	}

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s3Config.Bucket),
		Prefix: aws.String(prefix),
	}

	// List all backup objects
	var backupObjects []s3types.Object
	paginator := s3.NewListObjectsV2Paginator(client, listInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}
		backupObjects = append(backupObjects, page.Contents...)
	}

	return backupObjects, nil
}

// CleanupBackups cleans up old S3 backups based on retention policy
func (s *S3Client) CleanupBackups(ctx context.Context, s3Config *storagev1alpha1.S3Config, pvc corev1.PersistentVolumeClaim, retention *storagev1alpha1.RetentionPolicy) error {
	logger := log.FromContext(ctx)

	// List all backups
	backupObjects, err := s.ListBackups(ctx, s3Config, pvc)
	if err != nil {
		return fmt.Errorf("failed to list S3 backups for cleanup: %w", err)
	}

	if len(backupObjects) == 0 {
		return nil
	}

	// Sort by last modified time (oldest first)
	sort.Slice(backupObjects, func(i, j int) bool {
		return backupObjects[i].LastModified.Before(*backupObjects[j].LastModified)
	})

	// Load AWS credentials and config
	cfg, err := s.createAWSConfig(ctx, s3Config)
	if err != nil {
		return fmt.Errorf("failed to load AWS config for S3 cleanup: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// Apply retention policies
	if retention.MaxSnapshots > 0 && len(backupObjects) > int(retention.MaxSnapshots) {
		// Delete oldest backups beyond the limit
		backupsToDelete := backupObjects[:len(backupObjects)-int(retention.MaxSnapshots)]
		for _, backup := range backupsToDelete {
			deleteInput := &s3.DeleteObjectInput{
				Bucket: aws.String(s3Config.Bucket),
				Key:    backup.Key,
			}

			_, err := client.DeleteObject(ctx, deleteInput)
			if err != nil {
				logger.Error(err, "Failed to delete old S3 backup", "key", *backup.Key)
				continue
			}

			logger.Info("Deleted old S3 backup", "key", *backup.Key, "lastModified", backup.LastModified)
		}
	}

	if retention.MaxAge != nil {
		// Delete backups older than MaxAge
		cutoffTime := time.Now().Add(-retention.MaxAge.Duration)
		for _, backup := range backupObjects {
			if backup.LastModified.Before(cutoffTime) {
				deleteInput := &s3.DeleteObjectInput{
					Bucket: aws.String(s3Config.Bucket),
					Key:    backup.Key,
				}

				_, err := client.DeleteObject(ctx, deleteInput)
				if err != nil {
					logger.Error(err, "Failed to delete old S3 backup", "key", *backup.Key)
					continue
				}

				logger.Info("Deleted old S3 backup due to age", "key", *backup.Key, "lastModified", backup.LastModified, "age", time.Since(*backup.LastModified))
			}
		}
	}

	return nil
}
