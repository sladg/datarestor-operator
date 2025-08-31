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

// ResticBackupSpec defines the desired state of ResticBackup (merged with BackupJob)
type ResticBackupSpec struct {
	// Human-readable backup name
	// +required
	BackupName string `json:"backupName"`

	// PVC that was/is being backed up
	// +required
	PVCRef PVCReference `json:"pvcRef"`

	// Reference to the ResticRepository where this backup is stored
	// +required
	RepositoryRef corev1.LocalObjectReference `json:"repositoryRef"`

	// Backup type (scheduled, manual, auto-restore)
	// +optional
	// +kubebuilder:default="scheduled"
	BackupType string `json:"backupType,omitempty"`

	// Restic snapshot ID (populated when backup completes successfully)
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`

	// Tags associated with this backup
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// PVCReference contains PVC information for cross-namespace references
type PVCReference struct {
	// Name of the PVC
	// +required
	Name string `json:"name"`

	// Namespace of the PVC
	// +required
	Namespace string `json:"namespace"`
}

// ResticBackupStatus defines the observed state of ResticBackup (merged with BackupJob)
type ResticBackupStatus struct {
	// Current phase of the backup (Pending, Running, Completed, Ready, Failed)
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Ready;Failed
	Phase string `json:"phase,omitempty"`

	// Time when the backup job started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Time when the backup job completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Time when the backup was created in the repository
	// +optional
	BackupTime *metav1.Time `json:"backupTime,omitempty"`

	// Reference to the Kubernetes Job that executed the backup
	// +optional
	BackupJobRef *corev1.LocalObjectReference `json:"backupJobRef,omitempty"`

	// Reference to the verification job (if any)
	// +optional
	VerificationJobRef *corev1.LocalObjectReference `json:"verificationJobRef,omitempty"`

	// Reference to the deletion job (if any)
	// +optional
	DeletionJobRef *corev1.LocalObjectReference `json:"deletionJobRef,omitempty"`

	// Backup ID assigned by the system (for tracking)
	// +optional
	BackupID string `json:"backupID,omitempty"`

	// Size of the backup in bytes
	// +optional
	Size int64 `json:"size,omitempty"`

	// Last time this backup's existence was verified in the repository
	// +optional
	LastVerified *metav1.Time `json:"lastVerified,omitempty"`

	// Error message if backup is in failed state
	// +optional
	Error string `json:"error,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Backup Name",type="string",JSONPath=".spec.backupName"
// +kubebuilder:printcolumn:name="PVC",type="string",JSONPath=".spec.pvcRef.name"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Snapshot ID",type="string",JSONPath=".spec.snapshotID"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".status.size"
// +kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repositoryRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticBackup is the Schema for the resticbackups API
type ResticBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticBackupSpec   `json:"spec,omitempty"`
	Status ResticBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResticBackupList contains a list of ResticBackup
type ResticBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticBackup{}, &ResticBackupList{})
}
