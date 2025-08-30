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

// BackupJobSpec defines the desired state of BackupJob
type BackupJobSpec struct {
	// PVC reference for this backup job
	// +required
	PVCRef corev1.LocalObjectReference `json:"pvcRef"`

	// Backup target configuration
	// +required
	BackupTarget BackupTarget `json:"backupTarget"`

	// Parent BackupConfig reference
	// +required
	BackupConfigRef corev1.LocalObjectReference `json:"backupConfigRef"`

	// Backup type (manual, scheduled)
	// +optional
	BackupType string `json:"backupType,omitempty"`
}

// BackupJobStatus defines the observed state of BackupJob
type BackupJobStatus struct {
	// Current phase of the backup job
	// +optional
	Phase string `json:"phase,omitempty"`

	// Backup start timestamp
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Backup completion timestamp
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Restic backup ID
	// +optional
	ResticID string `json:"resticID,omitempty"`

	// Backup size in bytes
	// +optional
	Size int64 `json:"size,omitempty"`

	// Repository URL where backup is stored
	// +optional
	Repository string `json:"repository,omitempty"`

	// Error message if backup failed
	// +optional
	Error string `json:"error,omitempty"`

	// Kubernetes Job reference
	// +optional
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="PVC",type="string",JSONPath=".spec.pvcRef.name"
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.backupTarget.name"
// +kubebuilder:printcolumn:name="Restic ID",type="string",JSONPath=".status.resticID"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".status.size"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BackupJob is the Schema for the backupjobs API
type BackupJob struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of BackupJob
	// +required
	Spec BackupJobSpec `json:"spec"`

	// status defines the observed state of BackupJob
	// +optional
	Status BackupJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupJobList contains a list of BackupJob
type BackupJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupJob{}, &BackupJobList{})
}
