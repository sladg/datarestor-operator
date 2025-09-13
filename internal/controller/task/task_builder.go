package task_util

import (
	"encoding/json"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type BuildTaskParams struct {
	Config       *v1.Config
	PVC          *corev1.PersistentVolumeClaim
	Env          []corev1.EnvVar
	Args         []string
	TaskType     v1.TaskType
	StopPods     bool
	TaskName     string
	TaskSpecName string
}

func BuildTask(params BuildTaskParams) v1.Task {
	// Validate that PVC is not being deleted
	if utils.IsPVCBeingDeleted(params.PVC) {
		return v1.Task{}
	}

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
		TTLSecondsAfterFinished: ptr.To(int32(15)), // Delete after 15sec - we just need to reconcile and update Task. Necessary to avoid infinite Pods and deletions being stuck
		Completions:             ptr.To(int32(1)),  // Run exactly once
		Parallelism:             ptr.To(int32(1)),  // Single worker
		BackoffLimit:            ptr.To(int32(0)),  // Do not retry failed pods
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"managed-by":                       v1.OperatorDomain,
					constants.LabelTaskParentName:      params.Config.Name,
					constants.LabelTaskParentNamespace: params.Config.Namespace,
					constants.LabelTaskType:            string(params.TaskType),
					constants.LabelTaskPVC:             params.PVC.Name,
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
			Name:      params.TaskName,
			Namespace: params.PVC.Namespace, // Necessary to run same namespace in order to mount PVC
			Labels: map[string]string{ // Task is owned by operator's config. If config is deleted, Task should be deleted too.
				"managed-by":                       v1.OperatorDomain,
				constants.LabelTaskParentName:      params.Config.Name,
				constants.LabelTaskParentNamespace: params.Config.Namespace,
				constants.LabelTaskPVC:             params.PVC.Name,
				constants.LabelTaskType:            string(params.TaskType),
			},
			// No OwnerReferences - Tasks no longer own PVCs
			// Jobs will own PVCs during active operations instead
			// Tasks should not own PVCs as that blocks deletion. Jobs will self-delete after couple seconds on completion unblocking PVC.
			// Tasks live on even after PVC's removal as historical evidence and for easier finding of available backups.
		},
		Spec: v1.TaskSpec{
			Name:        params.TaskSpecName,
			Type:        params.TaskType,
			JobTemplate: apiextensionsv1.JSON{Raw: jobSpecBytes},
			StopPods:    params.StopPods,
			PVCRef:      utils.PVCToRef(params.PVC),
		},
		Status: v1.TaskStatus{
			State: v1.TaskStatePending,
		},
	}

	return task
}
