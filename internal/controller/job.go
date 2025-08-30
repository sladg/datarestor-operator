package controller

import (
	"context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Unified helper for creating restic jobs (backup/restore)
func (r *BackupConfigReconciler) createResticJob(ctx context.Context, pvc corev1.PersistentVolumeClaim, pvcBackup *backupv1alpha1.BackupConfig, jobType string, resticArgs []string) (*batchv1.Job, error) {
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
						// Add NFS volume if configured
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
	if err := r.Create(ctx, job); err != nil {
		logger.Failed("create job", err)
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	logger.Completed("create job")
	return job, nil
}

// Unified wait for job completion
func (r *BackupConfigReconciler) waitForJobCompletion(ctx context.Context, job *batchv1.Job) error {
	logger := LoggerFrom(ctx, "job").
		WithValues(
			"name", job.Name,
			"namespace", job.Namespace,
		)

	logger.Starting("wait for completion")

	for {
		var updatedJob batchv1.Job
		if err := r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &updatedJob); err != nil {
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
