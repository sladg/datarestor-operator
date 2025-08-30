package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	pointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResticBackupInfo represents information about a backup from Restic
type ResticBackupInfo struct {
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

// ResticJob handles restic operations via k8s jobs
type ResticJob struct {
	client      client.Client
	typedClient kubernetes.Interface
}

// NewResticJob creates a new ResticJob instance
func NewResticJob(client client.Client, typedClient kubernetes.Interface) *ResticJob {
	return &ResticJob{
		client:      client,
		typedClient: typedClient,
	}
}

// createResticJob creates a Kubernetes Job to run restic commands
func (r *ResticJob) createResticJob(ctx context.Context, pvc corev1.PersistentVolumeClaim, pvcBackup *backupv1alpha1.BackupConfig, jobType string, resticArgs []string) (*batchv1.Job, error) {
	logger := LoggerFrom(ctx, "job").
		WithPVC(pvc).
		WithValues("type", jobType)

	logger.Starting("create job")

	jobName := fmt.Sprintf("%s-%s-%s", jobType, pvc.Name, time.Now().Format("20060102-150405"))
	logger.WithValues("name", jobName).Debug("Generated job name")

	resticTarget := (*backupv1alpha1.BackupTarget)(nil)
	if pvcBackup != nil {
		for _, target := range pvcBackup.Spec.BackupTargets {
			if target.Restic != nil {
				resticTarget = &target
				break
			}
		}
	}
	if resticTarget == nil {
		logger.Failed("find restic target", fmt.Errorf("no target configured"))
		return nil, fmt.Errorf("no restic backup target configured")
	}

	logger.WithValues("target", resticTarget.Name).Debug("Found restic target")

	env := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: resticTarget.Restic.Repository,
		},
		{
			Name:  "RESTIC_PASSWORD",
			Value: "default-password", // TODO: Replace with proper secret handling
		},
	}
	for _, e := range resticTarget.Restic.Env {
		env = append(env, corev1.EnvVar{Name: e.Name, Value: e.Value})
	}

	image := resticTarget.Restic.Image
	if image == "" {
		image = "restic/restic:latest"
	}

	labels := map[string]string{
		"backupconfig.autorestore-backup-operator.com/restic-job": jobType,
		"backupconfig.autorestore-backup-operator.com/pvc-name":   pvc.Name,
		"backupconfig.autorestore-backup-operator.com/created-by": resticTarget.Name,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: pvc.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32Ptr(600),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "restic-job",
						Image:   image,
						Command: []string{"restic"},
						Args:    resticArgs,
						Env:     env,
						VolumeMounts: append(
							[]corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
									ReadOnly:  false,
								},
							},
							func() []corev1.VolumeMount {
								if resticTarget.NFS != nil {
									mountPath := resticTarget.NFS.MountPath
									if mountPath == "" {
										mountPath = "/mnt/backup"
									}
									return []corev1.VolumeMount{
										{
											Name:      "nfs-backup",
											MountPath: mountPath,
											ReadOnly:  false,
										},
									}
								}
								return nil
							}()...,
						),
					}},
					Volumes: append(
						[]corev1.Volume{
							{
								Name: "data",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvc.Name,
									},
								},
							},
						},
						func() []corev1.Volume {
							if resticTarget.NFS != nil {
								return []corev1.Volume{
									{
										Name: "nfs-backup",
										VolumeSource: corev1.VolumeSource{
											CSI: &corev1.CSIVolumeSource{
												Driver: "nfs.csi.k8s.io",
												VolumeAttributes: map[string]string{
													"server": resticTarget.NFS.Server,
													"share":  resticTarget.NFS.Path,
												},
												ReadOnly: pointer.Bool(false),
											},
										},
									},
								}
							}
							return nil
						}()...,
					),
				},
			},
		},
	}

	if err := r.client.Create(ctx, job); err != nil {
		logger.Failed("create job", err)
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	logger.Completed("create job")
	return job, nil
}

// waitForJobCompletion waits for a job to complete and returns its output
func (r *ResticJob) waitForJobCompletion(ctx context.Context, job *batchv1.Job) error {
	logger := LoggerFrom(ctx, "job").
		WithValues(
			"name", job.Name,
			"namespace", job.Namespace,
		)

	logger.Starting("wait for completion")

	for {
		var updatedJob batchv1.Job
		if err := r.client.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &updatedJob); err != nil {
			logger.Failed("get job status", err)
			return err
		}

		if updatedJob.Status.Succeeded > 0 {
			logger.Completed("wait for completion")
			return nil
		}

		if updatedJob.Status.Failed > 0 {
			err := fmt.Errorf("restic job failed")
			logger.Failed("wait for completion", err)
			return err
		}

		logger.WithValues(
			"active", updatedJob.Status.Active,
			"failed", updatedJob.Status.Failed,
			"succeeded", updatedJob.Status.Succeeded,
		).Debug("Job status")

		time.Sleep(2 * time.Second)
	}
}

// UploadBackup uploads backup data using Restic
func (r *ResticJob) UploadBackup(ctx context.Context, target backupv1alpha1.BackupTarget, backupData interface{}, pvc corev1.PersistentVolumeClaim, pvcBackup *backupv1alpha1.BackupConfig) error {
	if pvcBackup == nil {
		return fmt.Errorf("pvcBackup is required")
	}

	// Get backup ID from BackupConfig
	backupID, ok := pvcBackup.Annotations["backupconfig.autorestore-backup-operator.com/backup-id"]
	if !ok {
		return fmt.Errorf("backup ID not found in BackupConfig annotations")
	}

	// Create job with backup command and ID
	args := []string{"backup", "/data", "--tag", fmt.Sprintf("id=%s", backupID)}
	job, err := r.createResticJob(ctx, pvc, pvcBackup, "backup", args)
	if err != nil {
		return fmt.Errorf("failed to create backup job: %w", err)
	}
	defer func() { _ = r.client.Delete(ctx, job) }()

	// Wait for job completion
	if err := r.waitForJobCompletion(ctx, job); err != nil {
		return fmt.Errorf("backup job failed: %w", err)
	}

	return nil
}

// getBackupsFromRestic finds backups for a PVC using Restic snapshots command
func (r *ResticJob) getBackupsFromRestic(ctx context.Context, pvcBackup *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim) (string, error) {
	logger := LoggerFrom(ctx, "restic").
		WithPVC(pvc).
		WithValues("name", pvcBackup.Name)

	logger.Starting("find backups from restic")

	type ResticSnapshot struct {
		ID       string    `json:"id"`
		Time     time.Time `json:"time"`
		Hostname string    `json:"hostname"`
		Tags     []string  `json:"tags"`
		Paths    []string  `json:"paths"`
		Size     int64     `json:"size"`
	}

	var latestBackupID string
	var latestTime time.Time

	// Try each backup target in priority order
	targets := make([]backupv1alpha1.BackupTarget, len(pvcBackup.Spec.BackupTargets))
	copy(targets, pvcBackup.Spec.BackupTargets)
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Priority < targets[j].Priority
	})

	for _, target := range targets {
		targetLogger := logger.WithValues("target", target.Name)
		targetLogger.Debug("Checking backup target")

		if target.Restic == nil {
			targetLogger.Debug("Target has no Restic config, skipping")
			continue
		}

		// Create job to list snapshots
		args := []string{
			"snapshots",
			"--json",
			"--tag", fmt.Sprintf("pvc=%s", pvc.Name),
			"--tag", fmt.Sprintf("namespace=%s", pvc.Namespace),
		}
		job, err := r.createResticJob(ctx, pvc, pvcBackup, "list", args)
		if err != nil {
			targetLogger.Failed("create list job", err)
			continue
		}
		defer func() { _ = r.client.Delete(ctx, job) }()

		// Wait for job completion
		if err := r.waitForJobCompletion(ctx, job); err != nil {
			targetLogger.Failed("wait for list job", err)
			continue
		}

		// Get job logs
		var pods corev1.PodList
		if err := r.client.List(ctx, &pods, client.InNamespace(job.Namespace), client.MatchingLabels(job.Labels)); err != nil {
			targetLogger.Failed("list job pods", err)
			continue
		}
		if len(pods.Items) == 0 {
			targetLogger.Failed("find job pod", fmt.Errorf("no pods found"))
			continue
		}

		// Get logs from the first pod
		var logs []byte
		for _, pod := range pods.Items {
			if len(pod.Spec.Containers) == 0 {
				continue
			}

			// Get logs from the first container
			var stdout, stderr []byte
			stdout, err = r.getPodLogs(ctx, pod.Name, pod.Namespace, false)
			if err != nil {
				targetLogger.WithValues("error", err).Debug("Failed to get stdout")
				stderr, err = r.getPodLogs(ctx, pod.Name, pod.Namespace, true)
				if err != nil {
					targetLogger.WithValues("error", err).Debug("Failed to get stderr")
					continue
				}
				logs = stderr
			} else {
				logs = stdout
			}
			break
		}
		if logs == nil {
			targetLogger.Failed("get logs", fmt.Errorf("no logs found"))
			continue
		}

		// Parse JSON output
		var snapshots []ResticSnapshot
		if err := json.Unmarshal(logs, &snapshots); err != nil {
			targetLogger.Failed("parse logs", err)
			continue
		}

		// Find latest snapshot
		if len(snapshots) == 0 {
			targetLogger.Debug("No snapshots found in target")
			continue
		}

		// Sort by time descending
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].Time.After(snapshots[j].Time)
		})

		latest := snapshots[0]
		if latestBackupID == "" || latest.Time.After(latestTime) {
			latestBackupID = latest.ID
			latestTime = latest.Time
			targetLogger.WithValues(
				"backup_id", latestBackupID,
				"backup_time", latestTime,
			).Debug("Found newer backup")
		}
	}

	if latestBackupID == "" {
		logger.Debug("No backups found in any target")
		return "", nil
	}

	logger.WithValues(
		"backup_id", latestBackupID,
		"backup_time", latestTime,
	).Debug("Found latest backup from restic")
	return latestBackupID, nil
}

// getBackupsFromBackupJobs finds backups for a PVC using BackupJob resources
func (r *ResticJob) getBackupsFromBackupJobs(ctx context.Context, pvc corev1.PersistentVolumeClaim) (string, error) {
	logger := LoggerFrom(ctx, "restic").
		WithPVC(pvc)

	logger.Starting("find backups from BackupJobs")

	// List all BackupJobs for this PVC
	var pvcBackupJobs backupv1alpha1.BackupJobList
	if err := r.client.List(ctx, &pvcBackupJobs,
		client.InNamespace(pvc.Namespace),
		client.MatchingLabels{
			"backupconfig.autorestore-backup-operator.com/pvc": pvc.Name,
		}); err != nil {
		logger.Failed("list backupjobs", err)
		return "", fmt.Errorf("failed to list BackupJobs: %w", err)
	}

	// Find latest completed backup job
	var latestBackupJob *backupv1alpha1.BackupJob
	var latestTime time.Time

	for i := range pvcBackupJobs.Items {
		job := &pvcBackupJobs.Items[i]

		// Only consider completed jobs with restic ID
		if job.Status.Phase != "Completed" || job.Status.ResticID == "" {
			continue
		}

		// Check if this is the latest one
		if job.Status.CompletionTime != nil {
			completionTime := job.Status.CompletionTime.Time
			if latestBackupJob == nil || completionTime.After(latestTime) {
				latestBackupJob = job
				latestTime = completionTime
			}
		}
	}

	if latestBackupJob == nil {
		logger.Debug("No completed backup jobs found")
		return "", nil
	}

	logger.WithJob(latestBackupJob).Debug("Found latest backup from BackupJobs")

	return latestBackupJob.Status.ResticID, nil
}

// FindLatestBackup finds the latest backup for a PVC using both BackupJob and Restic resources
// This supports disaster recovery scenarios where the cluster state may be missing
func (r *ResticJob) FindLatestBackup(ctx context.Context, pvcBackup *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim) (string, error) {
	logger := LoggerFrom(ctx, "restic").
		WithPVC(pvc)

	if pvcBackup != nil {
		logger = logger.WithValues("backupconfig", pvcBackup.Name)
	}

	logger.Starting("find latest backup")

	// Try to find backup from BackupJobs first (normal operation)
	backupFromJobs, jobErr := r.getBackupsFromBackupJobs(ctx, pvc)
	if jobErr == nil && backupFromJobs != "" {
		logger.WithValues("backup_id", backupFromJobs, "source", "BackupJobs").Debug("Found backup from BackupJobs")
		return backupFromJobs, nil
	}
	logger.WithValues("error", jobErr).Debug("Failed to get backup from BackupJobs")

	// Try to find backup from Restic directly (disaster recovery)
	if pvcBackup != nil {
		backupFromRestic, resticErr := r.getBackupsFromRestic(ctx, pvcBackup, pvc)
		if resticErr == nil && backupFromRestic != "" {
			logger.WithValues("backup_id", backupFromRestic, "source", "Restic").Debug("Found backup from Restic")
			return backupFromRestic, nil
		}
		logger.WithValues("error", resticErr).Debug("Failed to get backup from Restic")
	} else {
		logger.Debug("No BackupConfig provided, skipping Restic direct query")
	}

	// No backups found - this is normal for newly created PVCs
	logger.Debug("No backups found (normal for new PVCs)")
	return "", nil
}

// FindLatestBackupForDisasterRecovery finds the latest backup for a PVC when no BackupConfig resource is available
// This is specifically for disaster recovery scenarios where the cluster state is missing
func (r *ResticJob) FindLatestBackupForDisasterRecovery(ctx context.Context, pvc corev1.PersistentVolumeClaim, backupTargets []backupv1alpha1.BackupTarget) (string, error) {
	logger := LoggerFrom(ctx, "restic").
		WithPVC(pvc).
		WithValues("mode", "disaster-recovery")

	logger.Starting("find latest backup for disaster recovery")

	// Try BackupJobs first (might exist even if BackupConfig is gone)
	backupFromJobs, jobErr := r.getBackupsFromBackupJobs(ctx, pvc)
	if backupFromJobs != "" {
		logger.WithValues(
			"backup_id", backupFromJobs,
			"source", "BackupJobs",
		).Debug("Found backup from existing BackupJobs")
		return backupFromJobs, nil
	}

	logger.WithValues("error", jobErr).Debug("No BackupJobs found, querying Restic directly")

	// Create a temporary BackupConfig structure for Restic queries
	tempBackupConfig := &backupv1alpha1.BackupConfig{
		Spec: backupv1alpha1.BackupConfigSpec{
			BackupTargets: backupTargets,
		},
	}

	// Query Restic directly using provided targets
	backupFromRestic, resticErr := r.getBackupsFromRestic(ctx, tempBackupConfig, pvc)
	if backupFromRestic != "" {
		logger.WithValues(
			"backup_id", backupFromRestic,
			"source", "Restic-direct",
		).Debug("Found backup from Restic")
		return backupFromRestic, nil
	}

	// No backups found - this could be normal for new PVCs or actual disaster recovery failure
	logger.WithValues("restic_error", resticErr).Debug("No backups found in disaster recovery mode")
	return "", nil
}

// getPodLogs gets logs from a pod's container using typed client
func (r *ResticJob) getPodLogs(ctx context.Context, podName, namespace string, previous bool) ([]byte, error) {
	// Get logs using typed client - much simpler!
	req := r.typedClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Previous: previous,
	})

	logStream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get log stream: %w", err)
	}
	defer logStream.Close()

	// Read all logs
	logs, err := io.ReadAll(logStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %w", err)
	}

	return logs, nil
}

// CleanupBackups cleans up old backups using Restic's forget command
func (r *ResticJob) CleanupBackups(ctx context.Context, target backupv1alpha1.BackupTarget, pvc corev1.PersistentVolumeClaim, retention *backupv1alpha1.RetentionPolicy) error {
	args := []string{"forget"}
	if retention.MaxSnapshots > 0 {
		args = append(args, "--keep-last", fmt.Sprintf("%d", retention.MaxSnapshots))
	}
	if retention.MaxAge != nil {
		args = append(args, "--keep-within", retention.MaxAge.Duration.String())
	}

	// Create job with forget command
	job, err := r.createResticJob(ctx, pvc, nil, "cleanup", args)
	if err != nil {
		return fmt.Errorf("failed to create cleanup job: %w", err)
	}
	defer func() { _ = r.client.Delete(ctx, job) }()

	// Wait for job completion
	if err := r.waitForJobCompletion(ctx, job); err != nil {
		return fmt.Errorf("cleanup job failed: %w", err)
	}

	return nil
}
