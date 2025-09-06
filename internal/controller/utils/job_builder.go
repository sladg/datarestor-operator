package utils

import (
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BuildInitJobSpec builds the spec for a repository initialization job.
func BuildInitJobSpec(repository *v1.ResticRepository) ResticJobSpec {
	return ResticJobSpec{
		Namespace: repository.Namespace,
		JobType:   "init",
		Args:      []string{"restic", "init", "--repo", repository.Spec.Target},
		Image:     repository.Spec.Image,
		Env:       repository.Spec.Env,
		Owner:     repository,
	}
}

// BuildCheckJobSpec builds the spec for a repository check job.
func BuildCheckJobSpec(repository *v1.ResticRepository) ResticJobSpec {
	return ResticJobSpec{
		Namespace: repository.Namespace,
		JobType:   "check",
		Args:      []string{"restic", "check", "--repo", repository.Spec.Target},
		Image:     repository.Spec.Image,
		Env:       repository.Spec.Env,
		Owner:     repository,
	}
}

// BuildRestoreJobSpec builds the spec for a restore job.
// args must include NAME (aka. latest, or snapshot ID)
func BuildRestoreJobSpec(restore *v1.ResticRestore, repository *v1.ResticRepository, args []string) ResticJobSpec {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: restore.Spec.TargetPVC.Name,
				},
			},
		},
	}

	jobArgs := []string{
		"restic", "restore",
		"--repo", repository.Spec.Target,
		// --host is set based on the annotation. it might be other PVC name
		"--tag", fmt.Sprintf("name=%s", restore.Spec.Name),
		"--target", "/data",
	}

	jobArgs = append(jobArgs, args...)

	return ResticJobSpec{
		Namespace:    restore.Spec.TargetPVC.Namespace,
		JobType:      "restore",
		Args:         jobArgs,
		Image:        repository.Spec.Image,
		Env:          repository.Spec.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        repository,
	}
}

// BuildBackupJobSpec builds the spec for a backup job.
// args must include NAME (aka. latest, or snapshot ID)
func BuildBackupJobSpec(repository *v1.ResticRepository, backup *v1.ResticBackup, args []string) ResticJobSpec {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
			ReadOnly:  true, // Backup is read-only
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: backup.Spec.SourcePVC.Name,
				},
			},
		},
	}

	// Build restic backup command arguments
	jobArgs := []string{
		"restic", "backup",
		"--repo", repository.Spec.Target,
		"--host", fmt.Sprintf("%s-%s", backup.Spec.SourcePVC.Namespace, backup.Spec.SourcePVC.Name),
		"--tag", fmt.Sprintf("name=%s", backup.Spec.Name),
		"--target", "/data",
	}

	jobArgs = append(jobArgs, args...)

	return ResticJobSpec{
		Namespace:    backup.Spec.SourcePVC.Namespace,
		JobType:      "backup",
		Args:         jobArgs,
		Image:        repository.Spec.Image,
		Env:          repository.Spec.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        repository,
	}
}
