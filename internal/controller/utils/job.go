package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// BuildBackupJobCommand returns command/args for a restic backup job.
func BuildBackupJobCommand(backupName string, customArgs []string) ([]string, []string, error) {
	command := []string{"restic", "backup"}
	args := []string{"/data"}
	if backupName != "" {
		args = append(args, "--tag", backupName)
	}
	args = append(args, customArgs...)
	return command, args, nil
}

// BuildRestoreJobCommand returns command/args for a restic restore job.
func BuildRestoreJobCommand(snapshotID string) ([]string, []string, error) {
	if snapshotID == "" {
		snapshotID = "latest"
	}
	command := []string{"restic", "restore"}
	args := []string{snapshotID, "--target", "/data"}
	return command, args, nil
}

// CreateRestoreJobWithOutput returns placeholder Job and ConfigMap objects.
func CreateRestoreJobWithOutput(_ context.Context, _ client.Client, _ corev1.PersistentVolumeClaim, _ v1alpha1.BackupTarget, _ string, _ metav1.Object) (*batchv1.Job, *corev1.ConfigMap, error) {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "stub-restore-job"}}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "stub-output-cm"}}
	return job, cm, nil
}

// CleanupJob is a no-op placeholder.
func CleanupJob(_ context.Context, _ client.Client, _ string, _ string) error {
	return nil
}

// GetRepositoryNameForTarget computes a stable repository name for a target.
func GetRepositoryNameForTarget(target v1alpha1.BackupTarget) string {
	if target.Name != "" {
		return "repo-" + target.Name
	}
	return "repo-default"
}

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

// BuildInitJobCommand returns command/args for repository initialization
func BuildInitJobCommand(repository string) ([]string, []string, error) {
	return []string{"restic", "init"}, []string{"--repo", repository}, nil
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
		"app.kubernetes.io/managed-by": constants.OperatorDomain,
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
