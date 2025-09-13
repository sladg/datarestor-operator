package utils

import (
	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TaskToOwnerReference(task *v1.Task) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: task.APIVersion,
		Kind:       task.Kind,
		Name:       task.Name,
		UID:        task.UID,
		Controller: ptr.To(true),
	}
}

func PVCToOwnerReference(pvc corev1.ObjectReference) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: pvc.APIVersion,
		Kind:       pvc.Kind,
		Name:       pvc.Name,
		UID:        pvc.UID,
		Controller: ptr.To(false),
	}
}

func PVCToRef(pvc *corev1.PersistentVolumeClaim) corev1.ObjectReference {
	apiVersion := "v1"

	// Those are often empty in order to match any API version
	if pvc.APIVersion != "" {
		apiVersion = pvc.APIVersion
	}

	kind := "PersistentVolumeClaim"
	if pvc.Kind != "" {
		kind = pvc.Kind
	}

	return corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       pvc.Name,
		UID:        pvc.UID,
	}
}

func JobToRef(job *batchv1.Job) corev1.ObjectReference {
	apiVersion := "batch/v1"
	if job.APIVersion != "" {
		apiVersion = job.APIVersion
	}
	kind := "Job"
	if job.Kind != "" {
		kind = job.Kind
	}
	return corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       job.Name,
		Namespace:  job.Namespace,
		UID:        job.UID,
	}
}
