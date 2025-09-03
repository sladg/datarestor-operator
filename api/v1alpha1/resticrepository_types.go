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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RepositoryMaintenanceSchedule defines when to run repository maintenance
type RepositoryMaintenanceSchedule struct {
	// Cron expression for repository check schedule (e.g., "0 4 * * 0" for weekly at 4 AM)
	// +optional
	// +kubebuilder:default="0 4 * * 0"
	CheckSchedule string `json:"checkSchedule,omitempty"`

	// Cron expression for repository prune schedule (e.g., "0 5 * * 0" for weekly at 5 AM)
	// +optional
	// +kubebuilder:default="0 5 * * 0"
	PruneSchedule string `json:"pruneSchedule,omitempty"`

	// Cron expression for snapshot verification schedule (e.g., "0 1 * * *" for daily at 1 AM)
	// +optional
	// +kubebuilder:default="0 1 * * *"
	VerificationSchedule string `json:"verificationSchedule,omitempty"`

	// Whether maintenance is enabled
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

type ResticRepositorySpec struct {
	// Reference to the BackupConfig that owns this repository
	// +optional
	BackupConfig *BackupConfig `json:"backupConfig,omitempty"`

	// Restic repository target (e.g., s3:s3.amazonaws.com/bucket/path)
	// +required
	Target string `json:"target"`

	// Environment variables for authentication (e.g., AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Arguments to pass directly to the restic command.
	// Use with caution, as invalid flags can break operations.
	// +optional
	Args []string `json:"args,omitempty"`

	// Image for restic operations
	// +optional
	// +kubebuilder:default="restic/restic:latest"
	// +kubebuilder:validation:Pattern=`^[^:]+:[^:]+$`
	Image string `json:"image,omitempty"`
}

// ResticRepositoryStatus defines the observed state of ResticRepository
type ResticRepositoryStatus struct {
	// Embed common status fields
	CommonStatus `json:",inline"`

	// Time when the repository was last initialized
	// +optional
	InitializedTime *metav1.Time `json:"initializedTime,omitempty"`

	// Reference to the current repository check job
	// +optional
	Job *batchv1.Job `json:"job,omitempty"`

	// Next maintenance time
	// +required
	MaintenanceSchedule RepositoryMaintenanceSchedule `json:"maintenanceSchedule"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="BackupConfig",type="string",JSONPath=".spec.backupConfig.name"
// +kubebuilder:printcolumn:name="Initialized",type="date",JSONPath=".status.initializedTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticRepository is the Schema for the resticrepositories API
type ResticRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticRepositorySpec   `json:"spec,omitempty"`
	Status ResticRepositoryStatus `json:"status,omitempty"`
}

// GetLogValues returns key-value pairs for structured logging.
func (r *ResticRepository) GetLogValues() []interface{} {
	return []interface{}{
		"repository", r.Name,
		"namespace", r.Namespace,
		"phase", r.Status.Phase,
	}
}

// +kubebuilder:object:root=true

// ResticRepositoryList contains a list of ResticRepository
type ResticRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticRepository{}, &ResticRepositoryList{})
}
