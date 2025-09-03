package utils

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ResticJobSpec defines the specification for a restic job
type ResticJobSpec struct {
	JobName      string
	Namespace    string
	JobType      string
	Repository   string
	Command      []string
	Args         []string
	Env          []corev1.EnvVar
	Image        string
	VolumeMounts []corev1.VolumeMount
	Volumes      []corev1.Volume
	Owner        metav1.Object
}

// CreateResticJobWithOutput creates a restic job with output capture
func CreateResticJobWithOutput(ctx context.Context, deps *Dependencies, spec ResticJobSpec, owner metav1.Object) (*batchv1.Job, *corev1.ConfigMap, error) {
	// Use the owner from spec if provided, otherwise use the passed owner
	actualOwner := owner
	if spec.Owner != nil {
		actualOwner = spec.Owner
	}

	// Use the namespace from spec if provided, otherwise use owner's namespace
	namespace := actualOwner.GetNamespace()
	if spec.Namespace != "" {
		namespace = spec.Namespace
	}

	// Use the job name from spec if provided, otherwise generate one
	jobName := spec.JobName
	if jobName == "" {
		// Generate a consistent name format: restic-{type}-{owner}-{timestamp}
		jobName = fmt.Sprintf("restic-%s-%s-%d", spec.JobType, actualOwner.GetName(), time.Now().Unix())
	}

	labels := map[string]string{
		"app.kubernetes.io/managed-by": v1.OperatorDomain,
		"app.kubernetes.io/instance":   actualOwner.GetName(),
		"job-type":                     spec.JobType,
	}

	// Create job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &[]int32{4}[0],
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "restic",
							Image:        spec.Image,
							Command:      append(spec.Command, spec.Args...),
							Env:          spec.Env,
							VolumeMounts: spec.VolumeMounts,
						},
					},
					Volumes:       spec.Volumes,
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(actualOwner, job, deps.Scheme); err != nil {
		return nil, nil, err
	}

	// Create output configmap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-output",
			Namespace: namespace,
			Labels:    labels,
		},
	}

	if err := controllerutil.SetControllerReference(actualOwner, cm, deps.Scheme); err != nil {
		return nil, nil, err
	}

	// Actually create the job in the cluster
	if err := deps.Create(ctx, job); err != nil {
		return nil, nil, fmt.Errorf("failed to create job in Kubernetes: %w", err)
	}

	// Actually create the ConfigMap in the cluster
	if err := deps.Create(ctx, cm); err != nil {
		// Attempt to clean up the job since the ConfigMap creation failed
		if deleteErr := deps.Delete(ctx, job); deleteErr != nil {
			return nil, nil, fmt.Errorf("failed to create ConfigMap: %w, and failed to clean up job: %v", err, deleteErr)
		}
		return nil, nil, fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	return job, cm, nil
}

// IsJobFinished checks if a job has completed, either successfully or with an error.
// It returns two booleans: the first indicates if the job is finished,
// the second indicates if the job was successful.
func IsJobFinished(job *batchv1.Job) (finished bool, succeeded bool) {
	if job.Status.Succeeded > 0 {
		return true, true
	}
	if job.Status.Failed > 0 {
		return true, false
	}

	return false, false
}
