package task_util

import (
	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type BuildTaskParams struct {
	PVC      *corev1.PersistentVolumeClaim
	Env      []corev1.EnvVar
	Args     []string
	TaskType v1.TaskType
}

func BuildTask(params BuildTaskParams) v1.Task {
	taskName, taskSpecName := GenerateUniqueName(UniqueNameParams{
		PVC:      params.PVC,
		TaskType: params.TaskType,
	})

	volumes := []corev1.Volume{
		{
			Name: constants.MountName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: params.PVC.Name,
				},
			},
		},
	}

	arguments := params.Args
	arguments = append(arguments, []string{"--target", constants.MountPath}...)

	task := v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskName,
			Namespace: params.PVC.Namespace, // Necessary to run same namespace in order to mount PVC
			Labels: map[string]string{ // Task is owner by operator's config. If config is deleted, Task should be deleted too.
				"managed-by": v1.OperatorDomain,
				"task-type":  string(params.TaskType),
				"pvc":        params.PVC.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				// When PVC is deleted, Task should be deleted too.
				{
					APIVersion: params.PVC.APIVersion,
					Kind:       params.PVC.Kind,
					Name:       params.PVC.Name,
					UID:        params.PVC.UID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: v1.TaskSpec{
			Name: taskSpecName,
			Type: v1.TaskTypeBackupScheduled,
			JobTemplate: batchv1.JobSpec{
				ManagedBy:               ptr.To("datarestor-operator"),
				TTLSecondsAfterFinished: ptr.To(int32(7200)), // Delete after 2 hours
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes:       volumes,
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{{
							Name:    "restic",
							Command: []string{"restic"},
							Args:    arguments,
							Env:     params.Env,
							Image:   constants.GetResticImage(),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      constants.MountName,
									MountPath: constants.MountPath,
								},
							},
						}},
					},
				},
			},
		},
	}

	return task
}
