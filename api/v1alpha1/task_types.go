/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobType defines the type of restic operation
type TaskType string

const (
	TaskTypeBackupScheduled  TaskType = "backup-scheduled"
	TaskTypeBackupManual     TaskType = "backup-manual"
	TaskTypeRestoreManual    TaskType = "restore-manual"
	TaskTypeRestoreAutomated TaskType = "restore-automated"
)

// TaskSpec defines the desired state of
type TaskSpec struct {
	// Name of the task
	// +required
	Name string `json:"name"`

	// Annotation of the task, {repoName}#{hostName}#{backupName} or {hostName}#{backupName} or {backupName}
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9-]+#([a-zA-Z0-9-]+#)?[a-zA-Z0-9-]+$`
	Annotation string `json:"annotation,omitempty"`

	// Type of restic operation
	// +required
	// +kubebuilder:validation:Enum=backup-scheduled;backup-manual;restore-manual;restore-automated
	Type TaskType `json:"type"`

	// Job template - if not specified, job will fail
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	JobTemplate apiextensionsv1.JSON `json:"jobTemplate,omitempty"`
}

// TaskStatus defines the observed state of Task
type TaskStatus struct {
	// Status of the underlying Kubernetes Job
	// +optional
	JobStatus batchv1.JobStatus `json:"jobStatus,omitempty"`

	// Reference to the underlying Kubernetes Job
	// +optional
	JobRef corev1.ObjectReference `json:"jobRef,omitempty"`

	// High-level state derived from JobStatus (Pending, Running, Succeeded, Failed)
	// +optional
	State string `json:"state,omitempty"`

	// Output from the restic command (if captured)
	// +optional
	Output string `json:"output,omitempty"`

	// Initialized at
	// +optional
	InitializedAt *metav1.Time `json:"initializedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="PVC",type="string",JSONPath=".spec.jobTemplate.template.spec.volumes[0].persistentVolumeClaim.claimName"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Job",type="string",JSONPath=".status.jobRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Task is the Schema for the tasks API
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec,omitempty"`
	Status TaskStatus `json:"status,omitempty"`
}

// GetLogValues returns key-value pairs for structured logging.
func (r *Task) GetLogValues() []interface{} {
	return []interface{}{
		"task", r.Name,
		"namespace", r.Namespace,
		"type", r.Spec.Type,
	}
}

// +kubebuilder:object:root=true

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Task{}, &TaskList{})
}
