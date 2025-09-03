package utils

import (
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BuildInitJobSpec builds the spec for a repository initialization job.
func BuildInitJobSpec(repo *v1.ResticRepository) (ResticJobSpec, error) {
	command, args, err := BuildInitJobCommand("init")
	if err != nil {
		return ResticJobSpec{}, fmt.Errorf("failed to build init command: %w", err)
	}

	return ResticJobSpec{
		Namespace:  repo.Namespace,
		JobType:    "init",
		Command:    command,
		Args:       args,
		Repository: repo.Spec.Target,
		Image:      repo.Spec.Image,
		Env:        repo.Spec.Env,
		Owner:      repo,
	}, nil
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
func BuildRestoreJobSpec(restore *v1.ResticRestore) (ResticJobSpec, error) {
	jobName := fmt.Sprintf("restore-%s", restore.Name)
	pvcName := restore.Spec.TargetPVC.Name

	command, args, err := BuildRestoreJobCommand(restore.Spec.SnapshotID)
	if err != nil {
		return ResticJobSpec{}, fmt.Errorf("failed to build restore command: %w", err)
	}

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
					ClaimName: pvcName,
				},
			},
		},
	}

	return ResticJobSpec{
		JobName:      jobName,
		Namespace:    restore.Spec.TargetPVC.Namespace,
		JobType:      "restore",
		Command:      command,
		Args:         args,
		Repository:   restore.Spec.Restic.Target,
		Image:        restore.Spec.Restic.Image,
		Env:          restore.Spec.Restic.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        restore,
	}, nil
}

// BuildBackupJobSpec builds the spec for a backup job.
func BuildBackupJobSpec(backup *v1.ResticBackup) (ResticJobSpec, error) {
	jobName := fmt.Sprintf("backup-%s", backup.Name)
	pvcName := backup.Spec.SourcePVC.Name

	command, args, err := BuildBackupJobCommand(jobName, backup.Spec.Args)
	if err != nil {
		return ResticJobSpec{}, fmt.Errorf("failed to build backup command: %w", err)
	}

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
					ClaimName: pvcName,
				},
			},
		},
	}

	return ResticJobSpec{
		JobName:      jobName,
		Namespace:    backup.Spec.SourcePVC.Namespace,
		JobType:      "backup",
		Command:      command,
		Args:         args,
		Repository:   backup.Spec.Restic.Target,
		Image:        backup.Spec.Restic.Image,
		Env:          backup.Spec.Restic.Env,
		VolumeMounts: volumeMounts,
		Volumes:      volumes,
		Owner:        backup,
	}, nil
}
