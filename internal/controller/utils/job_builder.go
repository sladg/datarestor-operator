package utils

import (
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BuildInitJobSpec builds the spec for a repository initialization job.
func BuildInitJobSpec(repository *v1.ResticRepository) ResticJobSpec {
	return ResticJobSpec{
		Namespace:  repository.Namespace,
		JobType:    "init",
		Command:    []string{"restic", "init"},
		Args:       []string{"--repo", repository.Spec.Target},
		Repository: repository.Spec.Target,
		Image:      repository.Spec.Image,
		Env:        repository.Spec.Env,
		Owner:      repository,
	}
}

// BuildCheckJobSpec builds the spec for a repository check job.
func BuildCheckJobSpec(repository *v1.ResticRepository) ResticJobSpec {
	return ResticJobSpec{
		Namespace:  repository.Namespace,
		JobType:    "check",
		Command:    []string{"restic", "check"},
		Args:       []string{"--repo", repository.Spec.Target},
		Repository: repository.Spec.Target,
		Image:      repository.Spec.Image,
		Env:        repository.Spec.Env,
		Owner:      repository,
	}
}

// BuildRestoreJobSpec builds the spec for a restore job.
func BuildRestoreJobSpec(restore *v1.ResticRestore, repository *v1.ResticRepository) ResticJobSpec {
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

	args := []string{"--repo", repository.Spec.Target}
	args = append(args, "--tag", fmt.Sprintf("namespace=%s", restore.Namespace))

	// Add snapshot ID (or latest if not specified)
	if restore.Spec.SnapshotID != "" {
		args = append(args, restore.Spec.SnapshotID)
	} else {
		args = append(args, "latest")
	}

	args = append(args, "--target", "/data")
	args = append(args, restore.Spec.Args...)

	return ResticJobSpec{
		Namespace:    restore.Spec.TargetPVC.Namespace,
		JobType:      "restore",
		Command:      []string{"restic", "restore"},
		Args:         args,
		Repository:   repository.Spec.Target,
		Image:        repository.Spec.Image,
		Env:          repository.Spec.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        repository,
	}
}

// BuildBackupJobSpec builds the spec for a backup job.
func BuildBackupJobSpec(backup *v1.ResticBackup, repository *v1.ResticRepository) ResticJobSpec {
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

	// Build args with tags for traceability
	args := []string{"--repo", repository.Spec.Target}

	args = append(args, "--tag", fmt.Sprintf("namespace=%s", backup.Namespace))

	if backup.Spec.SnapshotID != "" {
		// Use the snapshot ID as a tag to identify this backup
		args = append(args, "--tag", fmt.Sprintf("name=%s", backup.Spec.SnapshotID))
	}

	args = append(args, "/data")
	args = append(args, backup.Spec.Args...)

	return ResticJobSpec{
		Namespace:    backup.Spec.SourcePVC.Namespace,
		JobType:      "backup",
		Command:      []string{"restic", "backup"},
		Args:         args,
		Repository:   repository.Spec.Target,
		Image:        repository.Spec.Image,
		Env:          repository.Spec.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        repository,
	}
}
