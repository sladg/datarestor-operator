package task_util

import (
	"encoding/json"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type BuildTaskParams struct {
	Config   *v1.Config
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

	jobSpec := batchv1.JobSpec{
		// Managed by not set, it has to be empty in order for k8s to manage it automatically
		TTLSecondsAfterFinished: ptr.To(int32(7200)), // Delete after 2 hours
		Completions:             ptr.To(int32(1)),    // Run exactly once
		Parallelism:             ptr.To(int32(1)),    // Single worker
		BackoffLimit:            ptr.To(int32(0)),    // Do not retry failed pods
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constants.LabelTaskParentName:      params.Config.Name,
					constants.LabelTaskParentNamespace: params.Config.Namespace,
				},
			},
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
	}

	jobSpecBytes, _ := json.Marshal(jobSpec)

	task := v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskName,
			Namespace: params.PVC.Namespace, // Necessary to run same namespace in order to mount PVC
			Labels: map[string]string{ // Task is owned by operator's config. If config is deleted, Task should be deleted too.
				"managed-by":                       v1.OperatorDomain,
				"task-type":                        string(params.TaskType),
				"pvc":                              params.PVC.Name,
				constants.LabelTaskParentName:      params.Config.Name,
				constants.LabelTaskParentNamespace: params.Config.Namespace,
			},
			OwnerReferences: []metav1.OwnerReference{
				// PVC owns the Task - when PVC is deleted, Task should be deleted too
				// Note: Cannot set OwnerReference to Config due to cross-namespace constraints
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
			Name:        taskSpecName,
			Type:        params.TaskType,
			JobTemplate: apiextensionsv1.JSON{Raw: jobSpecBytes},
		},
	}

	return task
}
