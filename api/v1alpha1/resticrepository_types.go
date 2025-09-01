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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResticRepositorySpec defines the desired state of ResticRepository
type ResticRepositorySpec struct {
	// Restic repository URL (e.g., s3:s3.amazonaws.com/bucket/path)
	// +required
	Repository string `json:"repository"`

	// Password for restic repository
	// +optional
	Password string `json:"password,omitempty"`

	// Environment variables for authentication (e.g., AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Additional restic flags for repository operations
	// +optional
	Flags []string `json:"flags,omitempty"`

	// Image for restic operations
	// +optional
	// +kubebuilder:default="restic/restic:latest"
	Image string `json:"image,omitempty"`

	// Repository maintenance schedule (for check, prune operations)
	// +optional
	MaintenanceSchedule *RepositoryMaintenanceSchedule `json:"maintenanceSchedule,omitempty"`
}

// RepositoryMaintenanceSchedule defines when to run repository maintenance
type RepositoryMaintenanceSchedule struct {
	// Cron expression for repository check schedule (e.g., "0 4 * * 0" for weekly at 4 AM)
	// +optional
	// +kubebuilder:default="0 4 * * 0"
	CheckCron string `json:"checkCron,omitempty"`

	// Cron expression for repository prune schedule (e.g., "0 5 * * 0" for weekly at 5 AM)
	// +optional
	// +kubebuilder:default="0 5 * * 0"
	PruneCron string `json:"pruneCron,omitempty"`

	// Cron expression for snapshot verification schedule (e.g., "0 1 * * *" for daily at 1 AM)
	// +optional
	// +kubebuilder:default="0 1 * * *"
	VerificationCron string `json:"verificationCron,omitempty"`

	// Whether maintenance is enabled
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

// ResticRepositoryStatus defines the observed state of ResticRepository
type ResticRepositoryStatus struct {
	// Repository initialization status
	// +optional
	// +kubebuilder:validation:Enum=Unknown;Initializing;Ready;Failed
	Phase string `json:"phase,omitempty"`

	// Time when the repository was last initialized
	// +optional
	InitializedTime *metav1.Time `json:"initializedTime,omitempty"`

	// Last time repository connectivity was verified
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`

	// Next scheduled maintenance time
	// +optional
	NextMaintenance *metav1.Time `json:"nextMaintenance,omitempty"`

	// Repository statistics
	// +optional
	Stats *RepositoryStats `json:"stats,omitempty"`

	// Last time snapshot verification was performed
	// +optional
	LastSnapshotVerification *metav1.Time `json:"lastSnapshotVerification,omitempty"`

	// Number of snapshots successfully verified in last verification
	// +optional
	SnapshotVerificationCount int64 `json:"snapshotVerificationCount,omitempty"`

	// Number of snapshot verification errors in last verification
	// +optional
	SnapshotVerificationErrors int64 `json:"snapshotVerificationErrors,omitempty"`

	// Error message from snapshot verification (if any)
	// +optional
	SnapshotVerificationError string `json:"snapshotVerificationError,omitempty"`

	// Reference to the current repository check job (if any)
	// +optional
	CheckJobRef *corev1.LocalObjectReference `json:"checkJobRef,omitempty"`

	// Error message if repository is in failed state
	// +optional
	Error string `json:"error,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// RepositoryStats provides statistics about the repository
type RepositoryStats struct {
	// Total number of snapshots in repository
	// +optional
	TotalSnapshots int32 `json:"totalSnapshots,omitempty"`

	// Total size of repository in bytes
	// +optional
	TotalSize int64 `json:"totalSize,omitempty"`

	// Number of data blobs
	// +optional
	TotalBlobs int32 `json:"totalBlobs,omitempty"`

	// Repository version
	// +optional
	Version string `json:"version,omitempty"`

	// Last time stats were updated
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repository"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Initialized",type="boolean",JSONPath=".status.initialized"
// +kubebuilder:printcolumn:name="Snapshots",type="integer",JSONPath=".status.stats.totalSnapshots"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".status.stats.totalSize"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticRepository is the Schema for the resticrepositories API
type ResticRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticRepositorySpec   `json:"spec,omitempty"`
	Status ResticRepositoryStatus `json:"status,omitempty"`
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
